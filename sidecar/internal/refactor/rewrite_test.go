package refactor

import "testing"

func TestRewriteWikilinks_PreservesFragmentAndAlias(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		old, newName string
		want         string
	}{
		{
			"plain link",
			"see [[old.note]] for more",
			"old.note", "new.note",
			"see [[new.note]] for more",
		},
		{
			"with fragment",
			"see [[old.note#section]]",
			"old.note", "new.note",
			"see [[new.note#section]]",
		},
		{
			"with alias",
			"see [[old.note|the old one]]",
			"old.note", "new.note",
			"see [[new.note|the old one]]",
		},
		{
			"fragment + alias",
			"see [[old.note#sec|here]]",
			"old.note", "new.note",
			"see [[new.note#sec|here]]",
		},
		{
			"case-insensitive match, preserve new name casing",
			"see [[Old.Note]]",
			"old.note", "new.note",
			"see [[new.note]]",
		},
		{
			"unrelated link untouched",
			"see [[other.note]]",
			"old.note", "new.note",
			"see [[other.note]]",
		},
		{
			"prefix collision untouched",
			"see [[old.notebook]]",
			"old.note", "new.note",
			"see [[old.notebook]]",
		},
		{
			"multiple occurrences",
			"a [[old.note]] then [[old.note#x]]",
			"old.note", "new.note",
			"a [[new.note]] then [[new.note#x]]",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RewriteWikilinks(c.input, c.old, c.newName)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
