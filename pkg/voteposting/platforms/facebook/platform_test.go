package facebook

import (
	"errors"
	"fmt"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
)

type fakeAPI struct {
	captions   []string // captions, in call order
	uploads    []string // filenames, in call order
	uploadErr  error    // returned by the failOn-th upload (1-based); 0 = never
	failOn     int
	publishErr error
	message    string
	attached   []string
}

func (f *fakeAPI) upload(_ []byte, filename, caption string) (string, error) {
	f.uploads = append(f.uploads, filename)
	f.captions = append(f.captions, caption)
	if f.failOn == len(f.uploads) {
		return "", f.uploadErr
	}
	return fmt.Sprintf("photo_%d", len(f.uploads)), nil
}

func (f *fakeAPI) publish(message string, photoIDs []string) (string, error) {
	f.message, f.attached = message, photoIDs
	if f.publishErr != nil {
		return "", f.publishErr
	}
	return "page_1_post_1", nil
}

func newTestPlatform(f *fakeAPI, maxPostsPerRun int) *FacebookPlatform {
	return &FacebookPlatform{
		maxPostsPerRun:       maxPostsPerRun,
		uploadPhotoFunc:      f.upload,
		publishPhotoPostFunc: f.publish,
	}
}

func TestPost_UploadsEveryCardThenPublishesOnePost(t *testing.T) {
	f := &fakeAPI{}
	p := newTestPlatform(f, 5)
	content, err := p.Format(testfixtures.MultiVoteGroup())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	fb := content.(*FacebookContent)
	if len(fb.Images) < 2 {
		t.Fatalf("fixture should give several cards, got %d", len(fb.Images))
	}

	post, _, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	want := votelog.PostRef{ID: "page_1_post_1", URL: "https://www.facebook.com/page_1_post_1"}
	if post != want {
		t.Errorf("returned ref %+v, want %+v", post, want)
	}

	if len(f.uploads) != len(fb.Images) {
		t.Errorf("uploaded %d photos, want %d", len(f.uploads), len(fb.Images))
	}
	if f.message != fb.Message {
		t.Errorf("published the wrong message")
	}
	// Each card travels with its own caption, in the same order as the cards.
	for i, c := range fb.Captions {
		if f.captions[i] != c {
			t.Errorf("upload %d carried caption %q, want %q", i, f.captions[i], c)
		}
	}
	// The post attaches the uploads in the order they were made, which is the
	// carousel's order.
	for i, id := range f.attached {
		if want := fmt.Sprintf("photo_%d", i+1); id != want {
			t.Errorf("attached[%d] = %q, want %q", i, id, want)
		}
	}
	if len(f.attached) != len(fb.Images) {
		t.Errorf("attached %d photos, want %d", len(f.attached), len(fb.Images))
	}
}

// A failed upload must not publish a post with some of its cards missing, and
// must not count against the budget or report success: the pipeline would mark
// the votes as posted.
func TestPost_UploadFailureStopsBeforePublishing(t *testing.T) {
	f := &fakeAPI{failOn: 2, uploadErr: errors.New("boom")}
	p := newTestPlatform(f, 5)
	content, _ := p.Format(testfixtures.MultiVoteGroup())

	if _, _, err := p.Post(content); err == nil {
		t.Fatal("expected error")
	}
	if f.message != "" {
		t.Error("a post was published after an upload failed")
	}
	if p.postsThisRun != 0 {
		t.Errorf("failed post counted against the budget: %d", p.postsThisRun)
	}
}

func TestPost_PublishFailureIsAnError(t *testing.T) {
	f := &fakeAPI{publishErr: errors.New("boom")}
	p := newTestPlatform(f, 5)
	content, _ := p.Format(testfixtures.SingleVoteAngenommen())

	if _, _, err := p.Post(content); err == nil {
		t.Fatal("expected error")
	}
	if p.postsThisRun != 0 {
		t.Errorf("failed post counted against the budget: %d", p.postsThisRun)
	}
}

func TestPost_StopsAtBudget(t *testing.T) {
	f := &fakeAPI{}
	p := newTestPlatform(f, 2)
	content, _ := p.Format(testfixtures.SingleVoteAngenommen())

	if _, cont, _ := p.Post(content); !cont {
		t.Error("first post should leave budget to continue")
	}
	if _, cont, _ := p.Post(content); cont {
		t.Error("second post should exhaust a budget of 2")
	}
}

func TestPost_StubModeCallsNoAPI(t *testing.T) {
	p := NewFacebookPlatform(1)
	content, _ := p.Format(testfixtures.SingleVoteAngenommen())

	post, cont, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if cont {
		t.Error("budget of 1 should be exhausted after one post")
	}
	if post != (votelog.PostRef{}) {
		t.Errorf("stub mode published nothing, yet returned ref %+v", post)
	}
}

func TestPost_RejectsForeignContent(t *testing.T) {
	p := NewFacebookPlatform(1)
	if _, _, err := p.Post(nil); err == nil {
		t.Fatal("expected error for non-Facebook content")
	}
}
