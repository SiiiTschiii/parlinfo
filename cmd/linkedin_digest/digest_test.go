package main

import (
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestPostedSinceTakesAnyPlatformInsideTheWindow(t *testing.T) {
	since := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	entry := func(id string, day int) votelog.VoteEntry {
		return votelog.VoteEntry{ID: id, PostedAt: time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC)}
	}
	logs := []*votelog.VoteLog{
		{Platform: votelog.PlatformX, Votes: []votelog.VoteEntry{entry("old", 1), entry("both", 9)}},
		{Platform: votelog.PlatformBluesky, Votes: []votelog.VoteEntry{entry("both", 9), entry("bluesky-only", 10)}},
	}

	got := postedSince(logs, since)

	if len(got) != 2 || !got["both"] || !got["bluesky-only"] {
		t.Errorf("postedSince() = %v, want both and bluesky-only", got)
	}
}

func TestKeepPostedDropsVotesGroupingPulledIn(t *testing.T) {
	multi := testfixtures.MultiVoteGroup()
	groups := [][]votes.Vote{multi, testfixtures.SingleVoteAbgelehnt()}
	ids := map[string]bool{multi[1].SourceID: true, "never-fetched": true}

	got := keepPosted(groups, ids)

	if len(got) != 1 || len(got[0]) != 1 || got[0][0].SourceID != multi[1].SourceID {
		t.Fatalf("keepPosted() = %v, want only %s", got, multi[1].SourceID)
	}
	if lost := missing(ids, got); len(lost) != 1 || lost[0] != "never-fetched" {
		t.Errorf("missing() = %v, want [never-fetched]", lost)
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
	since := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)

	got := render(sections, since, since.AddDate(0, 0, 7), mapper)

	post, rest, _ := strings.Cut(got, "━━━ First comment ━━━")
	comment, tags, _ := strings.Cut(rest, "━━━ Tag by hand ━━━")

	for _, want := range []string{
		"🗳️ Gemeinderat Stadt Zürich | 08.11.2026 – 15.11.2026\n",
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
	if got := render([]section{{Name: "Nationalrat"}}, time.Now(), time.Now(), nil); got != "" {
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
