package x

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
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

// fakeCards returns a GenerateCardsFunc that renders n placeholder cards.
func fakeCards(n int) GenerateCardsFunc {
	return func([]votes.Vote) ([]imagegen.Card, error) {
		cards := make([]imagegen.Card, n)
		for i := range cards {
			cards[i] = imagegen.Card{JPEG: []byte{byte(i)}, Alt: fmt.Sprintf("card %d", i+1), Width: 1080, Height: 1350}
		}
		return cards, nil
	}
}

// withImages wires fake upload and alt-text calls into p. Uploads return
// "media-<card index>", alt texts are recorded by media ID, and any card whose
// index is in failUploads fails to upload.
func withImages(p *XPlatform, n int, failUploads map[byte]bool) map[string]string {
	alts := map[string]string{}
	p.generateCardsFunc = fakeCards(n)
	p.uploadImageFunc = func(_, _, _, _ string, jpeg []byte) (string, error) {
		if failUploads[jpeg[0]] {
			return "", errors.New("upload refused")
		}
		return fmt.Sprintf("media-%d", jpeg[0]), nil
	}
	p.setAltTextFunc = func(_, _, _, _, mediaID, alt string) error {
		alts[mediaID] = alt
		return nil
	}
	return alts
}

func TestPost_AttachesCardsToRoot(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	alts := withImages(p, 3, nil)

	content, err := p.Format(testfixtures.MultiVoteGroup())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}

	if got := strings.Join(calls[0].mediaIDs, ","); got != "media-0,media-1,media-2" {
		t.Errorf("root media = %s, want all three cards", got)
	}
	for i, c := range calls[1:] {
		if len(c.mediaIDs) != 0 {
			t.Errorf("reply %d carries media %v; three cards fit on the root", i+1, c.mediaIDs)
		}
	}
	if alts["media-1"] != "card 2" {
		t.Errorf("alt text for media-1 = %q", alts["media-1"])
	}
}

// Cards beyond what the thread's replies can hold go out as image-only replies
// at the end, still chained.
func TestPost_OverflowCardsBecomeImageOnlyReplies(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	withImages(p, 30, nil)

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}

	total := 0
	for i, c := range calls {
		if len(c.mediaIDs) > 4 {
			t.Errorf("post %d carries %d images, over X's four", i, len(c.mediaIDs))
		}
		total += len(c.mediaIDs)
		if i > 0 && c.inReplyToTweetID != fmt.Sprintf("fake-tweet-%d", i) {
			t.Errorf("post %d replies to %q, want the post before it", i, c.inReplyToTweetID)
		}
	}
	if total != 30 {
		t.Errorf("posted %d images, want all 30", total)
	}
	if last := calls[len(calls)-1]; last.message != "" {
		t.Errorf("overflow reply should be image-only, has text %q", last.message)
	}
}

func TestPost_FailedUploadDropsOnlyThatImage(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	withImages(p, 6, map[byte]bool{1: true, 4: true, 5: true})

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got := strings.Join(calls[0].mediaIDs, ","); got != "media-0,media-2,media-3" {
		t.Errorf("root media = %s", got)
	}
	for i, c := range calls {
		if c.message == "" && len(c.mediaIDs) == 0 {
			t.Errorf("post %d is empty: an image-only reply whose images all failed should be skipped", i)
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

func TestPost_ImageWithoutAltTextIsDropped(t *testing.T) {
	var calls []tweetCall
	p := newTestXPlatform(&calls, 10)
	withImages(p, 5, nil)
	p.setAltTextFunc = func(_, _, _, _, mediaID, _ string) error {
		if mediaID == "media-1" || mediaID == "media-4" {
			return errors.New("metadata refused")
		}
		return nil
	}

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got := strings.Join(calls[0].mediaIDs, ","); got != "media-0,media-2,media-3" {
		t.Errorf("root media = %s, want the undescribed media-1 left out", got)
	}
	for i, c := range calls {
		for _, id := range c.mediaIDs {
			if id == "media-4" {
				t.Errorf("post %d carries undescribed media-4", i)
			}
		}
		if c.message == "" && len(c.mediaIDs) == 0 {
			t.Errorf("post %d is empty: its only image was undescribed", i)
		}
	}
}
