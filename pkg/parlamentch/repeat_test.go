package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// repeatServer serves one Ordnungsantrag to repeat a vote, at the given time,
// with the given Ja and Nein counts.
func repeatServer(t *testing.T, at time.Time, ja, nein string) *Client {
	t.Helper()
	// The service writes Swiss wall-clock time as if it were UTC; see
	// parseODataDate.
	wall := time.Date(at.Year(), at.Month(), at.Day(), at.Hour(), at.Minute(), at.Second(), 0, time.UTC)
	orders := `{"d":{"results":[{"ID":34372,"Subject":"Ordnungsantrag Matter (Wiederholen der Gesamtabstimmung)","VoteEnd":"/Date(` +
		strconv.FormatInt(wall.UnixMilli(), 10) + `)/"}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		switch {
		case r.URL.Path == "/Voting/$count" && strings.Contains(filter, "Decision eq 1"):
			_, _ = rw.Write([]byte(ja))
		case r.URL.Path == "/Voting/$count" && strings.Contains(filter, "Decision eq 2"):
			_, _ = rw.Write([]byte(nein))
		case r.URL.Path == "/Vote" && strings.Contains(filter, "wiederhol"):
			_, _ = rw.Write([]byte(orders))
		default:
			http.NotFound(rw, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := New(votes.Jurisdiction{Key: "nationalrat"})
	c.SetBaseURL(srv.URL)
	c.maxAttempts = 1
	return c
}

// gesamt is a group of Gesamtabstimmungen alike on 25.060, at the given times
// of 08.09.2025.
func gesamt(c *Client, at ...time.Time) []votes.Vote {
	var g []votes.Vote
	for i, t := range at {
		v := votes.Vote{SourceID: string(rune('a' + i)), Date: t, Subtitle: "Gesamtabstimmung", Affair: votes.Affair{ID: "20250060", Number: "25.060"}}
		c.labels[v.SourceID] = voteLabel{subject: "Gesamtabstimmung", billNumber: 1, meaningYes: "Annahme der Vorlage", meaningNo: "Ablehnung der Vorlage"}
		g = append(g, v)
	}
	return g
}

func standingIDs(c *Client, g []votes.Vote) string {
	var ids []string
	for _, v := range c.standing(g) {
		ids = append(ids, v.SourceID)
	}
	return strings.Join(ids, ",")
}

// 25.060 on 08.09.2025: a Gesamtabstimmung, an adopted Ordnungsantrag to
// repeat it (181:0), and the repeated Gesamtabstimmung. The second replaces
// the first, and is labelled as the repetition.
func TestMarkRepeats(t *testing.T) {
	order := swiss(2025, 9, 8, 16, 58)
	c := repeatServer(t, order, "181", "0")
	g := gesamt(c, swiss(2025, 9, 8, 16, 54), swiss(2025, 9, 8, 16, 59))
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	c.label(g)
	if g[0].Subtitle != "Gesamtabstimmung" || g[1].Subtitle != "Gesamtabstimmung (Wiederholung)" {
		t.Errorf("subtitles = %q, %q", g[0].Subtitle, g[1].Subtitle)
	}
	if got := standingIDs(c, g); got != "b" {
		t.Errorf("standing = %s, want the repeated vote alone", got)
	}
}

// An Ordnungsantrag the chamber rejected repeats nothing.
func TestMarkRepeats_RejectedOrder(t *testing.T) {
	c := repeatServer(t, swiss(2025, 9, 8, 16, 58), "60", "120")
	g := gesamt(c, swiss(2025, 9, 8, 16, 54), swiss(2025, 9, 8, 16, 59))
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	if got := standingIDs(c, g); got != "a,b" {
		t.Errorf("standing = %s, want both", got)
	}
}

// Of three votes alike, an order between the second and third replaces the
// second alone: votes pair only with the next one alike.
func TestMarkRepeats_OnlyConsecutive(t *testing.T) {
	c := repeatServer(t, swiss(2025, 9, 8, 17, 5), "181", "0")
	g := gesamt(c, swiss(2025, 9, 8, 16, 54), swiss(2025, 9, 8, 17, 0), swiss(2025, 9, 8, 17, 10))
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	if got := standingIDs(c, g); got != "a,c" {
		t.Errorf("standing = %s, want a,c", got)
	}
}

// Without an Ordnungsantrag between them, two votes alike stay as they are:
// 24.314 voted twice, and nothing says why.
func TestMarkRepeats_NoOrder(t *testing.T) {
	c := repeatServer(t, swiss(2025, 9, 8, 18, 0), "181", "0")
	g := gesamt(c, swiss(2025, 9, 8, 16, 54), swiss(2025, 9, 8, 16, 59))
	if err := c.markRepeats(g); err != nil {
		t.Fatal(err)
	}
	if got := standingIDs(c, g); got != "a,b" {
		t.Errorf("standing = %s, want both", got)
	}
}
