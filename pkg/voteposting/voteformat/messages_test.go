package voteformat

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// A field left out of a language's Words is empty, and prints as nothing in
// a post: "📊 106  | 87 No". So every language must fill every field.
func TestWordsAreComplete(t *testing.T) {
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		w := reflect.ValueOf(W(l))
		for i := 0; i < w.NumField(); i++ {
			if w.Field(i).String() == "" {
				t.Errorf("%s has no %s", l, w.Type().Field(i).Name)
			}
		}
	}
}

// The wording that is a format string must take exactly one number in every
// language, or fmt prints "%!d(MISSING)" into a post.
func TestFormatWordsTakeOneNumber(t *testing.T) {
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		w := W(l)
		for _, format := range []string{w.VoteNumbered, w.CardOverview, w.OverviewMore} {
			if got := fmt.Sprintf(format, 3); strings.Contains(got, "%!") || !strings.Contains(got, "3") {
				t.Errorf("%s %q renders as %q", l, format, got)
			}
		}
	}
}

func TestW_FallsBackToGerman(t *testing.T) {
	for _, l := range []votes.Language{"", "rm"} {
		if got := W(l).Accepted; got != "Angenommen" {
			t.Errorf("W(%q) = %q, want German", l, got)
		}
	}
}

func TestFrenchElidesDeBeforeAVowel(t *testing.T) {
	andrea := []votes.Author{{Name: "Andrea Caroni", Party: "PLR"}}
	hans := []votes.Author{{Name: "Hans Stöckli", Party: "PS"}}
	for _, c := range []struct {
		lang    votes.Language
		authors []votes.Author
		want    string
	}{
		{votes.French, andrea, "Initiative parlementaire d’Andrea Caroni (PLR)"},
		{votes.French, hans, "Initiative parlementaire de Hans Stöckli (PS)"},
		{votes.Italian, andrea, "Initiative parlementaire di Andrea Caroni (PLR)"},
		{votes.German, andrea, "Initiative parlementaire von Andrea Caroni (PLR)"},
	} {
		group := []votes.Vote{{Language: c.lang, Affair: votes.Affair{Type: "Initiative parlementaire", Authors: c.authors}}}
		if got := GroupPrefixLine(group); got != c.want {
			t.Errorf("%s: GroupPrefixLine = %q, want %q", c.lang, got, c.want)
		}
		// The thread's block, when the label line had no room for anyone.
		got := blockTexts(SignatoryBlocks(group, c.authors))
		if want := "✍️ " + c.want; len(got) != 1 || got[0] != want {
			t.Errorf("%s: SignatoryBlocks = %q, want %q", c.lang, got, want)
		}
	}
}

// An elided first name that does not fit beside the label still moves to the
// next entry, as an unelided one does, rather than overrunning the limit.
func TestPackNames_ElidedNameStillSplitsFromTheLabel(t *testing.T) {
	b := NameBlock{"✍️ Initiative parlementaire de", []votes.Author{{Name: "Andrea Caroni", Party: "PLR"}}, votes.French}
	measure := func(s string) int { return len([]rune(s)) }
	got := PackNames(b, nil, 45, measure)
	for _, e := range got {
		if n := measure(e); n > 45 {
			t.Errorf("entry %q is %d runes, over the limit of 45", e, n)
		}
	}
	if len(got) != 2 {
		t.Errorf("PackNames = %q, want the label and the name apart", got)
	}
}
