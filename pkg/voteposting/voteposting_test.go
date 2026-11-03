package voteposting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

const testJurisdiction = "zurich-city"

// setupTempDir creates a temp directory for tests and changes to it
// Returns a cleanup function that should be deferred
func setupTempDir(t *testing.T) func() {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()

	// Create data directory in temp location
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	return func() {
		_ = os.Chdir(oldWd)
	}
}

// MockPlatform is a test implementation of the Platform interface
type MockPlatform struct {
	formatCalls    int
	postCalls      int
	maxPosts       int
	shouldFailPost bool
	// partialPost makes Post fail after publishing: it returns a ref and an
	// error, as a thread does when the root goes out and a reply is refused.
	partialPost bool
	// lastGroup records what the formatter was actually handed, so tests can
	// assert on votes dropped before formatting.
	lastGroup []votes.Vote
}

type MockContent struct {
	text string
}

func (c *MockContent) String() string {
	return c.text
}

func (p *MockPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	p.formatCalls++
	p.lastGroup = group
	return &MockContent{text: "mock post"}, nil
}

func (p *MockPlatform) Post(content platforms.Content) (votelog.PostRef, bool, error) {
	if p.shouldFailPost {
		return votelog.PostRef{}, false, errors.New("mock posting error")
	}
	p.postCalls++
	shouldContinue := p.postCalls < p.maxPosts
	id := fmt.Sprintf("post%d", p.postCalls)
	if p.partialPost {
		return votelog.PostRef{ID: id, URL: "https://example.test/" + id}, false, errors.New("mock reply error")
	}
	return votelog.PostRef{ID: id, URL: "https://example.test/" + id}, shouldContinue, nil
}

func (p *MockPlatform) MaxPostsPerRun() int {
	return p.maxPosts
}

func (p *MockPlatform) Name() string {
	return "Mock"
}

// Test helper to create a test group that passes validateGroup: a handled vote
// type, and a non-zero Ja count (all-zero counts are treated as unsupported).
func createVote(guid, affair, date string) votes.Vote {
	ja := 100
	return votes.Vote{
		SourceID:     guid,
		Jurisdiction: testJurisdiction,
		Date:         testfixtures.MustDate(date),
		Type:         "Normal",
		Yes:          &ja,
		Affair:       votes.Affair{Number: affair},
	}
}

func TestFilterUnpostedVotes(t *testing.T) {
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)
	voteLog.MarkAsPosted("vote1", votelog.PostRef{})
	voteLog.MarkAsPosted("vote3", votelog.PostRef{})

	group := []votes.Vote{
		createVote("vote1", "2025/369", "2025-11-19"),
		createVote("vote2", "2025/370", "2025-11-19"),
		createVote("vote3", "2025/371", "2025-11-19"),
		createVote("vote4", "2025/372", "2025-11-19"),
	}

	unposted := filterUnpostedVotes(group, voteLog)

	if len(unposted) != 2 {
		t.Errorf("Expected 2 unposted group, got %d", len(unposted))
	}

	if unposted[0].SourceID != "vote2" || unposted[1].SourceID != "vote4" {
		t.Errorf("Expected vote2 and vote4, got %v", unposted)
	}
}

func TestPostToPlatform_DryRun(t *testing.T) {
	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
		{createVote("vote2", "2025/370", "2025-11-19")},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), true)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// In dry run, posted counts groups printed (not real API calls)
	if posted != 2 {
		t.Errorf("Expected 2 printed in dry run, got %d", posted)
	}

	// Format should be called for each group
	if mockPlatform.formatCalls != 2 {
		t.Errorf("Expected 2 format calls, got %d", mockPlatform.formatCalls)
	}

	// Post should NOT be called in dry run
	if mockPlatform.postCalls != 0 {
		t.Errorf("Expected 0 post calls in dry run, got %d", mockPlatform.postCalls)
	}

	// Vote log should still be empty
	if voteLog.Count() != 0 {
		t.Errorf("Expected empty vote log in dry run, got %d", voteLog.Count())
	}
}

func TestPostToPlatform_RealPosting(t *testing.T) {
	defer setupTempDir(t)()

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
		{createVote("vote2", "2025/370", "2025-11-19"), createVote("vote3", "2025/370", "2025-11-19")},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have posted 2 groups
	if posted != 2 {
		t.Errorf("Expected 2 posted, got %d", posted)
	}

	// Format and post should be called for each group
	if mockPlatform.formatCalls != 2 {
		t.Errorf("Expected 2 format calls, got %d", mockPlatform.formatCalls)
	}

	if mockPlatform.postCalls != 2 {
		t.Errorf("Expected 2 post calls, got %d", mockPlatform.postCalls)
	}

	// All 3 group should be marked as posted
	if voteLog.Count() != 3 {
		t.Errorf("Expected 3 group in log, got %d", voteLog.Count())
	}

	// Check specific group are logged
	if !voteLog.IsPosted("vote1") || !voteLog.IsPosted("vote2") || !voteLog.IsPosted("vote3") {
		t.Error("Not all group were marked as posted")
	}
}

func TestPostToPlatform_LimitRespected(t *testing.T) {
	defer setupTempDir(t)()

	// Platform that stops after 2 posts
	mockPlatform := &MockPlatform{maxPosts: 2}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
		{createVote("vote2", "2025/370", "2025-11-19")},
		{createVote("vote3", "2025/371", "2025-11-19")},
		{createVote("vote4", "2025/372", "2025-11-19")},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have stopped at 2 posts due to platform limit
	if posted != 2 {
		t.Errorf("Expected 2 posted (platform limit), got %d", posted)
	}

	if mockPlatform.postCalls != 2 {
		t.Errorf("Expected 2 post calls, got %d", mockPlatform.postCalls)
	}

	// Only first 2 group should be logged
	if voteLog.Count() != 2 {
		t.Errorf("Expected 2 group in log, got %d", voteLog.Count())
	}
}

func TestPostToPlatform_ErrorHandling(t *testing.T) {
	// Platform that fails on posting
	mockPlatform := &MockPlatform{maxPosts: 10, shouldFailPost: true}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)

	// Should return error
	if err == nil {
		t.Error("Expected error from failed posting")
	}

	// Should not have posted anything
	if posted != 0 {
		t.Errorf("Expected 0 posted on error, got %d", posted)
	}

	// Vote log should be empty
	if voteLog.Count() != 0 {
		t.Errorf("Expected empty vote log on error, got %d", voteLog.Count())
	}
}

func TestFilterUnpostedVotes_AllPosted(t *testing.T) {
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)
	voteLog.MarkAsPosted("vote1", votelog.PostRef{})
	voteLog.MarkAsPosted("vote2", votelog.PostRef{})

	group := []votes.Vote{
		createVote("vote1", "2025/369", "2025-11-19"),
		createVote("vote2", "2025/370", "2025-11-19"),
	}

	unposted := filterUnpostedVotes(group, voteLog)

	if len(unposted) != 0 {
		t.Errorf("Expected 0 unposted group when all are posted, got %d", len(unposted))
	}
}

func TestFilterUnpostedVotes_NonePosted(t *testing.T) {
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	group := []votes.Vote{
		createVote("vote1", "2025/369", "2025-11-19"),
		createVote("vote2", "2025/370", "2025-11-19"),
		createVote("vote3", "2025/371", "2025-11-19"),
	}

	unposted := filterUnpostedVotes(group, voteLog)

	if len(unposted) != 3 {
		t.Errorf("Expected 3 unposted group, got %d", len(unposted))
	}
}

func TestPostToPlatform_EmptyGroups(t *testing.T) {
	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if posted != 0 {
		t.Errorf("Expected 0 posted for empty groups, got %d", posted)
	}

	if mockPlatform.formatCalls != 0 {
		t.Errorf("Expected 0 format calls for empty groups, got %d", mockPlatform.formatCalls)
	}
}

func TestPostToPlatform_AllVotesInGroupAreLogged(t *testing.T) {
	defer setupTempDir(t)()

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	// Create a group with 5 group (simulating a complex Geschäft)
	groups := [][]votes.Vote{
		{
			createVote("vote1", "2025/179", "2025-11-19"),
			createVote("vote2", "2025/179", "2025-11-19"),
			createVote("vote3", "2025/179", "2025-11-19"),
			createVote("vote4", "2025/179", "2025-11-19"),
			createVote("vote5", "2025/179", "2025-11-19"),
		},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// One group should be posted
	if posted != 1 {
		t.Errorf("Expected 1 group posted, got %d", posted)
	}

	// All 5 group should be logged
	if voteLog.Count() != 5 {
		t.Errorf("Expected 5 group logged, got %d", voteLog.Count())
	}

	// Verify each specific vote is logged
	expectedVotes := []string{"vote1", "vote2", "vote3", "vote4", "vote5"}
	for _, voteID := range expectedVotes {
		if !voteLog.IsPosted(voteID) {
			t.Errorf("Expected %s to be logged as posted", voteID)
		}
	}
}

func TestPostToPlatform_MultipleGroupsAllVotesLogged(t *testing.T) {
	defer setupTempDir(t)()

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	// Multiple groups with varying sizes
	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")}, // 1 vote
		{
			createVote("vote2", "2025/370", "2025-11-19"),
			createVote("vote3", "2025/370", "2025-11-19"),
		}, // 2 group
		{
			createVote("vote4", "2025/179", "2025-11-19"),
			createVote("vote5", "2025/179", "2025-11-19"),
			createVote("vote6", "2025/179", "2025-11-19"),
		}, // 3 group
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 3 groups should be posted
	if posted != 3 {
		t.Errorf("Expected 3 groups posted, got %d", posted)
	}

	// All 6 group should be logged (1+2+3)
	if voteLog.Count() != 6 {
		t.Errorf("Expected 6 group logged, got %d", voteLog.Count())
	}

	// Verify each specific vote is logged
	for i := 1; i <= 6; i++ {
		voteID := fmt.Sprintf("vote%d", i)
		if !voteLog.IsPosted(voteID) {
			t.Errorf("Expected %s to be logged as posted", voteID)
		}
	}
}

// TestPostToPlatform_LogsEachVoteWithItsPost checks the audit half of the log:
// every vote carries the ref of the post it went out in, and votes grouped into
// one post share it.
func TestPostToPlatform_LogsEachVoteWithItsPost(t *testing.T) {
	defer setupTempDir(t)()

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
		{
			createVote("vote2", "2025/370", "2025-11-19"),
			createVote("vote3", "2025/370", "2025-11-19"),
		},
	}

	if _, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	want := map[string]votelog.PostRef{
		"vote1": {ID: "post1", URL: "https://example.test/post1"},
		"vote2": {ID: "post2", URL: "https://example.test/post2"},
		"vote3": {ID: "post2", URL: "https://example.test/post2"},
	}
	for _, entry := range voteLog.Votes {
		if entry.PostRef != want[entry.ID] {
			t.Errorf("%s logged with %+v, want %+v", entry.ID, entry.PostRef, want[entry.ID])
		}
	}
}

// TestPostToPlatform_UnhandledVoteTypeIsNotPosted covers the case that made this
// guard necessary: Kanton Zürich publishes Anwesenheitsermittlung (a roll call)
// and the occasional quorum vote with no type at all, and both render as a
// perfectly ordinary lopsided Ja/Nein tally. Nothing about the counts gives them
// away, so only the type can stop them.
//
// The unhandled group must be skipped rather than posted, the handled one must
// still go out, and the run must end in an error so the failure is noticed
// instead of passing silently.
func TestPostToPlatform_UnhandledVoteTypeIsNotPosted(t *testing.T) {
	defer setupTempDir(t)()

	unhandled := createVote("anwesenheit-1", "2026/1", "2026-06-15")
	unhandled.Type = "" // what the source actually serves for a roll call

	handled := createVote("normal-1", "2026/2", "2026-06-15")

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{{unhandled}, {handled}}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)

	if !errors.Is(err, ErrUnsupportedVoteType) {
		t.Fatalf("expected ErrUnsupportedVoteType so the run fails visibly, got %v", err)
	}
	if posted != 1 {
		t.Errorf("expected the handled group to still post, got posted=%d", posted)
	}
	// The skipped vote must not be logged, or it would never be retried once the
	// type becomes renderable.
	if voteLog.IsPosted("anwesenheit-1") {
		t.Error("skipped vote was marked as posted; it would never be revisited")
	}
	if !voteLog.IsPosted("normal-1") {
		t.Error("handled vote was not marked as posted")
	}
}

// TestPostToPlatform_CantonCupVoteIsNotPosted covers the canton's Auswahl
// equivalent, a Cup-Abstimmung: one knockout round between more than two
// competing proposals.
//
// The real records cannot be published truthfully — every aggregate count is
// null, and the per-member rows are duplicated 296-for-180 with 175 members
// carrying a "Präsidium" value harmonised to abstention. Either defect alone
// would make a post wrong; the type check catches it before the counts do.
//
// Both are reported upstream. This pins that until they are fixed the group is
// skipped rather than rendered, and that the run still fails visibly.
func TestPostToPlatform_CantonCupVoteIsNotPosted(t *testing.T) {
	defer setupTempDir(t)()

	group := testfixtures.KantonsratCupVote()

	// Guard the fixture itself: if the source ever starts populating these, the
	// reason this test passes would silently change.
	v := group[0]
	if v.Yes != nil || v.No != nil || v.Abstention != nil {
		t.Fatalf("fixture should carry the null aggregates the source serves, got %v/%v/%v",
			v.Yes, v.No, v.Abstention)
	}
	if voteformat.IsHandledVoteType(v.Type) {
		t.Fatalf("Cup-Abstimmung should not be on the handled list, got type %q", v.Type)
	}

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	posted, err := PostToPlatform([][]votes.Vote{group}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	if !errors.Is(err, ErrUnsupportedVoteType) {
		t.Fatalf("expected ErrUnsupportedVoteType so the run fails visibly, got %v", err)
	}
	if posted != 0 {
		t.Errorf("a knockout round was published as an ordinary vote: posted=%d", posted)
	}
	if mockPlatform.formatCalls != 0 {
		t.Errorf("the group reached the formatter: formatCalls=%d", mockPlatform.formatCalls)
	}
	// Not logging it is what lets it be revisited once upstream is fixed.
	if voteLog.IsPosted(v.SourceID) {
		t.Error("skipped vote was marked as posted; it would never be revisited")
	}
}

// TestPostToPlatform_UnpostableVoteDoesNotSuppressItsGroup is the reason
// rejection is per vote rather than per group.
//
// Votes are grouped by business matter and sitting day, and Kanton Zürich
// interleaves vote types freely inside one business. The real 15.12.2025
// Steuerfuss item carries five ordinary Ja/Nein votes alongside three
// Cup-Abstimmung rounds; the 19.01.2026 Lehrpersonalgesetz item five alongside
// one. Rejecting the whole group would have suppressed the tax-rate decision
// because three procedural rounds in the same business are unrenderable.
func TestPostToPlatform_UnpostableVoteDoesNotSuppressItsGroup(t *testing.T) {
	defer setupTempDir(t)()

	// One business matter, one sitting day, mixed types — as the source serves it.
	substantive := createVote("steuerfuss-normal", "250382", "2025-12-15")
	unpostable := testfixtures.KantonsratCupVote()[0]
	unpostable.Jurisdiction = testJurisdiction
	unpostable.Affair.Number = "250382"
	unpostable.Date = substantive.Date

	group := []votes.Vote{substantive, unpostable}

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	posted, err := PostToPlatform([][]votes.Vote{group}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	// The run still fails, so the gap is noticed...
	if !errors.Is(err, ErrUnsupportedVoteType) {
		t.Fatalf("expected ErrUnsupportedVoteType, got %v", err)
	}
	// ...but the publishable vote is published rather than lost with it.
	if posted != 1 {
		t.Errorf("the substantive vote was suppressed by its neighbour: posted=%d", posted)
	}
	if !voteLog.IsPosted("steuerfuss-normal") {
		t.Error("substantive vote was not marked as posted")
	}
	// The dropped one must stay unlogged so it returns once the source is fixed.
	if voteLog.IsPosted(unpostable.SourceID) {
		t.Error("dropped vote was marked as posted; it would never be revisited")
	}

	// It must also not reach the formatter as part of the group.
	if got := len(mockPlatform.lastGroup); got != 1 {
		t.Errorf("formatter received %d votes, want only the postable one", got)
	}
}

// TestPostToPlatform_AttendanceRollCallIsSkippedWithoutFailingTheRun covers the
// case that made the 17.08.2026 run red for a fortnight.
//
// An attendance roll call is not a political vote, so it must never be
// published — but it is also not a fault. The Kantonsrat takes one at the start
// of most sittings, and nothing upstream needs fixing, so treating it like an
// unrecognised type would leave the action failing on every sitting day while
// the bot did exactly what it should.
func TestPostToPlatform_AttendanceRollCallIsSkippedWithoutFailingTheRun(t *testing.T) {
	defer setupTempDir(t)()

	rollCall := createVote("anwesenheit-1", "250999", "2026-08-17")
	rollCall.Type = "Anwesenheitsermittlung"

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	posted, err := PostToPlatform([][]votes.Vote{{rollCall}}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	if err != nil {
		t.Fatalf("a roll call must not fail the run, got %v", err)
	}
	if posted != 0 {
		t.Errorf("a roll call was published as a vote: posted=%d", posted)
	}
	if mockPlatform.formatCalls != 0 {
		t.Errorf("the roll call reached the formatter: formatCalls=%d", mockPlatform.formatCalls)
	}
	// Not logging it keeps the behaviour identical to any other skipped vote.
	if voteLog.IsPosted(rollCall.SourceID) {
		t.Error("skipped roll call was marked as posted")
	}
}

// TestPostToPlatform_WahlgeschaeftIsPosted covers the opposite case from
// TestPostToPlatform_AttendanceRollCallIsSkippedWithoutFailingTheRun: the roll
// call taken on an election business is the only trace that election leaves,
// so it reaches the formatter and is posted — as a notice carrying no result,
// not as a vote. See votes.IsWahlgeschaeft.
func TestPostToPlatform_WahlgeschaeftIsPosted(t *testing.T) {
	defer setupTempDir(t)()

	wahl := createVote("wahl-praesenz-1", "20/2026", "2026-09-14")
	wahl.Type = votes.AttendanceType
	wahl.Title = "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann"
	wahl.Affair.Type = votes.WahlAffairType

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	posted, err := PostToPlatform([][]votes.Vote{{wahl}}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	if err != nil {
		t.Fatalf("a Wahlgeschäft must not fail the run, got %v", err)
	}
	if posted != 1 {
		t.Errorf("Wahlgeschäft was not published: posted=%d", posted)
	}
	if mockPlatform.formatCalls != 1 {
		t.Errorf("Wahlgeschäft never reached the formatter: formatCalls=%d", mockPlatform.formatCalls)
	}
	if !voteLog.IsPosted(wahl.SourceID) {
		t.Error("published Wahlgeschäft was not marked as posted")
	}
}

// TestPostToPlatform_WahlgeschaeftRollCallsInOneGroup covers the two ways an
// election roll call can share a group, since votes are grouped by business
// and sitting day. Two roll calls — a second ballot round — are one notice
// covering both. A roll call beside a real vote is dropped, because the notice
// path needs the whole group to be roll calls and the ordinary path would
// render its headcount next to a real result.
func TestPostToPlatform_WahlgeschaeftRollCallsInOneGroup(t *testing.T) {
	rollCall := func(id string) votes.Vote {
		v := createVote(id, "20/2026", "2026-09-14")
		v.Type = votes.AttendanceType
		v.Title = "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann"
		v.Affair.Type = votes.WahlAffairType
		return v
	}

	t.Run("two roll calls are one notice", func(t *testing.T) {
		defer setupTempDir(t)()
		first, second := rollCall("wahl-round-1"), rollCall("wahl-round-2")

		mockPlatform := &MockPlatform{maxPosts: 10}
		voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)
		posted, err := PostToPlatform([][]votes.Vote{{first, second}}, mockPlatform,
			SingleLog(testJurisdiction, voteLog), false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if posted != 1 || len(mockPlatform.lastGroup) != 2 {
			t.Fatalf("want one post covering both roll calls, got posted=%d group=%d", posted, len(mockPlatform.lastGroup))
		}
		if !votes.IsWahlgeschaeftGroup(mockPlatform.lastGroup) {
			t.Error("the formatter was handed a group that does not take the notice path")
		}
		for _, id := range []string{first.SourceID, second.SourceID} {
			if !voteLog.IsPosted(id) {
				t.Errorf("%s was not marked as posted", id)
			}
		}
	})

	t.Run("a roll call beside a real vote is dropped", func(t *testing.T) {
		defer setupTempDir(t)()
		call := rollCall("wahl-praesenz-mixed")
		real := createVote("wahl-real-mixed", "20/2026", "2026-09-14")
		real.Affair.Type = votes.WahlAffairType

		mockPlatform := &MockPlatform{maxPosts: 10}
		voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)
		posted, err := PostToPlatform([][]votes.Vote{{call, real}}, mockPlatform,
			SingleLog(testJurisdiction, voteLog), false)

		if err != nil {
			t.Fatalf("a dropped roll call must not fail the run, got %v", err)
		}
		if posted != 1 {
			t.Fatalf("the real vote was not published: posted=%d", posted)
		}
		if len(mockPlatform.lastGroup) != 1 || mockPlatform.lastGroup[0].SourceID != real.SourceID {
			t.Errorf("formatter was handed %+v, want the real vote alone", mockPlatform.lastGroup)
		}
		if voteLog.IsPosted(call.SourceID) {
			t.Error("the dropped roll call was marked as posted")
		}
	})
}

// TestPostToPlatform_AttendanceRollCallDoesNotMaskAnUnknownType pins that the
// two rejection kinds stay distinguishable when both occur in one run. A roll
// call must not swallow the signal that the source served something new.
func TestPostToPlatform_AttendanceRollCallDoesNotMaskAnUnknownType(t *testing.T) {
	defer setupTempDir(t)()

	rollCall := createVote("anwesenheit-2", "250998", "2026-08-17")
	rollCall.Type = "Anwesenheitsermittlung"

	novel := createVote("etwas-neues", "250997", "2026-08-17")
	novel.Type = "Wahlgang"

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	_, err := PostToPlatform([][]votes.Vote{{rollCall}, {novel}}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	if !errors.Is(err, ErrUnsupportedVoteType) {
		t.Fatalf("expected the unknown type to still fail the run, got %v", err)
	}
}

// TestPostToPlatform_InconsistentDecisionIsSkippedButStillFailsTheRun covers the
// 08.09.2026 run, which aborted on one bad record from PARIS.
//
// The source reported the 02.09.2026 Zuweisung on 2026/408 as carried at 56 Ja
// to 58 Nein, and corrected it days later. Publishing that verdict was never an
// option — but neither was what happened instead: the whole run died before its
// first post, taking both jurisdictions and every other platform with it, on
// data nobody here could fix.
//
// So the vote is dropped like any other the pipeline will not render, and the
// run still ends non-zero, because a silent skip on wrong data is how a
// correction goes unnoticed for a fortnight.
func TestPostToPlatform_InconsistentDecisionIsSkippedButStillFailsTheRun(t *testing.T) {
	defer setupTempDir(t)()

	sound := createVote("2026-408-schluss", "2026/408", "2026-09-02")
	contradictory := createVote("2026-408-zuweisung", "2026/408", "2026-09-02")
	ja, nein := 56, 58
	contradictory.Yes, contradictory.No = &ja, &nein
	contradictory.Decision = "Ja"

	mockPlatform := &MockPlatform{maxPosts: 10}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	posted, err := PostToPlatform([][]votes.Vote{{sound, contradictory}}, mockPlatform,
		SingleLog(testJurisdiction, voteLog), false)

	if !errors.Is(err, ErrInconsistentDecision) {
		t.Fatalf("expected ErrInconsistentDecision, got %v", err)
	}
	if !IsRejectedVoteError(err) {
		t.Error("an inconsistent decision must read as a rejected vote, not a posting failure")
	}
	// The rest of the business is published rather than held hostage by it.
	if posted != 1 {
		t.Errorf("posted = %d, want the sound vote posted anyway", posted)
	}
	if !voteLog.IsPosted("2026-408-schluss") {
		t.Error("sound vote was not marked as posted")
	}
	if voteLog.IsPosted("2026-408-zuweisung") {
		t.Error("contradictory vote was marked as posted; it would never be revisited")
	}
	if got := len(mockPlatform.lastGroup); got != 1 {
		t.Errorf("formatter received %d votes, want only the consistent one", got)
	}
}

// The message a rejection carries is the whole interface an operator gets from a
// failed run: they have to look at the vote on the source's own site to decide
// whether PARIS is wrong or we are. Identifying the 2026/408 Zuweisung from an
// error that named only the business number and a Traktandum title full of
// literal newlines took three API calls by hand.
func TestValidateVoteRejectionNamesTheVoteAndLinksToIt(t *testing.T) {
	ja, nein := 56, 58
	v := votes.Vote{
		SourceID:  "ab0946e0bffb459dafd828d032627edb",
		Type:      "Normal",
		Title:     "2026/408 Weisung vom 19.08.2026:\nPräsidialdepartement, Stadtkanzlei,\r\nErklärvideos",
		Subtitle:  "2026_0408 Zuweisung",
		Yes:       &ja,
		No:        &nein,
		Decision:  "Ja",
		SourceURL: "https://www.gemeinderat-zuerich.ch/abstimmungen/detail.php?aid=ab0946e0bffb459dafd828d032627edb",
		Affair: votes.Affair{
			Number: "2026/408",
			Title:  "Präsidialdepartement, Stadtkanzlei, Erklärvideos zu den kommunalen Abstimmungsvorlagen, neue wiederkehrende Ausgaben",
		},
	}

	err := validateVote(v)
	if !errors.Is(err, ErrInconsistentDecision) {
		t.Fatalf("validateVote() = %v, want ErrInconsistentDecision", err)
	}

	msg := err.Error()
	for _, want := range []string{
		v.SourceURL,     // the link that makes it debuggable
		"2026/408",      // which business
		"Zuweisung",     // which ballot within it
		`Decision="Ja"`, // what the source claims
		"Ja=56 Nein=58", // and what its own counts say
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}

	// Everything but the link stays on one line: PARIS titles carry literal
	// newlines, and folding them in stranded the counts several lines below the
	// vote they belong to.
	head, _, _ := strings.Cut(msg, "\n")
	if !strings.Contains(head, "Ja=56 Nein=58") {
		t.Errorf("counts are not on the first line of the message:\n%s", msg)
	}
}

// A source with no per-vote page must not leave a rejection trailing an empty
// line where the link would be.
func TestValidateVoteRejectionWithoutASourceURL(t *testing.T) {
	v := createVote("no-link", "250999", "2026-08-17")
	v.Type = "Anwesenheitsermittlung"
	v.SourceURL = ""

	err := validateVote(v)
	if !errors.Is(err, ErrUnpostableVoteType) {
		t.Fatalf("validateVote() = %v, want ErrUnpostableVoteType", err)
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("message has a dangling line for the absent link:\n%q", err.Error())
	}
}

// The Kantonsrat repeats a quorum round until enough members have voted, and
// the archive keeps the attempt that drew no votes. Rejecting it as an
// unfamiliar format reddened every run until it left the window, although the
// round that counted was posted.
func TestValidateVoteEmptyQuorumRoundIsRoutine(t *testing.T) {
	v := createVote("quorum-0", "6099", "2026-09-28")
	v.Type = "Quorum"
	zero := 0
	v.Yes, v.No, v.Abstention = &zero, &zero, &zero

	if err := validateVote(v); !errors.Is(err, ErrUnpostableVoteType) {
		t.Fatalf("validateVote() = %v, want ErrUnpostableVoteType", err)
	}

	// Anything else with nothing counted is still a source fault: another
	// type, the Ausgabenbremse (no such empty round has been seen), or a
	// negative count hiding among zeros.
	for _, tc := range []struct {
		name string
		typ  string
		ja   int
	}{
		{"normal", "Normal", 0},
		{"ausgabenbremse", "Ausgabenbremse", 0},
		{"negative", "Quorum", -1},
	} {
		w := v
		w.Type = tc.typ
		ja := tc.ja
		w.Yes = &ja
		if err := validateVote(w); !errors.Is(err, ErrUnsupportedVoteType) {
			t.Errorf("%s: validateVote() = %v, want ErrUnsupportedVoteType", tc.name, err)
		}
	}
}

// A thread whose root went out and whose reply failed is live, so the vote must
// be logged — and persisted — even though the run fails. Otherwise the next run
// publishes it a second time.
func TestPostToPlatform_PartiallyPostedThreadIsStillLogged(t *testing.T) {
	cleanup := setupTempDir(t)
	defer cleanup()

	mockPlatform := &MockPlatform{maxPosts: 10, partialPost: true}
	voteLog := votelog.NewEmpty(testJurisdiction, votelog.PlatformX)

	groups := [][]votes.Vote{
		{createVote("vote1", "2025/369", "2025-11-19")},
		{createVote("vote2", "2025/370", "2025-11-19")},
	}

	posted, err := PostToPlatform(groups, mockPlatform, SingleLog(testJurisdiction, voteLog), false)
	if err == nil {
		t.Fatal("expected the reply failure to be reported")
	}
	if posted != 1 {
		t.Errorf("expected the live post to count as posted, got %d", posted)
	}
	if mockPlatform.postCalls != 1 {
		t.Errorf("expected the run to stop after the failure, got %d post calls", mockPlatform.postCalls)
	}
	if !voteLog.IsPosted("vote1") {
		t.Error("vote1 went out but was not marked as posted")
	}
	if voteLog.IsPosted("vote2") {
		t.Error("vote2 was never posted but is marked as posted")
	}

	reloaded, err := votelog.Load(testJurisdiction, votelog.PlatformX)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.IsPosted("vote1") {
		t.Error("the log was not saved to disk before the error was returned")
	}
}
