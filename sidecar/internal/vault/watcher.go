package vault

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ChangeKind describes what happened to a note.
type ChangeKind string

const (
	ChangeCreate ChangeKind = "create"
	ChangeModify ChangeKind = "modify"
	ChangeDelete ChangeKind = "delete"
)

// Change is a single observed filesystem event for a vault note.
type Change struct {
	RelPath string
	Kind    ChangeKind
}

// Watch observes the vault root recursively for changes to .md files. Hidden
// directories (.git, .muninn) are skipped. Each observed change is debounced
// (per path, 100ms) and emitted on the returned channel. The channel closes
// when ctx is cancelled or the underlying watcher fails.
//
// The caller is responsible for consuming the channel; events are dropped if
// the consumer falls behind by more than 64 events.
func (v *Vault) Watch(ctx context.Context, logger *log.Logger) (<-chan Change, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := walkAndAdd(w, v.root); err != nil {
		_ = w.Close()
		return nil, err
	}

	out := make(chan Change, 64)
	go runWatcher(ctx, w, v.root, out, logger)
	return out, nil
}

func walkAndAdd(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") && path != root {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func runWatcher(ctx context.Context, w *fsnotify.Watcher, root string, out chan<- Change, logger *log.Logger) {
	defer close(out)
	defer w.Close()

	// Per-path debounce timers. Coalesces editor save-bursts (write-temp +
	// rename + chmod) into a single Change.
	type pending struct {
		kind ChangeKind
		t    *time.Timer
	}
	debounce := make(map[string]*pending)
	const debounceFor = 100 * time.Millisecond

	emit := func(rel string, kind ChangeKind) {
		select {
		case out <- Change{RelPath: rel, Kind: kind}:
		case <-ctx.Done():
		default:
			logger.Printf("watcher: dropped event for %s (consumer slow)", rel)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-w.Events:
			if !ok {
				return
			}

			// If a directory was created, add it to the watch set so we see
			// .md files inside it.
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !strings.HasPrefix(filepath.Base(event.Name), ".") {
						if err := walkAndAdd(w, event.Name); err != nil {
							logger.Printf("watcher: add %q: %v", event.Name, err)
						}
					}
					continue
				}
			}

			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}
			rel, err := filepath.Rel(root, event.Name)
			if err != nil {
				continue
			}

			var kind ChangeKind
			switch {
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				kind = ChangeDelete
			case event.Has(fsnotify.Create):
				kind = ChangeCreate
			case event.Has(fsnotify.Write):
				kind = ChangeModify
			default:
				continue
			}

			if existing, ok := debounce[rel]; ok {
				existing.t.Stop()
			}
			capturedRel, capturedKind := rel, kind
			debounce[rel] = &pending{
				kind: kind,
				t: time.AfterFunc(debounceFor, func() {
					emit(capturedRel, capturedKind)
				}),
			}

		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				logger.Printf("watcher: event overflow; some changes may have been missed")
				continue
			}
			logger.Printf("watcher: %v", err)
		}
	}
}
