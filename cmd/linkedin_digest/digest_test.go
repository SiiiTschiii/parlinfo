package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestPostedIDsTakesEveryPlatform(t *testing.T) {
	logs := []*votelog.VoteLog{
		{Platform: votelog.PlatformX, Votes: []votelog.VoteEntry{{ID: "both"}, {ID: "x-only"}}},
		{Platform: votelog.PlatformBluesky, Votes: []votelog.VoteEntry{{ID: "both"}, {ID: "bluesky-only"}}},
	}

	got := postedIDs(logs)

	if len(got) != 3 || !got["both"] || !got["x-only"] || !got["bluesky-only"] {
		t.Errorf("postedIDs() = %v, want both, x-only and bluesky-only", got)
	}
}

func TestWindowCoversItsDaysInclusive(t *testing.T) {
	w := newWindow(time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC), 7)
	on := func(y int, m time.Month, d, h int) votes.Vote {
		return votes.Vote{Date: time.Date(y, m, d, h, 0, 0, 0, time.UTC)}
	}

	if w != (window{From: "2026-11-04", To: "2026-11-10"}) {
		t.Fatalf("newWindow() = %v, want 2026-11-04 to 2026-11-10", w)
	}
	for _, c := range []struct {
		name string
		v    votes.Vote
		want bool
	}{
		{"day before", on(2026, 11, 3, 23), false},
		{"first day", on(2026, 11, 4, 0), true},
		{"last day, evening", on(2026, 11, 10, 23), true},
		{"day after", on(2026, 11, 11, 0), false},
		{"unknown date", votes.Vote{}, false},
	} {
		if got := w.holds(c.v); got != c.want {
			t.Errorf("holds(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReachesBack(t *testing.T) {
	w := window{From: "2026-11-04", To: "2026-11-10"}
	inside := votes.Vote{Date: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)}
	before := votes.Vote{Date: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}

	if reachesBack([]votes.Vote{inside, {}}, w) {
		t.Error("reachesBack() = true for a fetch that stops inside the window")
	}
	if !reachesBack([]votes.Vote{inside, before}, w) {
		t.Error("reachesBack() = false for a fetch that reaches the day before")
	}
}

func TestKeepDropsVotesGroupingPulledIn(t *testing.T) {
	multi := testfixtures.MultiVoteGroup()
	groups := [][]votes.Vote{multi, testfixtures.SingleVoteAbgelehnt()}

	got := keep(groups, func(v votes.Vote) bool { return v.SourceID == multi[1].SourceID })

	if len(got) != 1 || len(got[0]) != 1 || got[0][0].SourceID != multi[1].SourceID {
		t.Fatalf("keep() = %v, want only %s", got, multi[1].SourceID)
	}
}

func TestRender(t *testing.T) {
	mapper, err := contacts.LoadContactFiles("testdata/contacts.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sections := []section{
		{Name: "Gemeinderat Stadt Zürich", Groups: [][]votes.Vote{
			testfixtures.VoteWithMentions(),
			testfixtures.MultiVoteGroup(),
			testfixtures.AuswahlVote(),
		}},
		{Name: "Kantonsrat Zürich"},
	}
	got := render(sections, mapper)

	post, rest, _ := strings.Cut(got, "━━━ First comment ━━━")
	comment, tags, _ := strings.Cut(rest, "━━━ Tag by hand ━━━")

	for _, want := range []string{
		"🗳️ Gemeinderat Stadt Zürich | Sitzung vom 15.06.2025\n",
		"1. ✅ Angenommen (80 Ja, 30 Nein) · Postulat von Anna Graff (SP) vom 18.09.2024",
		"2. Teilrevision der Gemeindeordnung",
		"   ✅ Angenommen (90 Ja, 20 Nein) · Einleitungsartikel\n",
		"   ❌ Abgelehnt (40 Ja, 70 Nein) · Schlussabstimmung\n",
	} {
		if !strings.Contains(post, want) {
			t.Errorf("post lacks %q:\n%s", want, post)
		}
	}
	if strings.Contains(post, "http") {
		t.Errorf("post carries a link, which belongs in the first comment:\n%s", post)
	}
	if strings.Contains(post, "Kantonsrat") {
		t.Errorf("post has a heading for a body with no votes:\n%s", post)
	}
	// An Auswahl vote has no accepted/rejected outcome to state.
	if line := lineStarting(post, "3. "); strings.ContainsAny(line, "✅❌") || !strings.Contains(line, "A: ") {
		t.Errorf("Auswahl entry = %q, want its counts and no verdict", line)
	}

	for _, n := range []string{"1. http", "2. http", "3. http"} {
		if !strings.Contains(comment, n) {
			t.Errorf("first comment lacks a link for entry %q:\n%s", n, comment)
		}
	}
	if want := "1. Anna Graff: https://www.linkedin.com/in/anna-graff\n"; strings.TrimLeft(tags, "\n") != want {
		t.Errorf("tagging list = %q, want %q", tags, want)
	}
}

func TestRenderWithoutVotesIsEmpty(t *testing.T) {
	if got := render([]section{{Name: "Nationalrat"}}, nil); got != "" {
		t.Errorf("render() = %q, want nothing", got)
	}
}

func lineStarting(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func TestRenderElection(t *testing.T) {
	// The title names Adrian Bergmann, the member being replaced.
	mapper, err := contacts.LoadContactFiles("testdata/contacts.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sections := []section{{Name: "Kantonsrat Zürich", Groups: [][]votes.Vote{testfixtures.KantonsratWahlgeschaeft()}}}

	got := render(sections, mapper)

	rest, tags, _ := strings.Cut(got, "━━━ Tag by hand ━━━")
	_, comment, _ := strings.Cut(rest, "━━━ First comment ━━━")
	if want := "1. " + sections[0].Groups[0][0].BulletinURL + "\n"; !strings.Contains(comment, want) {
		t.Errorf("first comment = %q, want the Bulletin, the only place the outcome is", comment)
	}
	if strings.Contains(tags, "Adrian Bergmann") {
		t.Errorf("tagging list names the member being replaced:\n%s", tags)
	}
}

func TestGroupEntryNamesALoneThresholdVote(t *testing.T) {
	group := testfixtures.KantonsratLoneQuorumVote()

	got := groupEntry(1, "Vorlage", group)

	if !strings.HasPrefix(got, "1. Ausgabenbremse · ") {
		t.Errorf("groupEntry() = %q, want the counts named as an Ausgabenbremse vote", got)
	}
}

func TestGroupEntrySaysWhatJaSupported(t *testing.T) {
	group := testfixtures.SingleVoteAbgelehnt()
	group[0].Question = "Ja: keine Folge geben"

	got := groupEntry(1, "Parlamentarische Initiative", group)

	if want := "(20 Ja, 95 Nein) · Ja: keine Folge geben · Parlamentarische Initiative"; !strings.Contains(got, want) {
		t.Errorf("groupEntry() = %q, want it to contain %q", got, want)
	}
}

func TestRenderHeadsEachSitting(t *testing.T) {
	first := testfixtures.SingleVoteAngenommen()
	second := testfixtures.SingleVoteAbgelehnt()
	second[0].Date = first[0].Date.AddDate(0, 0, 7)
	// The later sitting first, as a source might return them.
	sections := []section{{Name: "Gemeinderat Stadt Zürich", Groups: [][]votes.Vote{second, first}}}

	got := render(sections, nil)

	want := "🗳️ Gemeinderat Stadt Zürich | Sitzung vom 15.06.2025\n\n1. ✅ Angenommen" +
		"(.|\n)*" + "\n🗳️ Gemeinderat Stadt Zürich | Sitzung vom 22.06.2025\n\n2. ❌ Abgelehnt"
	if !regexp.MustCompile(want).MatchString(got) {
		t.Errorf("render() did not head each sitting in date order:\n%s", got)
	}
}
