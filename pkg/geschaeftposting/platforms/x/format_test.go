package x

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/internal/testcontacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
	xvote "github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
)

// Both limits the channel can be configured with: free accounts and Premium.
func TestEveryPostFitsAtEitherLimit(t *testing.T) {
	for _, limit := range []int{xvote.DefaultMaxChars, 2000} {
		for name, g := range testfixtures.Supported() {
			t.Run(fmt.Sprintf("%s/%d", name, limit), func(t *testing.T) {
				thread := FormatThread(g, testcontacts.Mapper(t), limit)
				for i, p := range thread {
					if n := xvote.WeightedLen(p.Text); n > limit {
						t.Errorf("post %d costs %d, limit %d:\n%s", i, n, limit, p.Text)
					}
				}
				if !strings.Contains(thread[0].Text, geschaeftposting.Link(g)) {
					t.Errorf("root lacks the Geschäft link:\n%s", thread[0].Text)
				}
				last := thread[len(thread)-1].Text
				if !strings.HasSuffix(last, geschaeftposting.ArtenURL) {
					t.Errorf("thread does not end on the Geschäftsarten link:\n%s", last)
				}
			})
		}
	}
}

// At Premium length nothing is cut or split: the whole title, every signer,
// and the explainer in one reply.
func TestPremiumKeepsEverything(t *testing.T) {
	g := testfixtures.PostulatManySigners()
	thread := FormatThread(g, nil, 2000)
	if len(thread) != 2 {
		t.Fatalf("thread has %d posts, want 2", len(thread))
	}
	if !strings.Contains(thread[0].Text, geschaeftposting.Title(g)) ||
		!strings.Contains(thread[0].Text, geschaeftposting.SignerLine(g, 0)) {
		t.Errorf("Premium root lost content:\n%s", thread[0].Text)
	}
}

func TestSignerIsTagged(t *testing.T) {
	thread := FormatThread(testfixtures.MotionMember(), testcontacts.Mapper(t), xvote.DefaultMaxChars)
	if !strings.Contains(thread[0].Text, "@"+testcontacts.XHandle) {
		t.Errorf("first signer not tagged:\n%s", thread[0].Text)
	}
}
