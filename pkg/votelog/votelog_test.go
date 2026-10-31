package votelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testJurisdiction stands in for a real body, so the path layout is exercised
// alongside the log contents.
const testJurisdiction = "zurich-city"

// setupTempDir runs the test in a scratch working directory with a data/
// directory, since log paths are relative to the working directory.
// The returned function restores the previous directory.
func setupTempDir(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.MkdirAll(filepath.Join(tmpDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(oldWd) }
}

func TestNewEmpty(t *testing.T) {
	log := NewEmpty(testJurisdiction, PlatformX)

	if log.Platform != PlatformX {
		t.Errorf("Expected platform X, got %s", log.Platform)
	}

	if len(log.Votes) != 0 {
		t.Errorf("Expected empty votes, got %d", len(log.Votes))
	}

	if log.index == nil {
		t.Error("Expected index to be initialized")
	}
}

func TestMarkAsPosted(t *testing.T) {
	log := NewEmpty(testJurisdiction, PlatformX)

	log.MarkAsPosted("vote1", PostRef{})
	log.MarkAsPosted("vote2", PostRef{})

	if len(log.Votes) != 2 {
		t.Errorf("Expected 2 votes, got %d", len(log.Votes))
	}

	if !log.IsPosted("vote1") {
		t.Error("Expected vote1 to be posted")
	}

	if !log.IsPosted("vote2") {
		t.Error("Expected vote2 to be posted")
	}
}

func TestMarkAsPosted_NoDuplicates(t *testing.T) {
	log := NewEmpty(testJurisdiction, PlatformX)

	log.MarkAsPosted("vote1", PostRef{})
	log.MarkAsPosted("vote1", PostRef{})
	log.MarkAsPosted("vote1", PostRef{})

	if len(log.Votes) != 1 {
		t.Errorf("Expected 1 vote (no duplicates), got %d", len(log.Votes))
	}

	if log.Votes[0].ID != "vote1" {
		t.Errorf("Expected vote1, got %s", log.Votes[0].ID)
	}
}

func TestIsPosted(t *testing.T) {
	log := NewEmpty(testJurisdiction, PlatformX)

	log.MarkAsPosted("vote1", PostRef{})

	if !log.IsPosted("vote1") {
		t.Error("Expected vote1 to be posted")
	}

	if log.IsPosted("vote2") {
		t.Error("Expected vote2 to NOT be posted")
	}
}

func TestMarkMultipleVotes_SimulatingGroupPost(t *testing.T) {
	log := NewEmpty(testJurisdiction, PlatformX)

	// Simulate posting a group of 3 votes (like a Geschäft with multiple votes)
	voteGroup := []string{"vote1", "vote2", "vote3"}

	for _, voteID := range voteGroup {
		log.MarkAsPosted(voteID, PostRef{})
	}

	// Verify all votes are marked as posted
	if len(log.Votes) != 3 {
		t.Errorf("Expected 3 votes logged, got %d", len(log.Votes))
	}

	for _, voteID := range voteGroup {
		if !log.IsPosted(voteID) {
			t.Errorf("Expected %s to be posted", voteID)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	defer setupTempDir(t)()

	// Create and save log
	log := NewEmpty(testJurisdiction, PlatformX)
	log.MarkAsPosted("vote1", PostRef{})
	log.MarkAsPosted("vote2", PostRef{})
	log.MarkAsPosted("vote3", PostRef{})

	if err := log.Save(); err != nil {
		t.Fatalf("Failed to save log: %v", err)
	}

	// Load log
	loadedLog, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatalf("Failed to load log: %v", err)
	}

	// Verify loaded data
	if len(loadedLog.Votes) != 3 {
		t.Errorf("Expected 3 votes after loading, got %d", len(loadedLog.Votes))
	}

	if !loadedLog.IsPosted("vote1") || !loadedLog.IsPosted("vote2") || !loadedLog.IsPosted("vote3") {
		t.Error("Expected all votes to be marked as posted after loading")
	}
}

// TestSaveAndLoad_PostRef pins the on-disk shape: the ref sits flat on the entry
// as post_id/post_url, and survives a round trip.
func TestSaveAndLoad_PostRef(t *testing.T) {
	defer setupTempDir(t)()

	ref := PostRef{ID: "1234567890", URL: "https://x.com/i/status/1234567890"}
	log := NewEmpty(testJurisdiction, PlatformX)
	log.MarkAsPosted("vote1", ref)
	if err := log.Save(); err != nil {
		t.Fatalf("Failed to save log: %v", err)
	}

	data, err := os.ReadFile(LogFilePath(testJurisdiction, PlatformX))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"post_id": "1234567890"`, `"post_url": "https://x.com/i/status/1234567890"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved log lacks %s:\n%s", want, data)
		}
	}

	loaded, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Votes[0].PostRef; got != ref {
		t.Errorf("loaded ref %+v, want %+v", got, ref)
	}
}

// TestLoad_EntriesWithoutPostRef covers the logs already on the state branch,
// written before refs were recorded: they load as posted, with an empty ref,
// and an empty ref is not written back as empty fields.
func TestLoad_EntriesWithoutPostRef(t *testing.T) {
	defer setupTempDir(t)()

	path := LogFilePath(testJurisdiction, PlatformX)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"platform": "x", "votes": [{"id": "vote1", "posted_at": "2025-11-19T10:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	log, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatal(err)
	}
	if !log.IsPosted("vote1") {
		t.Error("Expected vote1 to be posted")
	}
	if log.Votes[0].PostRef != (PostRef{}) {
		t.Errorf("Expected empty ref, got %+v", log.Votes[0].PostRef)
	}

	if err := log.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "post_") {
		t.Errorf("empty ref written to disk:\n%s", data)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	// Change to temp directory (no data dir created)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Load should return empty log without error
	log, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if len(log.Votes) != 0 {
		t.Errorf("Expected empty log, got %d votes", len(log.Votes))
	}

	if log.Platform != PlatformX {
		t.Errorf("Expected platform X, got %s", log.Platform)
	}
}

func TestPersistenceAcrossMultipleSaves(t *testing.T) {
	defer setupTempDir(t)()

	// First save
	log1 := NewEmpty(testJurisdiction, PlatformX)
	log1.MarkAsPosted("vote1", PostRef{})
	log1.MarkAsPosted("vote2", PostRef{})
	if err := log1.Save(); err != nil {
		t.Fatal(err)
	}

	// Second save (simulating a new run adding more votes)
	log2, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatal(err)
	}
	log2.MarkAsPosted("vote3", PostRef{})
	log2.MarkAsPosted("vote4", PostRef{})
	if err := log2.Save(); err != nil {
		t.Fatal(err)
	}

	// Load and verify all votes are there
	log3, err := Load(testJurisdiction, PlatformX)
	if err != nil {
		t.Fatal(err)
	}

	if len(log3.Votes) != 4 {
		t.Errorf("Expected 4 votes total, got %d", len(log3.Votes))
	}

	expectedVotes := []string{"vote1", "vote2", "vote3", "vote4"}
	for _, voteID := range expectedVotes {
		if !log3.IsPosted(voteID) {
			t.Errorf("Expected %s to be posted", voteID)
		}
	}
}

func TestNewNoOp(t *testing.T) {
	log := NewNoOp(testJurisdiction, PlatformX)

	// IsPosted always returns false
	log.MarkAsPosted("vote1", PostRef{}) // should be a no-op
	if log.IsPosted("vote1") {
		t.Error("NewNoOp: IsPosted should always return false")
	}

	// Votes slice should remain empty (MarkAsPosted is a no-op)
	if len(log.Votes) != 0 {
		t.Errorf("NewNoOp: expected 0 votes, got %d", len(log.Votes))
	}

	// Save should be a no-op (no error, no file written)
	if err := log.Save(); err != nil {
		t.Errorf("NewNoOp: Save should return nil, got %v", err)
	}
}

func TestLogFilePath(t *testing.T) {
	if got, want := LogFilePath("zurich-city", PlatformInstagram), "data/zurich-city/posted_votes_instagram.json"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
	if got, want := LogFilePath("zurich-canton", PlatformX), "data/zurich-canton/posted_votes_x.json"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// A Geschäft log lives beside the vote log, not in it: an ID marked in one must
// not read as posted in the other.
func TestLoadKind_SeparateFromVotes(t *testing.T) {
	defer setupTempDir(t)()

	if got, want := KindFilePath(KindGeschaefte, testJurisdiction, PlatformBluesky),
		filepath.Join("data", testJurisdiction, "posted_geschaefte_bluesky.json"); got != want {
		t.Fatalf("KindFilePath = %q, want %q", got, want)
	}

	if Exists(KindGeschaefte, testJurisdiction, PlatformBluesky) {
		t.Fatal("Exists reported a log that was never written")
	}

	gl, err := LoadKind(KindGeschaefte, testJurisdiction, PlatformBluesky)
	if err != nil {
		t.Fatal(err)
	}
	gl.MarkAsPosted("g1", PostRef{})
	if err := gl.Save(); err != nil {
		t.Fatal(err)
	}

	if !Exists(KindGeschaefte, testJurisdiction, PlatformBluesky) {
		t.Fatal("Exists missed a log that was just written")
	}

	vl, err := Load(testJurisdiction, PlatformBluesky)
	if err != nil {
		t.Fatal(err)
	}
	if vl.IsPosted("g1") {
		t.Error("an ID in the Geschäft log reads as posted in the vote log")
	}

	reloaded, err := LoadKind(KindGeschaefte, testJurisdiction, PlatformBluesky)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.IsPosted("g1") {
		t.Error("Geschäft log lost its entry across a save")
	}
}
