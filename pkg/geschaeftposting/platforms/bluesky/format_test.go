package bluesky

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/internal/testcontacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
)

func TestEveryPostFitsAndLinksResolve(t *testing.T) {
	for name, g := range testfixtures.Supported() {
		t.Run(name, func(t *testing.T) {
			thread := FormatThread(g, nil)
			// Root plus one explainer: both types' copy is written to fit one
			// reply, and a third post would bury the link to the source.
			if len(thread) != 2 {
				t.Fatalf("thread has %d posts, want 2", len(thread))
			}
			for i, p := range thread {
				if n := graphemeLen(p.Text); n > maxGraphemes {
					t.Errorf("post %d is %d graphemes, limit %d:\n%s", i, n, maxGraphemes, p.Text)
				}
				if len(p.Facets) != 1 {
					t.Fatalf("post %d has %d facets, want 1 link", i, len(p.Facets))
				}
			}

			root := thread[0]
			if !strings.Contains(root.Text, geschaeftposting.Title(g)) && !strings.Contains(root.Text, "…") {
				t.Errorf("root neither carries the title nor marks a cut:\n%s", root.Text)
			}
			f := root.Facets[0]
			if covered := root.Text[f.Index.ByteStart:f.Index.ByteEnd]; covered != "GR Nr. "+g.GRNr {
				t.Errorf("root facet covers %q, want the GR number label", covered)
			}
			if uri := f.Features[0].URI; uri != geschaeftposting.Link(g) {
				t.Errorf("root facet links %q, want %q", uri, geschaeftposting.Link(g))
			}

			reply := thread[1]
			rf := reply.Facets[0]
			if covered := reply.Text[rf.Index.ByteStart:rf.Index.ByteEnd]; covered != artenLabel {
				t.Errorf("reply facet covers %q", covered)
			}
			if rf.Features[0].URI != geschaeftposting.ArtenURL {
				t.Errorf("reply facet links %q", rf.Features[0].URI)
			}
		})
	}
}

func TestSignerIsMentioned(t *testing.T) {
	thread := FormatThread(testfixtures.MotionMember(), testcontacts.Mapper(t))
	ms := thread[0].Mentions
	if len(ms) != 1 || ms[0].Handle != testcontacts.BlueskyHandle {
		t.Fatalf("mentions = %+v, want Brigitte Fürer", ms)
	}
	if got := thread[0].Text[ms[0].ByteStart:ms[0].ByteEnd]; got != "Brigitte Fürer" {
		t.Errorf("mention covers %q", got)
	}
}
