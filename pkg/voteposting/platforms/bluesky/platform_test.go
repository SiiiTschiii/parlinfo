package bluesky

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/bskyapi"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
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

// withImages wires n placeholder cards and a fake blob upload into p. Each
// blob is {"card":<index>}; any card whose index is in failUploads fails.
func withImages(p *BlueskyPlatform, n int, failUploads map[byte]bool) {
	p.generateCardsFunc = func([]votes.Vote) ([]imagegen.Card, error) {
		cards := make([]imagegen.Card, n)
		for i := range cards {
			cards[i] = imagegen.Card{JPEG: []byte{byte(i)}, Alt: fmt.Sprintf("card %d", i+1), Width: 1080, Height: 1350}
		}
		return cards, nil
	}
	p.uploadBlobFunc = func(_ *bskyapi.Session, data []byte, mimeType string) (json.RawMessage, error) {
		if mimeType != "image/jpeg" {
			return nil, fmt.Errorf("unexpected mime type %q", mimeType)
		}
		if failUploads[data[0]] {
			return nil, errors.New("upload refused")
		}
		return json.RawMessage(fmt.Sprintf(`{"card":%d}`, data[0])), nil
	}
}

func blobs(images []bskyapi.Image) string {
	s := ""
	for _, img := range images {
		s += string(img.Blob)
	}
	return s
}

func TestPost_AttachesCardsToRoot(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	withImages(p, 3, nil)

	content, err := p.Format(testfixtures.MultiVoteGroup())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}

	root := recordCalls[0].images
	if got := blobs(root); got != `{"card":0}{"card":1}{"card":2}` {
		t.Errorf("root blobs = %s, want all three cards", got)
	}
	if root[1].Alt != "card 2" || root[1].Width != 1080 || root[1].Height != 1350 {
		t.Errorf("root image 2 = %+v, want its alt text and aspect ratio", root[1])
	}
	for i, c := range recordCalls[1:] {
		if len(c.images) != 0 {
			t.Errorf("reply %d carries %d images; three cards fit on the root", i+1, len(c.images))
		}
	}
}

func TestPost_OverflowCardsBecomeImageOnlyReplies(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	withImages(p, 30, nil)

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}

	total := 0
	for i, c := range recordCalls {
		if len(c.images) > bskyapi.MaxImagesPerPost {
			t.Errorf("post %d carries %d images", i, len(c.images))
		}
		total += len(c.images)
		if i > 0 && c.replyTo.Parent.URI != fmt.Sprintf("at://did:fake/app.bsky.feed.post/%d", i) {
			t.Errorf("post %d replies to %s, want the post before it", i, c.replyTo.Parent.URI)
		}
	}
	if total != 30 {
		t.Errorf("posted %d images, want all 30", total)
	}
	if last := recordCalls[len(recordCalls)-1]; last.text != "" {
		t.Errorf("overflow reply should be image-only, has text %q", last.text)
	}
}

func TestPost_FailedUploadDropsOnlyThatImage(t *testing.T) {
	var recordCalls []recordCall
	var sessionCalls int
	p := newTestBlueskyPlatform(&recordCalls, &sessionCalls, 10)
	withImages(p, 6, map[byte]bool{1: true, 4: true, 5: true})

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got := blobs(recordCalls[0].images); got != `{"card":0}{"card":2}{"card":3}` {
		t.Errorf("root blobs = %s", got)
	}
	for i, c := range recordCalls {
		if c.text == "" && len(c.images) == 0 {
			t.Errorf("post %d is empty: an image-only reply whose images all failed should be skipped", i)
		}
	}
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
