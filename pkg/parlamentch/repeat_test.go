package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// 25.060 on 08.09.2025: a Gesamtabstimmung, an Ordnungsantrag to repeat it,
// and the repeated Gesamtabstimmung. The second replaces the first, and is
// labelled as the repetition.
func TestMarkRepeats(t *testing.T) {
	orders := `{"d":{"results":[{"Subject":"Ordnungsantrag Matter (Wiederholen der Gesamtabstimmung)","VoteEnd":"/Date(1757350710000)/"}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("$filter"), "wiederhol") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(orders))
	}))
	t.Cleanup(srv.Close)

	newGroup := func(c *Client) []votes.Vote {
		g := []votes.Vote{
			{SourceID: "34716", Date: swiss(2025, 9, 8, 16, 54), Subtitle: "Gesamtabstimmung", Affair: votes.Affair{ID: "20250060", Number: "25.060"}},
			{SourceID: "34717", Date: swiss(2025, 9, 8, 16, 59), Subtitle: "Gesamtabstimmung", Affair: votes.Affair{ID: "20250060", Number: "25.060"}},
		}
		for _, v := range g {
			c.labels[v.SourceID] = voteLabel{subject: "Gesamtabstimmung", billNumber: 1, meaningYes: "Annahme der Vorlage", meaningNo: "Ablehnung der Vorlage"}
		}
		return g
	}

	c := New(votes.Jurisdiction{Key: "nationalrat"})
	c.SetBaseURL(srv.URL)
	c.maxAttempts = 1
	g := newGroup(c)
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	c.label(g)
	if g[0].Subtitle != "Gesamtabstimmung" || g[1].Subtitle != "Gesamtabstimmung (Wiederholung)" {
		t.Errorf("subtitles = %q, %q", g[0].Subtitle, g[1].Subtitle)
	}
	if s := c.standing(g); len(s) != 1 || s[0].SourceID != "34717" {
		t.Errorf("standing = %+v, want the repeated vote alone", s)
	}

	// Without an Ordnungsantrag between them, two votes alike stay as they
	// are: 24.314 voted twice, and nothing says why.
	orders = `{"d":{"results":[]}}`
	c = New(votes.Jurisdiction{Key: "nationalrat"})
	c.SetBaseURL(srv.URL)
	g = newGroup(c)
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	if s := c.standing(g); len(s) != 2 {
		t.Errorf("standing = %d votes, want both", len(s))
	}
}
