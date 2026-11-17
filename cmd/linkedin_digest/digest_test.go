package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
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
	// One sitting, so one post: the Auswahl fixture is dated another day.
	auswahl := testfixtures.AuswahlVote()
	auswahl[0].Date = testfixtures.VoteWithMentions()[0].Date
	sections := []section{
		{Name: "Gemeinderat Stadt Zürich", Groups: [][]votes.Vote{
			testfixtures.VoteWithMentions(),
			testfixtures.MultiVoteGroup(),
			auswahl,
		}},
		{Name: "Kantonsrat Zürich"},
	}
	got, _ := render(sections, mapper, options{})

	post, rest, _ := strings.Cut(got, "━━━ First comment ━━━")
	comment, tags, _ := strings.Cut(rest, "━━━ Tag by hand ━━━")

	for _, want := range []string{
		"🗳️ Gemeinderat Stadt Zürich | Sitzung vom 15.06.2025\n",
		"1. ✅ Angenommen (80 Ja, 30 Nein) · Postulat von Anna Graff (SP) betreffend",
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
	if got, _ := render([]section{{Name: "Nationalrat"}}, nil, options{}); got != "" {
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

	got, _ := render(sections, mapper, options{})

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

func TestRenderPostsEachSittingOnItsOwn(t *testing.T) {
	first := testfixtures.SingleVoteAngenommen()
	second := testfixtures.SingleVoteAbgelehnt()
	second[0].Date = first[0].Date.AddDate(0, 0, 7)
	// The later sitting first, as a source might return them.
	sections := []section{{Name: "Gemeinderat Stadt Zürich", Groups: [][]votes.Vote{second, first}}}

	got, _ := render(sections, nil, options{})

	// Each post numbers from 1 and has its own first comment.
	want := "══════ Beitrag 1 von 2 ══════\n\n━━━ Post ━━━\n🗳️ Gemeinderat Stadt Zürich | Sitzung vom 15.06.2025\n\n1. ✅ Angenommen" +
		"(.|\n)*━━━ First comment ━━━\n1. " +
		"(.|\n)*══════ Beitrag 2 von 2 ══════\n\n━━━ Post ━━━\n🗳️ Gemeinderat Stadt Zürich | Sitzung vom 22.06.2025\n\n1. ❌ Abgelehnt" +
		"(.|\n)*━━━ First comment ━━━\n1. "
	if !regexp.MustCompile(want).MatchString(got) {
		t.Errorf("render() did not give each sitting a post of its own, in date order:\n%s", got)
	}
}

func TestGroupSubjectDropsTheFilingDate(t *testing.T) {
	got := groupSubject(testfixtures.VoteWithMentions())

	if want := "Postulat von Anna Graff (SP) betreffend"; !strings.HasPrefix(got, want) {
		t.Errorf("groupSubject() = %q, want it to start %q", got, want)
	}
}

func TestShortenCutsAtAWord(t *testing.T) {
	for _, c := range []struct {
		in   string
		max  int
		want string
	}{
		{"Verbesserung der Sicherheit, rund um den Hauptbahnhof", 30, "Verbesserung der Sicherheit…"},
		{"kurz", 30, "kurz"},
		{"Verbesserung der Sicherheit", 0, "Verbesserung der Sicherheit"},
	} {
		if got := shorten(c.in, c.max); got != c.want {
			t.Errorf("shorten(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
		if n := len([]rune(shorten(c.in, c.max))); c.max > 0 && n > c.max {
			t.Errorf("shorten(%q, %d) is %d characters", c.in, c.max, n)
		}
	}
}

func TestRenderSittingFitsTheLimit(t *testing.T) {
	var groups [][]votes.Vote
	for i := 0; i < 6; i++ {
		groups = append(groups, testfixtures.SingleVoteAngenommen())
	}
	s := sitting{Body: "Gemeinderat Stadt Zürich", Date: groups[0][0].Date, Groups: groups}
	full, _, _, _ := sittingBlocks(s, nil, options{All: true}, 0)
	limit := chars(full) - 200

	got, cut := renderSitting(s, nil, options{All: true}, limit)

	post, _, _ := strings.Cut(strings.TrimPrefix(got, "━━━ Post ━━━\n"), "\n━━━ First comment")
	if n := chars(post); n > limit {
		t.Errorf("post is %d characters, over the limit of %d:\n%s", n, limit, post)
	}
	if cut != 6 || !strings.Contains(post, "…") {
		t.Errorf("cut %d subjects, want all 6 shortened with an ellipsis:\n%s", cut, post)
	}
	// Cut no further than it had to: one character more would not fit.
	if p, _, _, _ := sittingBlocks(s, nil, options{All: true}, len([]rune(groupSubject(groups[0])))); chars(p) <= limit {
		t.Fatal("test is wrong: the full post fits")
	}
}

// lone is a single-vote business with the given result.
func lone(id, decision string, yes, no int) []votes.Vote {
	g := testfixtures.SingleVoteAngenommen()
	g[0].SourceID = id
	g[0].Title = "Postulat " + id
	g[0].Decision = decision
	g[0].Yes, g[0].No = &yes, &no
	return g
}

func TestStatsLine(t *testing.T) {
	groups := [][]votes.Vote{
		lone("a", "angenommen", 90, 30),
		lone("b", "abgelehnt", 20, 95),
		testfixtures.MultiVoteGroup(), // one adopted, one rejected
		lone("c", "", 70, 50),         // no verdict reported
	}

	got := statsLine(groups)

	if want := "4 Geschäfte, 5 Abstimmungen: 2 angenommen, 2 abgelehnt, 1 ohne ausgewiesenes Resultat."; got != want {
		t.Errorf("statsLine() = %q, want %q", got, want)
	}
}

func TestHighlightsTakeRejectionsThenTheClosest(t *testing.T) {
	multi := testfixtures.MultiVoteGroup() // 90:20 adopted, 40:70 rejected
	groups := [][]votes.Vote{
		lone("wide", "angenommen", 90, 30),
		lone("closest", "angenommen", 61, 59),
		lone("rejected-wide", "abgelehnt", 20, 95),
		lone("third", "angenommen", 70, 50),
		lone("unanimous", "angenommen", 100, 0),
		multi,
		lone("second", "angenommen", 66, 60),
		testfixtures.KantonsratWahlgeschaeft(), // 157:0, records no result
	}

	rejected, closest := highlights(groups, 5)

	ids := func(es []entry) (out []string) {
		for _, e := range es {
			out = append(out, e.group[e.vote].SourceID)
		}
		return out
	}
	if got, want := ids(rejected), []string{multi[1].SourceID, "rejected-wide"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("rejected = %v, want %v, closest first and the business by its rejected vote", got, want)
	}
	if got, want := ids(closest), []string{"closest", "second", "third"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("closest = %v, want %v, filling up to five", got, want)
	}
}

func TestPostHighlightsALargeSittingAndListsASmallOne(t *testing.T) {
	var large [][]votes.Vote
	for i, c := range []struct {
		decision string
		yes, no  int
	}{{"angenommen", 90, 30}, {"angenommen", 61, 59}, {"abgelehnt", 20, 95}, {"angenommen", 70, 50},
		{"angenommen", 100, 0}, {"angenommen", 66, 60}, {"angenommen", 80, 10}} {
		large = append(large, lone(fmt.Sprintf("g%d", i+1), c.decision, c.yes, c.no))
	}
	o := options{Site: "https://parlinfo.ch/de/zh/"}

	got, _ := render([]section{{Name: "Gemeinderat Stadt Zürich", Groups: large}}, nil, o)

	post, rest, _ := strings.Cut(got, "━━━ First comment ━━━")
	comment, _, _ := strings.Cut(rest, "━━━ Tag by hand ━━━")
	want := "7 Geschäfte: 6 angenommen, 1 abgelehnt.\n\n" +
		"Abgelehnt:\n1. ❌ 20 Ja, 95 Nein · Postulat g3\n\n" +
		"Am knappsten:\n2. ✅ 61 Ja, 59 Nein · Postulat g2\n3. ✅ 66 Ja, 60 Nein · Postulat g6\n" +
		"4. ✅ 70 Ja, 50 Nein · Postulat g4\n5. ✅ 90 Ja, 30 Nein · Postulat g1\n\n" +
		"Alle 7 Geschäfte einzeln, mit Resultat nach Fraktion: Link im ersten Kommentar.\n"
	if !strings.Contains(post, want) {
		t.Errorf("post = %s\nwant it to contain:\n%s", post, want)
	}
	// With a parlinfo.ch page, that page is the comment's only link, beside
	// the credit the source's terms require.
	if want := "Alle Abstimmungen einzeln: https://parlinfo.ch/de/zh/\nDaten: Open Data Zürich"; strings.TrimSpace(comment) != want {
		t.Errorf("first comment = %q, want %q", comment, want)
	}

	small, _ := render([]section{{Name: "Gemeinderat Stadt Zürich", Groups: large[:3]}}, nil, o)

	if strings.Contains(small, "Am knappsten") || !strings.Contains(small, "3. ❌ Abgelehnt (20 Ja, 95 Nein) · Postulat g3") ||
		!strings.Contains(small, "Jede Abstimmung einzeln") {
		t.Errorf("a sitting of three should be listed whole:\n%s", small)
	}
}

func TestWriteCardsAttachesTheFirstCardOfEachEntry(t *testing.T) {
	var large [][]votes.Vote
	for i, c := range []struct {
		decision string
		yes, no  int
	}{{"angenommen", 90, 30}, {"angenommen", 61, 59}, {"abgelehnt", 20, 95}, {"angenommen", 70, 50},
		{"angenommen", 100, 0}, {"angenommen", 66, 60}} {
		large = append(large, lone(fmt.Sprintf("g%d", i+1), c.decision, c.yes, c.no))
	}
	multi := testfixtures.MultiVoteGroup()
	sections := []section{
		{Name: "Gemeinderat Stadt Zürich", Groups: large},
		{Name: "Kantonsrat Zürich", Groups: [][]votes.Vote{multi}},
	}

	// The cards follow the post's numbering: the rejection first, then the
	// closest.
	entries := postEntries(sections, false)
	var order []string
	for _, g := range entries[0].Groups {
		order = append(order, g[0].SourceID)
	}
	if got, want := strings.Join(order, " "), "g3 g2 g6 g4 g1"; got != want {
		t.Errorf("first post's entries = %s, want %s", got, want)
	}

	dir := t.TempDir()
	// A card from an earlier run of an entry the post no longer has.
	stale := filepath.Join(dir, cardPrefix(entries[0].Sitting)+"-6.jpg")
	if err := os.WriteFile(stale, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCards(dir, sections, false); err != nil {
		t.Fatal(err)
	}

	got, _ := filepath.Glob(filepath.Join(dir, "*"))
	for i := range got {
		got[i] = filepath.Base(got[i])
	}
	gr := cardPrefix(entries[0].Sitting)
	kr := cardPrefix(entries[1].Sitting)
	want := []string{gr + "-1.jpg", gr + "-2.jpg", gr + "-3.jpg", gr + "-4.jpg", gr + "-5.jpg", gr + "-alt.txt", kr + "-1.jpg", kr + "-alt.txt"}
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("files = %v, want %v: one card per entry, only the first of a multi-vote business, none stale", got, want)
	}

	first, err := imagegen.GenerateCards(multi)
	if err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(filepath.Join(dir, kr+"-1.jpg"))
	if !bytes.Equal(written, first[0].JPEG) {
		t.Error("the multi-vote business's card is not its first card")
	}
}
