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
