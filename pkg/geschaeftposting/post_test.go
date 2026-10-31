package geschaeftposting

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

func grNrs(gs []zurichapi.Geschaeft) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.GRNr
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUnposted(t *testing.T) {
	// Two from the same sitting, out of GR order and with "/97" < "/541"
	// numerically but not as strings.
	sameDayLow := testfixtures.MotionMember()
	sameDayLow.OBJGUID, sameDayLow.GRNr = "low", "2026/97"

	undated := testfixtures.PostulatCounted()
	undated.OBJGUID, undated.Beginn = "undated", zurichapi.Zeitraum{}

	noGUID := testfixtures.PostulatFraktionen()
	noGUID.OBJGUID = ""

	posted := testfixtures.MotionCounted()
	log := votelog.NewEmpty("zurich-city", votelog.PlatformX)
	log.MarkAsPosted(posted.OBJGUID, votelog.PostRef{})

	gs := []zurichapi.Geschaeft{
		testfixtures.MotionMember(),        // 2026/541, 30.09.
		testfixtures.SchriftlicheAnfrage(), // unsupported
		posted,                             // already posted
		testfixtures.PostulatCounted(),     // 2026/517, 16.09.
		sameDayLow,                         // 2026/97, 30.09.
		testfixtures.MotionMember(),        // duplicate
		undated,                            // no date
		noGUID,                             // no ID to record
		testfixtures.MotionFraktion(),      // 26.08., before cutoff
	}
	cutoff := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	got := grNrs(Unposted(gs, log, cutoff))
	want := []string{"2026/517", "2026/97", "2026/541"}
	if !equal(got, want) {
		t.Errorf("Unposted = %v, want %v", got, want)
	}
}

// fakePoster records what it was asked to post and stops at its budget.
type fakePoster struct {
	budget int
	posted []string
	failAt int // 1-based post that fails; 0 never
}

func (p *fakePoster) Post(c platforms.Content) (votelog.PostRef, bool, error) {
	if p.failAt == len(p.posted)+1 {
		return votelog.PostRef{}, false, errors.New("boom")
	}
	p.posted = append(p.posted, c.String())
	return votelog.PostRef{ID: "id-" + c.String(), URL: "https://example.test/" + c.String()}, len(p.posted) < p.budget, nil
}
func (p *fakePoster) MaxPostsPerRun() int { return p.budget }
func (p *fakePoster) Name() string        { return "fake" }

type textContent string

func (c textContent) String() string { return string(c) }

func grFormat(g zurichapi.Geschaeft) (platforms.Content, error) { return textContent(g.GRNr), nil }

// inTempDir runs the test where a log can be saved without touching data/.
func inTempDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestPostStopsAtBudgetAndRecordsWhatWentOut(t *testing.T) {
	inTempDir(t)
	gs := []zurichapi.Geschaeft{testfixtures.MotionMember(), testfixtures.PostulatCounted(), testfixtures.MotionCounted()}
	log := votelog.NewEmpty("zurich-city", votelog.PlatformBluesky)
	p := &fakePoster{budget: 2}

	n, err := Post(gs, grFormat, p, log, false)
	if err != nil || n != 2 {
		t.Fatalf("Post = %d, %v; want 2, nil", n, err)
	}
	if !equal(p.posted, []string{"2026/541", "2026/517"}) {
		t.Errorf("posted %v", p.posted)
	}
	if !log.IsPosted(gs[0].OBJGUID) || !log.IsPosted(gs[1].OBJGUID) || log.IsPosted(gs[2].OBJGUID) {
		t.Error("log does not match what was posted")
	}
	if !votelog.Exists(votelog.KindVotes, "zurich-city", votelog.PlatformBluesky) {
		t.Error("log not saved after posting")
	}
	// The entry records where the announcement went, as vote entries do.
	if got := log.Votes[0].PostRef; got.ID != "id-2026/541" || got.URL != "https://example.test/2026/541" {
		t.Errorf("first entry's post ref = %+v", got)
	}
}

// A failed post must not be recorded, or it would never be retried — but what
// went out before it must be, or it would be posted twice.
func TestPostFailureKeepsEarlierRecords(t *testing.T) {
	inTempDir(t)
	gs := []zurichapi.Geschaeft{testfixtures.MotionMember(), testfixtures.PostulatCounted()}
	log := votelog.NewEmpty("zurich-city", votelog.PlatformX)
	p := &fakePoster{budget: 10, failAt: 2}

	n, err := Post(gs, grFormat, p, log, false)
	if err == nil || n != 1 {
		t.Fatalf("Post = %d, %v; want 1 and an error", n, err)
	}
	if !log.IsPosted(gs[0].OBJGUID) {
		t.Error("the post that went out was not recorded")
	}
	if log.IsPosted(gs[1].OBJGUID) {
		t.Error("the failed post was recorded")
	}
}

func TestPostDryRunPostsAndRecordsNothing(t *testing.T) {
	gs := []zurichapi.Geschaeft{testfixtures.MotionMember(), testfixtures.PostulatCounted(), testfixtures.MotionCounted()}
	log := votelog.NewEmpty("zurich-city", votelog.PlatformX)
	p := &fakePoster{budget: 2}

	n, err := Post(gs, grFormat, p, log, true)
	if err != nil || n != 2 {
		t.Fatalf("dry Post = %d, %v; want 2 (the budget), nil", n, err)
	}
	if len(p.posted) != 0 || log.Count() != 0 {
		t.Errorf("dry run posted %v and recorded %d", p.posted, log.Count())
	}
}

func TestSeedRecordsWithoutPosting(t *testing.T) {
	inTempDir(t)
	gs := []zurichapi.Geschaeft{testfixtures.MotionMember(), testfixtures.PostulatCounted()}
	log, err := votelog.LoadKind(votelog.KindGeschaefte, "zurich-city", votelog.PlatformInstagram)
	if err != nil {
		t.Fatal(err)
	}
	if err := Seed(gs, log); err != nil {
		t.Fatal(err)
	}
	if !votelog.Exists(votelog.KindGeschaefte, "zurich-city", votelog.PlatformInstagram) {
		t.Fatal("seeded log not written")
	}
	if got := Unposted(gs, log, time.Time{}); len(got) != 0 {
		t.Errorf("seeded Geschäfte still unposted: %v", grNrs(got))
	}
}

func TestPostZeroBudgetPostsNothing(t *testing.T) {
	gs := []zurichapi.Geschaeft{testfixtures.MotionMember()}
	for _, dry := range []bool{false, true} {
		log := votelog.NewEmpty("zurich-city", votelog.PlatformX)
		p := &fakePoster{budget: 0}
		n, err := Post(gs, grFormat, p, log, dry)
		if err != nil || n != 0 || len(p.posted) != 0 || log.Count() != 0 {
			t.Errorf("dry=%v: Post = %d, %v, posted %v, recorded %d; want nothing", dry, n, err, p.posted, log.Count())
		}
	}
}
