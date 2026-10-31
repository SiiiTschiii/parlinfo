package votelog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Platform represents a social media platform
type Platform string

const (
	PlatformX         Platform = "x"
	PlatformBluesky   Platform = "bluesky"
	PlatformInstagram Platform = "instagram"
	PlatformFacebook  Platform = "facebook"
	PlatformMastodon  Platform = "mastodon"
)

// dataDir is the root the logs live under, relative to the working directory.
const dataDir = "data"

// Kind names what a log records, and is the stem of its file name. Votes and
// Geschäfte are deduplicated separately because their IDs come from different
// PARIS indexes, and because a Geschäft being announced says nothing about
// whether its vote has been.
type Kind string

const (
	KindVotes      Kind = "votes"
	KindGeschaefte Kind = "geschaefte"
)

// PostRef identifies the post a vote went out in, so the log doubles as an
// audit trail from a vote to what the bot published for it.
//
// Both fields are optional: entries written before refs were recorded have
// neither, and a platform that published but could not resolve a URL (an
// Instagram permalink lookup failing, say) keeps the ID alone. The ID is kept
// beside the URL because URL schemes change and tooling needs the ID anyway.
type PostRef struct {
	ID  string `json:"post_id,omitempty"`
	URL string `json:"post_url,omitempty"`
}

// VoteEntry represents a single posted vote
type VoteEntry struct {
	ID       string    `json:"id"`
	PostedAt time.Time `json:"posted_at"`
	PostRef
}

// VoteLog tracks posted votes for one jurisdiction on one platform.
//
// The pairing matters: dedup is per account *and* per body, because a vote
// posted to X has not thereby been posted to Bluesky, and two jurisdictions
// sharing an account still keep separate histories.
type VoteLog struct {
	Jurisdiction string               `json:"jurisdiction,omitempty"`
	Platform     Platform             `json:"platform"`
	Votes        []VoteEntry          `json:"votes"`
	filepath     string               // not exported, internal use
	index        map[string]VoteEntry // for fast lookup
	noOp         bool                 // when true, all votes are treated as unposted
}

// Load reads a jurisdiction's log for one platform.
//
// A missing file yields an empty log with no error, which reads as "nothing was
// ever posted". That is the correct behaviour for a genuinely new jurisdiction
// and a dangerous one for an existing jurisdiction whose log went missing — see
// the workflow's restore step, which treats a missing state branch as fatal for
// exactly this reason.
func Load(jurisdiction string, platform Platform) (*VoteLog, error) {
	return LoadKind(KindVotes, jurisdiction, platform)
}

// LoadKind is Load for a log of the given kind. The entries are still called
// votes in the file, whatever they record: the format is shared so every log
// round-trips through the same code and the same workflow glob.
func LoadKind(kind Kind, jurisdiction string, platform Platform) (*VoteLog, error) {
	path := KindFilePath(kind, jurisdiction, platform)

	vl := &VoteLog{
		Jurisdiction: jurisdiction,
		Platform:     platform,
		Votes:        []VoteEntry{},
		filepath:     path,
		index:        make(map[string]VoteEntry),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return vl, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	if err := json.Unmarshal(data, vl); err != nil {
		return nil, fmt.Errorf("failed to parse log file: %w", err)
	}

	// The stored file may predate the jurisdiction field, and its path is
	// authoritative either way.
	vl.Jurisdiction = jurisdiction
	vl.Platform = platform
	vl.filepath = path

	for _, entry := range vl.Votes {
		vl.index[entry.ID] = entry
	}

	return vl, nil
}

// IsPosted checks if a vote has been posted
func (l *VoteLog) IsPosted(voteID string) bool {
	if l.noOp {
		return false
	}
	_, exists := l.index[voteID]
	return exists
}

// MarkAsPosted marks a vote as posted in the given post. Votes grouped into
// one post all carry the same ref.
func (l *VoteLog) MarkAsPosted(voteID string, post PostRef) {
	if l.noOp {
		return
	}

	// Don't add duplicates
	if l.IsPosted(voteID) {
		return
	}

	entry := VoteEntry{
		ID:       voteID,
		PostedAt: time.Now(),
		PostRef:  post,
	}

	l.Votes = append(l.Votes, entry)
	l.index[voteID] = entry
}

// Save writes the log to disk
func (l *VoteLog) Save() error {
	if l.noOp {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(l.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	// Write to file
	if err := os.WriteFile(l.filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %w", err)
	}

	return nil
}

// Count returns the number of posted votes
func (l *VoteLog) Count() int {
	return len(l.Votes)
}

// NewEmpty creates an empty vote log (useful for testing or when we want to show all votes)
func NewEmpty(jurisdiction string, platform Platform) *VoteLog {
	return &VoteLog{
		Jurisdiction: jurisdiction,
		Platform:     platform,
		Votes:        []VoteEntry{},
		filepath:     LogFilePath(jurisdiction, platform),
		index:        make(map[string]VoteEntry),
	}
}

// NewNoOp creates a no-op vote log that treats all votes as unposted
// and discards all mark/save operations. Used for manual e2e testing.
func NewNoOp(jurisdiction string, platform Platform) *VoteLog {
	return &VoteLog{
		Jurisdiction: jurisdiction,
		Platform:     platform,
		Votes:        []VoteEntry{},
		index:        make(map[string]VoteEntry),
		noOp:         true,
	}
}

// LogFilePath returns where a jurisdiction's log for a platform is stored.
// The nesting is what lets the CI workflow round-trip every jurisdiction's
// logs with one glob instead of an entry per jurisdiction.
func LogFilePath(jurisdiction string, platform Platform) string {
	return KindFilePath(KindVotes, jurisdiction, platform)
}

// KindFilePath returns where a jurisdiction's log of the given kind for a
// platform is stored.
func KindFilePath(kind Kind, jurisdiction string, platform Platform) string {
	return filepath.Join(dataDir, jurisdiction, fmt.Sprintf("posted_%s_%s.json", kind, platform))
}

// Exists reports whether a log file is on disk. Load cannot say: it answers a
// missing file with an empty log, which is right for a new jurisdiction and
// wrong for one whose state was lost — callers that must tell the two apart
// ask here first.
func Exists(kind Kind, jurisdiction string, platform Platform) bool {
	_, err := os.Stat(KindFilePath(kind, jurisdiction, platform))
	return err == nil
}
