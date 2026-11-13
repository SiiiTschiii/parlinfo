package x

import (
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// X refuses a post over the limit after the posts before it in the thread are
// already up, so no reply may exceed it, in any language: a French or Italian
// bill title is often a third longer than the German. The Zürich fixtures
// include a heading too long to fit even alone, which has to be cut.
func TestThreadPostsFitTheLimit_EveryLanguage(t *testing.T) {
	for name, group := range testfixtures.AllFixtures() {
		for i, p := range FormatVoteThread(group, nil, DefaultMaxChars) {
			if n := weightedLen(p.Text); n > DefaultMaxChars {
				t.Errorf("%s post %d weighs %d:\n%s", name, i, n, p.Text)
			}
		}
	}
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		for name, group := range testfixtures.NationalratFixtures(l) {
			for _, limit := range []int{280, 25000} {
				for i, p := range FormatVoteThread(group, nil, limit) {
					if n := weightedLen(p.Text); n > limit {
						t.Errorf("%s %s (limit %d) post %d weighs %d:\n%s", l, name, limit, i, n, p.Text)
					}
				}
			}
		}
	}
}
