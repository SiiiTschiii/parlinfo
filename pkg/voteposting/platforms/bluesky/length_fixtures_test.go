package bluesky

import (
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Bluesky refuses a post over its limit, so no reply may exceed it, in any
// language: a French or Italian bill title is often a third longer.
func TestThreadPostsFitTheLimit_EveryLanguage(t *testing.T) {
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		for name, group := range testfixtures.NationalratFixtures(l) {
			for i, p := range FormatVoteThread(group, nil) {
				if n := graphemeLen(p.Text); n > maxGraphemes {
					t.Errorf("%s %s post %d is %d graphemes:\n%s", l, name, i, n, p.Text)
				}
			}
		}
	}
}
