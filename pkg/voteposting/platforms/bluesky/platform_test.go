package bluesky

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/bskyapi"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// recordCall records a single call to the mock createRecordFunc.
type recordCall struct {
	text    string
	facets  []bskyapi.Facet
	replyTo *bskyapi.ReplyRef
	images  []bskyapi.Image
}

// mockCreateRecord returns a CreateRecordFunc that records calls and returns sequential fake PostRefs.
func mockCreateRecord(calls *[]recordCall) CreateRecordFunc {
	counter := 0
	return func(session *bskyapi.Session, text string, facets []bskyapi.Facet, replyTo *bskyapi.ReplyRef, images []bskyapi.Image) (*bskyapi.PostRef, error) {
		*calls = append(*calls, recordCall{text: text, facets: facets, replyTo: replyTo, images: images})
		counter++
		return &bskyapi.PostRef{
			URI: fmt.Sprintf("at://did:fake/app.bsky.feed.post/%d", counter),
			CID: fmt.Sprintf("cid-fake-%d", counter),
		}, nil
	}
}

// mockCreateSession returns a CreateSessionFunc that returns a dummy session and tracks call count.
func mockCreateSession(callCount *int) CreateSessionFunc {
	return func(handle, password string) (*bskyapi.Session, error) {
		*callCount++
		return &bskyapi.Session{
			DID:             "did:plc:fake",
			Handle:          "test.bsky.social",
			AccessJwt:       "fake-jwt",
			ServiceEndpoint: "https://fake.bsky.social",
		}, nil
	}
}

// mockResolveHandle returns a ResolveHandleFunc that returns a synthetic DID.
func mockResolveHandle() ResolveHandleFunc {
	return func(handle string) (string, error) {
		return "did:plc:resolved-" + handle, nil
	}
}

// newTestBlueskyPlatform creates a BlueskyPlatform with all functions mocked.
func newTestBlueskyPlatform(recordCalls *[]recordCall, sessionCallCount *int, maxPostsPerRun int) *BlueskyPlatform {
	return &BlueskyPlatform{
		handle:            "test.bsky.social",
		password:          "test-password",
		maxPostsPerRun:    maxPostsPerRun,
		postsThisRun:      0,
		didCache:          make(map[string]string),
		createRecordFunc:  mockCreateRecord(recordCalls),
		createSessionFunc: mockCreateSession(sessionCallCount),
		resolveHandleFunc: mockResolveHandle(),
	}
}

func TestPost_SinglePost(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)

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

	if len(recordCalls) < 1 {
		t.Fatalf("expected at least 1 createRecord call, got %d", len(recordCalls))
	}

	// Root should have no replyTo
	if recordCalls[0].replyTo != nil {
		t.Errorf("root post should have nil replyTo, got %+v", recordCalls[0].replyTo)
	}
}

func TestPost_ThreadChaining(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)

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

	if len(recordCalls) < 2 {
		t.Fatalf("expected at least 2 createRecord calls, got %d", len(recordCalls))
	}

	// Root has no replyTo
	if recordCalls[0].replyTo != nil {
		t.Errorf("root should have nil replyTo, got %+v", recordCalls[0].replyTo)
	}

	rootURI := "at://did:fake/app.bsky.feed.post/1"
	rootCID := "cid-fake-1"

	// The logged ref is the root post, addressed by DID rather than handle
	want := votelog.PostRef{ID: rootURI, URL: "https://bsky.app/profile/did:fake/post/1"}
	if post != want {
		t.Errorf("expected ref %+v, got %+v", want, post)
	}

	// All replies should reference the root as Root, and the previous post as Parent
	for i := 1; i < len(recordCalls); i++ {
		call := recordCalls[i]
		if call.replyTo == nil {
			t.Fatalf("reply %d should have replyTo, got nil", i)
		}
		// Root ref should always point to the first post
		if call.replyTo.Root.URI != rootURI {
			t.Errorf("reply %d: Root.URI = %q, want %q", i, call.replyTo.Root.URI, rootURI)
		}
		if call.replyTo.Root.CID != rootCID {
			t.Errorf("reply %d: Root.CID = %q, want %q", i, call.replyTo.Root.CID, rootCID)
		}
		// Parent ref should point to the previous post
		expectedParentURI := fmt.Sprintf("at://did:fake/app.bsky.feed.post/%d", i)
		expectedParentCID := fmt.Sprintf("cid-fake-%d", i)
		if call.replyTo.Parent.URI != expectedParentURI {
			t.Errorf("reply %d: Parent.URI = %q, want %q", i, call.replyTo.Parent.URI, expectedParentURI)
		}
		if call.replyTo.Parent.CID != expectedParentCID {
			t.Errorf("reply %d: Parent.CID = %q, want %q", i, call.replyTo.Parent.CID, expectedParentCID)
		}
	}
}

func TestPost_WithMentionFacets(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)

	// Use the mention fixture — it has a politician name that triggers mention matching
	// We need a contactMapper for this, but since we mock resolveHandle, we just need
	// to verify that facets from format are passed through to createRecord
	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	// Manually inject a facet to verify it's passed through
	bskyContent := content.(*BlueskyContent)
	if len(bskyContent.thread) > 0 {
		bskyContent.thread[0].Facets = append(bskyContent.thread[0].Facets,
			bskyapi.LinkFacet(0, 5, "https://example.com"))
	}

	_, _, err = p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}

	if len(recordCalls) < 1 {
		t.Fatalf("expected at least 1 call, got %d", len(recordCalls))
	}

	// Root call should have the facet we injected
	if len(recordCalls[0].facets) == 0 {
		t.Error("expected facets to be passed through to createRecord, got none")
	}
}

func TestPost_SessionLazyAuth(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)

	// Post twice — session should only be created once
	votes := testfixtures.SingleVoteAngenommen()
	content1, _ := p.Format(votes)
	content2, _ := p.Format(votes)

	_, _, err := p.Post(content1)
	if err != nil {
		t.Fatalf("Post 1 error: %v", err)
	}
	_, _, err = p.Post(content2)
	if err != nil {
		t.Fatalf("Post 2 error: %v", err)
	}

	if sessionCalls != 1 {
		t.Errorf("expected createSession to be called once, got %d", sessionCalls)
	}
}

func TestPost_EmptyThread(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)

	content := &BlueskyContent{thread: []*BlueskyPost{}}
	_, _, err := p.Post(content)
	if err == nil {
		t.Error("expected error for empty thread")
	}
}

func TestPost_PostLimitReached(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 1)

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

func TestPostURL(t *testing.T) {
	cases := map[string]string{
		"at://did:plc:abc123/app.bsky.feed.post/3kxyz": "https://bsky.app/profile/did:plc:abc123/post/3kxyz",
		"at://did:plc:abc123/app.bsky.feed.like/3kxyz": "",
		"at://did:plc:abc123/app.bsky.feed.post":       "",
		"https://bsky.app/profile/x/post/y":            "",
		"":                                             "",
	}
	for uri, want := range cases {
		if got := postURL(uri); got != want {
			t.Errorf("postURL(%q) = %q, want %q", uri, got, want)
		}
	}
}

// withImages wires real-count placeholder cards and a fake blob upload into
// p. Card k's blob is {"card":k}; any k in failUploads fails to upload.
func withImages(p *BlueskyPlatform, failUploads map[int]bool) {
	p.generateCardsFunc = func(group []votes.Vote) ([]imagegen.Card, error) {
		n := 1
		if len(group) > 1 && !votes.IsWahlgeschaeftGroup(group) {
			n = len(group) + 1
		}
		cards := make([]imagegen.Card, n)
		for i := range cards {
			cards[i] = imagegen.Card{JPEG: []byte{byte(i)}, Alt: fmt.Sprintf("card %d", i), Width: 1080, Height: 1350}
		}
		return cards, nil
	}
	p.uploadBlobFunc = func(_ *bskyapi.Session, data []byte, mimeType string) (json.RawMessage, error) {
		if mimeType != "image/jpeg" {
			return nil, fmt.Errorf("unexpected mime type %q", mimeType)
		}
		if failUploads[int(data[0])] {
			return nil, errors.New("upload refused")
		}
		return json.RawMessage(fmt.Sprintf(`{"card":%d}`, data[0])), nil
	}
}

// cardOf returns the card index a post's single image carries, or -1.
func cardOf(t *testing.T, images []bskyapi.Image) int {
	t.Helper()
	if len(images) == 0 {
		return -1
	}
	if len(images) > 1 {
		t.Errorf("post carries %d images, want at most one", len(images))
	}
	var b struct{ Card int }
	_ = json.Unmarshal(images[0].Blob, &b)
	return b.Card
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
			var recordCalls []recordCall
			var sessionCalls int
			p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
			withImages(p, nil)

			content, err := p.Format(tc.group)
			if err != nil {
				t.Fatalf("Format error: %v", err)
			}
			if _, _, err := p.Post(content); err != nil {
				t.Fatalf("Post error: %v", err)
			}

			if got := cardOf(t, recordCalls[0].images); got != 0 {
				t.Errorf("root carries card %d, want the overview (0)", got)
			}
			if img := recordCalls[0].images[0]; img.Alt != "card 0" || img.Width != 1080 || img.Height != 1350 {
				t.Errorf("root image = %+v, want its alt text and aspect ratio", img)
			}
			seen := map[int]bool{}
			for i, c := range recordCalls[1:] {
				k := cardOf(t, c.images)
				if k < 0 {
					continue
				}
				v := k - 1
				label := voteformat.SubVoteLabel(tc.group[v], v, len(tc.group))
				if !strings.Contains(c.text, label) {
					t.Errorf("reply %d carries vote %d's card but its text is about something else:\n%s", i+1, v+1, c.text)
				}
				for w, other := range tc.group {
					if w != v && strings.Contains(c.text, voteformat.SubVoteLabel(other, w, len(tc.group))) {
						t.Errorf("reply %d carries vote %d's card but also talks about vote %d:\n%s", i+1, v+1, w+1, c.text)
					}
				}
				seen[k] = true
			}
			if len(seen) != len(tc.group) {
				t.Errorf("posted %d result cards, want one per vote (%d)", len(seen), len(tc.group))
			}
		})
	}
}

func TestPost_LoneVoteCardOnRootOnly(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	withImages(p, nil)

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got := cardOf(t, recordCalls[0].images); got != 0 {
		t.Errorf("root carries card %d, want the combined card", got)
	}
	for i, c := range recordCalls[1:] {
		if len(c.images) != 0 {
			t.Errorf("reply %d carries an image; a lone vote's card is on the root", i+1)
		}
	}
}

func TestPost_FailedUploadKeepsTheReplyText(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	withImages(p, map[int]bool{1: true})

	group := testfixtures.MultiVoteGroup()
	content, err := p.Format(group)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	label := voteformat.SubVoteLabel(group[0], 0, len(group))
	for i, c := range recordCalls {
		if cardOf(t, c.images) == 1 {
			t.Errorf("post %d carries card 1, which failed to upload", i)
		}
		if strings.Contains(c.text, label) {
			return
		}
	}
	t.Errorf("vote 1's text was lost with its card")
}

func TestFormat_RenderFailureKeepsText(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	p.generateCardsFunc = func([]votes.Vote) ([]imagegen.Card, error) { return nil, errors.New("font missing") }

	content, err := p.Format(testfixtures.MultiVoteGroup())
	if err != nil {
		t.Fatalf("Format should fall back to text, got error: %v", err)
	}
	for i, post := range content.(*BlueskyContent).thread {
		if len(post.Images) != 0 {
			t.Errorf("post %d has images after a render failure", i)
		}
		if post.Text == "" {
			t.Errorf("post %d lost its text", i)
		}
	}
}
