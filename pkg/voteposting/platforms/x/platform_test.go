package x

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// tweetCall records a single call to the mock postTweetFunc.
type tweetCall struct {
	message          string
	inReplyToTweetID string
	mediaIDs         []string
}

// mockPostTweet returns a PostTweetFunc that records calls and returns sequential fake tweet IDs.
func mockPostTweet(calls *[]tweetCall) PostTweetFunc {
	counter := 0
	return func(apiKey, apiSecret, accessToken, accessSecret, message, inReplyToTweetID string, mediaIDs []string) (string, error) {
		*calls = append(*calls, tweetCall{message: message, inReplyToTweetID: inReplyToTweetID, mediaIDs: mediaIDs})
		counter++
		return fmt.Sprintf("fake-tweet-%d", counter), nil
	}
}

// newTestXPlatform creates an XPlatform with a mock postTweetFunc.
func newTestXPlatform(calls *[]tweetCall, maxPostsPerRun int) *XPlatform {
	return &XPlatform{
		apiKey:         "test-key",
		apiSecret:      "test-secret",
		accessToken:    "test-token",
		accessSecret:   "test-access-secret",
		maxPostsPerRun: maxPostsPerRun,
		postsThisRun:   0,
		postTweetFunc:  mockPostTweet(calls),
	}
}

func TestPost_SingleTweet(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)

	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	_, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if !shouldContinue {
		t.Error("expected shouldContinue=true")
	}

	if len(calls) < 1 {
		t.Fatalf("expected at least 1 call, got %d", len(calls))
	}

	// Root tweet should have no reply ID
	if calls[0].inReplyToTweetID != "" {
		t.Errorf("root tweet should have empty inReplyToTweetID, got %q", calls[0].inReplyToTweetID)
	}
}

func TestPost_ThreadChaining(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)

	// Use multi-vote group to generate a thread with replies
	votes := testfixtures.MultiVoteGroup()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	post, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if !shouldContinue {
		t.Error("expected shouldContinue=true")
	}

	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls (root + reply), got %d", len(calls))
	}

	// Root has no reply ID
	if calls[0].inReplyToTweetID != "" {
		t.Errorf("root should have empty inReplyToTweetID, got %q", calls[0].inReplyToTweetID)
	}

	// Each reply should reference the previous tweet ID
	for i := 1; i < len(calls); i++ {
		expectedParent := fmt.Sprintf("fake-tweet-%d", i)
		if calls[i].inReplyToTweetID != expectedParent {
			t.Errorf("call %d: expected inReplyToTweetID=%q, got %q", i, expectedParent, calls[i].inReplyToTweetID)
		}
	}

	// The logged ref is the root tweet, not the last reply
	want := votelog.PostRef{ID: "fake-tweet-1", URL: "https://x.com/i/status/fake-tweet-1"}
	if post != want {
		t.Errorf("expected ref %+v, got %+v", want, post)
	}
}

func TestPost_EmptyThread(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)

	// Create empty content
	content := &XContent{thread: []*XPost{}}
	_, _, err := p.Post(content)
	if err == nil {
		t.Error("expected error for empty thread")
	}
}

func TestPost_PostLimitReached(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 1)

	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	_, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if shouldContinue {
		t.Error("expected shouldContinue=false after reaching maxPostsPerRun=1")
	}
}

// withImages wires real-count placeholder cards and fake upload and alt-text
// calls into p. Card k's upload returns "media-k" unless k is in failUploads,
// and its alt text fails if k is in failAlts.
func withImages(p *XPlatform, failUploads, failAlts map[int]bool) {
	p.generateCardsFunc = func(group []votes.Vote) ([]imagegen.Card, error) {
		n := 1
		if len(group) > 1 && !votes.IsWahlgeschaeftGroup(group) {
			n = len(group) + 1
		}
		cards := make([]imagegen.Card, n)
		for i := range cards {
			cards[i] = imagegen.Card{JPEG: []byte{byte(i)}, Alt: fmt.Sprintf("card %d", i)}
		}
		return cards, nil
	}
	p.uploadImageFunc = func(_, _, _, _ string, jpeg []byte) (string, error) {
		if failUploads[int(jpeg[0])] {
			return "", errors.New("upload refused")
		}
		return fmt.Sprintf("media-%d", jpeg[0]), nil
	}
	p.setAltTextFunc = func(_, _, _, _, mediaID, _ string) error {
		var k int
		_, _ = fmt.Sscanf(mediaID, "media-%d", &k)
		if failAlts[k] {
			return errors.New("metadata refused")
		}
		return nil
	}
}

// Every post's image shows what that post's text says: the root carries the
// overview, and each vote's result card rides on the reply that opens that
// vote — never on a reply about another vote.
func TestPost_EachCardOnItsVotesReply(t *testing.T) {
	for _, tc := range []struct {
		name  string
		group []votes.Vote
	}{
		{"multi-vote", testfixtures.MultiVoteGroup()},
		{"ten votes", testfixtures.TenVoteStressTest()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []tweetCall
			p := newTestXPlatform(&calls, 10)
			withImages(p, nil, nil)

			content, err := p.Format(tc.group)
			if err != nil {
				t.Fatalf("Format error: %v", err)
			}
			if _, _, err := p.Post(content); err != nil {
				t.Fatalf("Post error: %v", err)
			}

			if got := strings.Join(calls[0].mediaIDs, ","); got != "media-0" {
				t.Errorf("root media = %q, want the overview card alone", got)
			}
			seen := map[string]bool{}
			for i, c := range calls[1:] {
				if len(c.mediaIDs) > 1 {
					t.Errorf("reply %d carries %d images, want at most one", i+1, len(c.mediaIDs))
				}
				for _, id := range c.mediaIDs {
					var k int
					_, _ = fmt.Sscanf(id, "media-%d", &k)
					v := k - 1
					label := voteformat.SubVoteLabel(tc.group[v], v, len(tc.group))
					if !strings.Contains(c.message, label) {
						t.Errorf("reply %d carries vote %d's card but its text is about something else:\n%s", i+1, v+1, c.message)
					}
					for w, other := range tc.group {
						if w != v && strings.Contains(c.message, voteformat.SubVoteLabel(other, w, len(tc.group))) {
							t.Errorf("reply %d carries vote %d's card but also talks about vote %d:\n%s", i+1, v+1, w+1, c.message)
						}
					}
					seen[id] = true
				}
			}
			if len(seen) != len(tc.group) {
				t.Errorf("posted %d result cards, want one per vote (%d)", len(seen), len(tc.group))
			}
		})
	}
}

func TestPost_LoneVoteCardOnRootOnly(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	withImages(p, nil, nil)

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got := strings.Join(calls[0].mediaIDs, ","); got != "media-0" {
		t.Errorf("root media = %q, want the combined card", got)
	}
	for i, c := range calls[1:] {
		if len(c.mediaIDs) != 0 {
			t.Errorf("reply %d carries %v; a lone vote's card is on the root", i+1, c.mediaIDs)
		}
	}
}

// A card that fails to upload or to take its alt text is left out; its reply
// still posts its text.
func TestPost_FailedImageKeepsTheReplyText(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	withImages(p, map[int]bool{1: true}, map[int]bool{2: true})

	group := testfixtures.MultiVoteGroup()
	content, err := p.Format(group)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	for i, c := range calls {
		for _, id := range c.mediaIDs {
			if id == "media-1" || id == "media-2" {
				t.Errorf("post %d carries %s, which failed", i, id)
			}
		}
	}
	for v := 0; v < 2; v++ {
		label := voteformat.SubVoteLabel(group[v], v, len(group))
		found := false
		for _, c := range calls {
			found = found || strings.Contains(c.message, label)
		}
		if !found {
			t.Errorf("vote %d's text was lost with its card", v+1)
		}
	}
}

func TestFormat_RenderFailureKeepsText(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	p.generateCardsFunc = func([]votes.Vote) ([]imagegen.Card, error) { return nil, errors.New("font missing") }

	content, err := p.Format(testfixtures.MultiVoteGroup())
	if err != nil {
		t.Fatalf("Format should fall back to text, got error: %v", err)
	}
	for i, post := range content.(*XContent).thread {
		if len(post.Images) != 0 {
			t.Errorf("post %d has images after a render failure", i)
		}
		if post.Text == "" {
			t.Errorf("post %d lost its text", i)
		}
	}
}
