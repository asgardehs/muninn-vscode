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

// ChangeKind describes what happened to the affected file.
type ChangeKind string

const (
	ChangeCreate ChangeKind = "create"
	ChangeModify ChangeKind = "modify"
	ChangeDelete ChangeKind = "delete"
)

// ChangeSource discriminates between vault note edits (Markdown files in the
// note tree) and schema edits (YAML files inside .muninn/schemas/).
type ChangeSource string

const (
	SourceNote   ChangeSource = "note"
	SourceSchema ChangeSource = "schema"
)

// Change is a single observed filesystem event.
type Change struct {
	RelPath string
	Kind    ChangeKind
	Source  ChangeSource
}

// schemaDir returns the vault-relative path to the schema directory.
const schemaDir = ".muninn/schemas"

// Watch observes the vault for both note (.md) and schema (.muninn/schemas/*.yml)
// changes. Hidden directories are skipped except for .muninn/schemas/, which is
// watched explicitly so user-authored schemas trigger live reload. Per-path
// debounce (100ms) coalesces editor save bursts. The channel closes when ctx
// is cancelled or the watcher fails.
//
// Events are dropped if the consumer falls behind by more than 64.
func (v *Vault) Watch(ctx context.Context, logger *log.Logger) (<-chan Change, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := walkAndAdd(w, v.root); err != nil {
		_ = w.Close()
		return nil, err
	}
	addSchemaDir(w, v.root, logger)

	out := make(chan Change, 64)
	go runWatcher(ctx, w, v.root, out, logger)
	return out, nil
}

// addSchemaDir watches <root>/.muninn/schemas/ if it already exists. We watch
// it explicitly because walkAndAdd skips hidden directories, but users
// expect schemas they drop here to take effect live.
func addSchemaDir(w *fsnotify.Watcher, root string, logger *log.Logger) {
	abs := filepath.Join(root, schemaDir)
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return
	}
	if err := w.Add(abs); err != nil {
		logger.Printf("watcher: add schema dir %q: %v", abs, err)
	}
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

	emit := func(c Change) {
		select {
		case out <- c:
		case <-ctx.Done():
		default:
			logger.Printf("watcher: dropped %s event for %s (consumer slow)", c.Source, c.RelPath)
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

			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					base := filepath.Base(event.Name)
					rel, _ := filepath.Rel(root, event.Name)
					switch {
					case rel == schemaDir:
						// User just created .muninn/schemas/ at the
						// root — start watching it so subsequent YAML
						// drops are picked up.
						if err := w.Add(event.Name); err != nil {
							logger.Printf("watcher: add schema dir %q: %v", event.Name, err)
						}
					case !strings.HasPrefix(base, "."):
						if err := walkAndAdd(w, event.Name); err != nil {
							logger.Printf("watcher: add %q: %v", event.Name, err)
						}
					}
					continue
				}
			}

			rel, err := filepath.Rel(root, event.Name)
			if err != nil {
				continue
			}
			source, ok := classify(rel)
			if !ok {
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
			captured := Change{RelPath: rel, Kind: kind, Source: source}
			debounce[rel] = &pending{
				kind: kind,
				t:    time.AfterFunc(debounceFor, func() { emit(captured) }),
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

// classify decides which channel a vault-relative path belongs to. Returns
// false for paths we don't care about (non-md, non-schema files).
func classify(rel string) (ChangeSource, bool) {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, schemaDir+"/") && strings.HasSuffix(rel, ".yml") {
		return SourceSchema, true
	}
	if strings.HasSuffix(rel, ".md") {
		return SourceNote, true
	}
	return "", false
}
