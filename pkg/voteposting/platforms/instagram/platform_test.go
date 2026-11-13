package instagram

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/igapi"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
)

// Verify InstagramPlatform satisfies platforms.Platform at compile time.
var _ platforms.Platform = (*InstagramPlatform)(nil)

func TestNewInstagramPlatform(t *testing.T) {
	p := NewInstagramPlatform(5)
	if p.MaxPostsPerRun() != 5 {
		t.Errorf("expected MaxPostsPerRun=5, got %d", p.MaxPostsPerRun())
	}
	if p.Name() != "Instagram" {
		t.Errorf("expected Name()=Instagram, got %q", p.Name())
	}
}

func TestPlatform_FormatAndPost(t *testing.T) {
	p := NewInstagramPlatform(5)

	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	igContent, ok := content.(*InstagramContent)
	if !ok {
		t.Fatal("expected *InstagramContent from Format")
	}

	if len(igContent.Images) == 0 {
		t.Error("expected at least 1 image")
	}
	if igContent.Caption == "" {
		t.Error("expected non-empty caption")
	}

	_, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if !shouldContinue {
		t.Error("expected shouldContinue=true after first post with limit=5")
	}
}

func TestPlatform_PostLimitReached(t *testing.T) {
	p := NewInstagramPlatform(1)

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

func TestPlatform_PostWrongContentType(t *testing.T) {
	p := NewInstagramPlatform(5)

	// Create a mock content that doesn't implement InstagramContent
	_, _, err := p.Post(mockContent{})
	if err == nil {
		t.Error("expected error for wrong content type")
	}
}

type mockContent struct{}

func (m mockContent) String() string { return "mock" }

func TestPlatform_StubMode(t *testing.T) {
	p := NewInstagramPlatform(5)
	if !p.stubMode {
		t.Error("expected stub mode when created without credentials")
	}
}

func TestPlatform_RealPostFlow(t *testing.T) {
	// Create a platform with injected mock functions to test the full posting flow
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {} // no-op sleep for testing
	p.waitForImagesFunc = func(_ []string) error { return nil }

	var uploadedNames []string
	var cleanedUpNames []string
	singleImageCreated := false

	p.uploadImagesFunc = func(images [][]byte, names []string) ([]string, error) {
		uploadedNames = names
		urls := make([]string, len(images))
		for i := range images {
			urls[i] = "https://example.github.io/repo/ig-images/" + names[i]
		}
		return urls, nil
	}

	p.createSingleImageContainerFunc = func(_ string, _ string) (string, error) {
		singleImageCreated = true
		return "single_container", nil
	}

	p.createMediaContainerFunc = func(_ string) (string, error) {
		t.Error("createMediaContainerFunc should not be called for single image")
		return "", nil
	}

	p.createCarouselContainerFunc = func(_ []string, _ string) (string, error) {
		t.Error("createCarouselContainerFunc should not be called for single image")
		return "", nil
	}

	p.publishContainerFunc = func(_ string) (string, error) {
		return "media_123", nil
	}

	p.pollContainerStatusFunc = func(_ string) (string, error) {
		return igapi.StatusPublished, nil
	}

	p.getPermalinkFunc = func(mediaID string) (string, error) {
		if mediaID != "media_123" {
			t.Errorf("permalink looked up for %q, want media_123", mediaID)
		}
		return "https://www.instagram.com/p/DAbC123/", nil
	}

	p.cleanupImagesFunc = func(names []string) error {
		cleanedUpNames = names
		return nil
	}

	// Format and post
	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	post, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if !shouldContinue {
		t.Error("expected shouldContinue=true with limit=5")
	}
	want := votelog.PostRef{ID: "media_123", URL: "https://www.instagram.com/p/DAbC123/"}
	if post != want {
		t.Errorf("expected ref %+v, got %+v", want, post)
	}

	// Verify the full flow was executed
	if len(uploadedNames) == 0 {
		t.Error("expected images to be uploaded")
	}
	if !singleImageCreated {
		t.Error("expected single image container to be created")
	}
	if len(cleanedUpNames) == 0 {
		t.Error("expected images to be cleaned up")
	}
	// Upload and cleanup should use same names
	if len(uploadedNames) != len(cleanedUpNames) {
		t.Errorf("upload names (%d) != cleanup names (%d)", len(uploadedNames), len(cleanedUpNames))
	}
}

// Channels share one image directory and post side by side, so each one's
// files carry its key.
func TestPlatform_RealPostFlow_ImageNamesCarryChannel(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {}
	p.waitForImagesFunc = func(_ []string) error { return nil }
	p.SetImageNamePrefix("chparl-fr")

	var uploaded, cleaned []string
	p.uploadImagesFunc = func(images [][]byte, names []string) ([]string, error) {
		uploaded = names
		return make([]string, len(images)), nil
	}
	p.createSingleImageContainerFunc = func(_, _ string) (string, error) { return "c", nil }
	p.pollContainerStatusFunc = func(_ string) (string, error) { return igapi.StatusFinished, nil }
	p.publishContainerFunc = func(_ string) (string, error) { return "m", nil }
	p.getPermalinkFunc = func(_ string) (string, error) { return "https://example.com/p", nil }
	p.cleanupImagesFunc = func(names []string) error { cleaned = names; return nil }

	content := &InstagramContent{Images: [][]byte{{0xFF}}, Caption: "c"}
	if _, _, err := p.Post(content); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(uploaded) != 1 || !strings.HasPrefix(uploaded[0], "chparl-fr_carousel_") {
		t.Errorf("uploaded %v, want one name starting chparl-fr_carousel_", uploaded)
	}
	if len(cleaned) != 1 || cleaned[0] != uploaded[0] {
		t.Errorf("cleaned up %v, want %v", cleaned, uploaded)
	}
}

// TestPlatform_RealPostFlow_PermalinkError covers a lookup failing after the
// media is live: the post must still count as published, or the next run would
// publish it a second time, and the media ID is kept so the post can be found.
func TestPlatform_RealPostFlow_PermalinkError(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {}
	p.waitForImagesFunc = func(_ []string) error { return nil }
	p.uploadImagesFunc = func(images [][]byte, names []string) ([]string, error) {
		return make([]string, len(images)), nil
	}
	p.createSingleImageContainerFunc = func(_ string, _ string) (string, error) { return "single_container", nil }
	p.publishContainerFunc = func(_ string) (string, error) { return "media_123", nil }
	p.pollContainerStatusFunc = func(_ string) (string, error) { return igapi.StatusPublished, nil }
	p.getPermalinkFunc = func(_ string) (string, error) { return "", errors.New("graph api down") }
	p.cleanupImagesFunc = func(_ []string) error { return nil }

	content, err := p.Format(testfixtures.SingleVoteAngenommen())
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	post, _, err := p.Post(content)
	if err != nil {
		t.Fatalf("a failed permalink lookup must not fail the post: %v", err)
	}
	if want := (votelog.PostRef{ID: "media_123"}); post != want {
		t.Errorf("expected ref %+v, got %+v", want, post)
	}
}

func TestPlatform_RealPostFlow_UploadError(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {}
	p.waitForImagesFunc = func(_ []string) error { return nil }

	p.uploadImagesFunc = func(_ [][]byte, _ []string) ([]string, error) {
		return nil, errTest
	}

	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	_, _, err = p.Post(content)
	if err == nil {
		t.Fatal("expected error when upload fails")
	}
}

func TestPlatform_RealPostFlow_ContainerError(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {}
	p.waitForImagesFunc = func(_ []string) error { return nil }

	p.uploadImagesFunc = func(images [][]byte, names []string) ([]string, error) {
		urls := make([]string, len(images))
		for i := range images {
			urls[i] = "https://example.com/" + names[i]
		}
		return urls, nil
	}

	p.createSingleImageContainerFunc = func(_ string, _ string) (string, error) {
		return "", errTest
	}

	p.createMediaContainerFunc = func(_ string) (string, error) {
		return "", errTest
	}

	votes := testfixtures.SingleVoteAngenommen()
	content, err := p.Format(votes)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}

	_, _, err = p.Post(content)
	if err == nil {
		t.Fatal("expected error when container creation fails")
	}
}

func TestPlatform_PollUntilPublished_ImmediateSuccess(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.sleepFunc = func(_ time.Duration) {}
	p.pollContainerStatusFunc = func(_ string) (string, error) {
		return igapi.StatusPublished, nil
	}

	err := p.pollUntilPublished("container_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlatform_PollUntilPublished_ErrorStatus(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.sleepFunc = func(_ time.Duration) {}
	p.pollContainerStatusFunc = func(_ string) (string, error) {
		return igapi.StatusError, nil
	}

	err := p.pollUntilPublished("container_1")
	if err == nil {
		t.Fatal("expected error for ERROR status")
	}
}

func TestPlatform_PollUntilPublished_ExpiredStatus(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.sleepFunc = func(_ time.Duration) {}
	p.pollContainerStatusFunc = func(_ string) (string, error) {
		return igapi.StatusExpired, nil
	}

	err := p.pollUntilPublished("container_1")
	if err == nil {
		t.Fatal("expected error for EXPIRED status")
	}
}

func TestPlatform_PollUntilPublished_FinishedStatus(t *testing.T) {
	p := NewInstagramPlatform(5)
	p.sleepFunc = func(_ time.Duration) {}
	p.pollContainerStatusFunc = func(_ string) (string, error) {
		return igapi.StatusFinished, nil
	}

	err := p.pollUntilPublished("container_1")
	if err != nil {
		t.Fatalf("unexpected error for FINISHED status: %v", err)
	}
}

func TestPlatform_NewWithCredentials(t *testing.T) {
	p := NewInstagramPlatformWithCredentials("ig_user", "token", "gh_token", "owner", "repo", 10)
	if p.stubMode {
		t.Error("expected real mode when created with credentials")
	}
	if p.MaxPostsPerRun() != 10 {
		t.Errorf("expected MaxPostsPerRun=10, got %d", p.MaxPostsPerRun())
	}
	if p.igClient == nil {
		t.Error("expected igClient to be set")
	}
	if p.imageHoster == nil {
		t.Error("expected imageHoster to be set")
	}
}

var errTest = fmt.Errorf("test error")

// newDuplicateTestPlatform wires a real-mode platform whose every step
// succeeds, with recent media and the publish outcome left to the test.
func newDuplicateTestPlatform(t *testing.T, recent func() ([]igapi.Media, error), publish func() (string, error)) (*InstagramPlatform, *int) {
	t.Helper()
	p := NewInstagramPlatform(5)
	p.stubMode = false
	p.sleepFunc = func(_ time.Duration) {}
	p.waitForImagesFunc = func(_ []string) error { return nil }
	uploads := 0
	p.uploadImagesFunc = func(images [][]byte, _ []string) ([]string, error) {
		uploads++
		return make([]string, len(images)), nil
	}
	p.createSingleImageContainerFunc = func(_, _ string) (string, error) { return "c", nil }
	p.createMediaContainerFunc = func(_ string) (string, error) { return "child", nil }
	p.createCarouselContainerFunc = func(_ []string, _ string) (string, error) { return "c", nil }
	p.pollContainerStatusFunc = func(_ string) (string, error) { return igapi.StatusFinished, nil }
	p.publishContainerFunc = func(_ string) (string, error) { return publish() }
	p.getPermalinkFunc = func(_ string) (string, error) { return "https://www.instagram.com/p/New/", nil }
	p.recentMediaFunc = func(limit int) ([]igapi.Media, error) {
		if limit != recentMediaChecked {
			t.Errorf("listed %d media, want %d", limit, recentMediaChecked)
		}
		return recent()
	}
	p.cleanupImagesFunc = func(_ []string) error { return nil }
	return p, &uploads
}

var errBlocked = errors.New(`publish container returned status 403: {"error":{"code":4,"error_subcode":2207051}}`)

// A vote whose caption is already on the account was published by an earlier
// run that logged it as failed. It is logged now, and not posted again.
func TestPlatform_SkipsCaptionAlreadyOnAccount(t *testing.T) {
	content := &InstagramContent{Images: [][]byte{{0xFF}}, Caption: "🗳️ Vote\n\nMotion X"}
	p, uploads := newDuplicateTestPlatform(t,
		func() ([]igapi.Media, error) {
			return []igapi.Media{
				{ID: "other", Caption: "🗳️ Vote\n\nMotion Y"},
				{ID: "m1", Caption: "🗳️ Vote\r\n\r\nMotion X\n", Permalink: "https://www.instagram.com/p/Old/"},
			}, nil
		},
		func() (string, error) { t.Error("published a caption already on the account"); return "", nil })

	post, shouldContinue, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if want := (votelog.PostRef{ID: "m1", URL: "https://www.instagram.com/p/Old/"}); post != want {
		t.Errorf("ref %+v, want %+v", post, want)
	}
	if *uploads != 0 {
		t.Errorf("uploaded images %d time(s), want none", *uploads)
	}
	if !shouldContinue {
		t.Error("a post found on the account must not use up the run's limit")
	}
}

// Instagram answered the publish with a 403 and published anyway: the post
// counts, so the vote is logged and the next run does not post it again.
func TestPlatform_PublishErrorButPostIsLive(t *testing.T) {
	content := &InstagramContent{Images: [][]byte{{0xFF}}, Caption: "caption"}
	published := false
	p, _ := newDuplicateTestPlatform(t,
		func() ([]igapi.Media, error) {
			if !published {
				return nil, nil
			}
			return []igapi.Media{{ID: "m2", Caption: "caption", Permalink: "https://www.instagram.com/p/Live/"}}, nil
		},
		func() (string, error) { published = true; return "", errBlocked })

	post, _, err := p.Post(content)
	if err != nil {
		t.Fatalf("a publish that went out must not fail: %v", err)
	}
	if want := (votelog.PostRef{ID: "m2", URL: "https://www.instagram.com/p/Live/"}); post != want {
		t.Errorf("ref %+v, want %+v", post, want)
	}
}

// The votes of a group that arrive after it was posted share its headline and
// links; the image count keeps them from passing for the earlier post.
func TestPlatform_LaterVotesOfPostedGroupStillPost(t *testing.T) {
	content := &InstagramContent{Images: [][]byte{{0xFF}, {0xFF}}, Caption: "🗳️ Vote du 01.10.2026\n\nhttps://example.com/a"}
	var earlier igapi.Media
	earlier.ID, earlier.Caption = "m1", content.Caption
	for range 3 {
		earlier.Children.Data = append(earlier.Children.Data, struct {
			ID string `json:"id"`
		}{"c"})
	}
	p, uploads := newDuplicateTestPlatform(t,
		func() ([]igapi.Media, error) { return []igapi.Media{earlier}, nil },
		func() (string, error) { return "m2", nil })

	post, _, err := p.Post(content)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if post.ID != "m2" || *uploads != 1 {
		t.Errorf("posted %+v after %d upload(s), want a new post", post, *uploads)
	}
}

// A publish error with nothing on the account is a real failure.
func TestPlatform_PublishErrorAndNoPost(t *testing.T) {
	content := &InstagramContent{Images: [][]byte{{0xFF}}, Caption: "caption"}
	p, _ := newDuplicateTestPlatform(t,
		func() ([]igapi.Media, error) { return nil, nil },
		func() (string, error) { return "", errBlocked })
	if _, _, err := p.Post(content); !errors.Is(err, errBlocked) {
		t.Fatalf("Post error = %v, want the publish error", err)
	}
}

// When the account cannot be checked, posting could duplicate, so it fails.
func TestPlatform_RecentMediaErrorFailsBeforePosting(t *testing.T) {
	content := &InstagramContent{Images: [][]byte{{0xFF}}, Caption: "caption"}
	p, uploads := newDuplicateTestPlatform(t,
		func() ([]igapi.Media, error) { return nil, errors.New("graph api down") },
		func() (string, error) { t.Error("published without checking the account"); return "", nil })

	if _, _, err := p.Post(content); err == nil {
		t.Fatal("expected an error")
	}
	if *uploads != 0 {
		t.Errorf("uploaded images %d time(s), want none", *uploads)
	}
}

// The key ignores what changes between runs and keeps what tells groups apart.
func TestCaptionKey(t *testing.T) {
	const link = "🔗 https://www.parlament.ch/fr/geschaeft?AffairId=20253314"
	posted := "🗳️ Conseil national | Vote du 01.10.2026 (12:53)\n\nMotion X @someone\n63 Oui\n\n➡️ Next: Conseil des Etats\n\n" + link
	for name, tc := range map[string]struct {
		caption string
		same    bool
	}{
		"same caption":               {posted, true},
		"CRLF and trailing space":    {strings.ReplaceAll(posted, "\n", "\r\n") + "  ", true},
		"tags and next step changed": {"🗳️ Conseil national | Vote du 01.10.2026 (12:53)\n\nMotion X\n63 Oui\n\n" + link, true},
		"body cut for length":        {"🗳️ Conseil national | Vote du 01.10.2026 (12:53)\n\nMot…\nℹ️ cut\n" + link, true},
		"other vote, same business":  {"🗳️ Conseil national | Vote du 01.10.2026 (12:54)\n\nMotion X\n\n" + link, false},
		"same time, other business":  {"🗳️ Conseil national | Vote du 01.10.2026 (12:53)\n\nMotion Y\n\n🔗 https://www.parlament.ch/fr/geschaeft?AffairId=20253408", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := captionKey(tc.caption) == captionKey(posted); got != tc.same {
				t.Errorf("same key = %v, want %v", got, tc.same)
			}
		})
	}
}
