package voteformat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// A message missing from a language falls back to German, which is right for
// a language nobody posts in and wrong for one somebody does: a French post
// with one German word in it. So every language must carry every message.
func TestMessagesAreComplete(t *testing.T) {
	for lang, table := range messages {
		for m := range messages[votes.German] {
			if table[m] == "" {
				t.Errorf("%s has no %q", lang, m)
			}
		}
		for m := range table {
			if _, ok := messages[votes.German][m]; !ok {
				t.Errorf("%s has %q, which German does not", lang, m)
			}
		}
	}
}

// The messages that are format strings must take exactly one number in every
// language, or fmt prints "%!d(MISSING)" into a post.
func TestFormatMessagesTakeOneNumber(t *testing.T) {
	for lang, table := range messages {
		for _, m := range []Msg{MsgVoteNumbered, MsgCardOverview, MsgOverviewMore} {
			if got := fmt.Sprintf(table[m], 3); strings.Contains(got, "%!") || !strings.Contains(got, "3") {
				t.Errorf("%s %q renders as %q", lang, m, got)
			}
		}
	}
}

func TestT_FallsBackToGerman(t *testing.T) {
	if got := T("", MsgAccepted); got != "Angenommen" {
		t.Errorf("zero language = %q, want German", got)
	}
	if got := T("rm", MsgAccepted); got != "Angenommen" {
		t.Errorf("unknown language = %q, want German", got)
	}
	if got := T(votes.Italian, MsgAccepted); got != "Accettato" {
		t.Errorf("Italian = %q", got)
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
