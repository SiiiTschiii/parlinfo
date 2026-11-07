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
		for _, m := range []Msg{MsgVoteNumbered, MsgCardOverview} {
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
