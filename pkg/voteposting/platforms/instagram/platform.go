package instagram

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/igapi"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

const (
	// pollInterval is the time between container status checks.
	pollInterval = 10 * time.Second
	// pollTimeout is the maximum time to wait for a container to be published.
	pollTimeout = 5 * time.Minute
	// pagesDeploymentPollInterval is the time between GitHub Pages availability checks.
	pagesDeploymentPollInterval = 10 * time.Second
	// pagesDeploymentTimeout is the maximum time to wait for GitHub Pages deployment.
	pagesDeploymentTimeout = 6 * time.Minute
)

// CreateMediaContainerFunc is the signature for creating an IG media container.
type CreateMediaContainerFunc func(imageURL string) (string, error)

// CreateCarouselContainerFunc is the signature for creating an IG carousel container.
type CreateCarouselContainerFunc func(childIDs []string, caption string) (string, error)

// PublishContainerFunc is the signature for publishing an IG container.
type PublishContainerFunc func(containerID string) (string, error)

// PollContainerStatusFunc is the signature for polling container status.
type PollContainerStatusFunc func(containerID string) (string, error)

// GetPermalinkFunc is the signature for looking up a published media's URL.
type GetPermalinkFunc func(mediaID string) (string, error)

// UploadImagesFunc is the signature for uploading images to hosting.
type UploadImagesFunc func(images [][]byte, names []string) ([]string, error)

// CleanupImagesFunc is the signature for cleaning up hosted images.
type CleanupImagesFunc func(names []string) error

// CreateSingleImageContainerFunc is the signature for creating a standalone image container.
type CreateSingleImageContainerFunc func(imageURL, caption string) (string, error)

// InstagramPlatform implements the platforms.Platform interface for Instagram
type InstagramPlatform struct {
	postsThisRun   int
	maxPostsPerRun int
	igClient       *igapi.Client
	imageHoster    *igapi.ImageHoster
	contactMapper  *contacts.Mapper
	stubMode       bool // true when no credentials are configured

	// Injectable functions for testing
	createMediaContainerFunc       CreateMediaContainerFunc
	createCarouselContainerFunc    CreateCarouselContainerFunc
	createSingleImageContainerFunc CreateSingleImageContainerFunc
	publishContainerFunc           PublishContainerFunc
	pollContainerStatusFunc        PollContainerStatusFunc
	getPermalinkFunc               GetPermalinkFunc
	uploadImagesFunc               UploadImagesFunc
	cleanupImagesFunc              CleanupImagesFunc
	waitForImagesFunc              func(urls []string) error
	sleepFunc                      func(time.Duration)
}

// NewInstagramPlatform creates a new Instagram platform poster in stub mode (no real posting).
func NewInstagramPlatform(maxPostsPerRun int) *InstagramPlatform {
	return &InstagramPlatform{
		maxPostsPerRun:    maxPostsPerRun,
		stubMode:          true,
		waitForImagesFunc: waitForImageURLs,
		sleepFunc:         time.Sleep,
	}
}

// NewInstagramPlatformWithCredentials creates a new Instagram platform poster with real API credentials.
func NewInstagramPlatformWithCredentials(igUserID, accessToken, githubToken, repoOwner, repoName string, maxPostsPerRun int) *InstagramPlatform {
	igClient := igapi.NewClient(igUserID, accessToken)
	hoster := igapi.NewImageHoster(repoOwner, repoName, githubToken)

	p := &InstagramPlatform{
		maxPostsPerRun: maxPostsPerRun,
		igClient:       igClient,
		imageHoster:    hoster,
		stubMode:       false,
		sleepFunc:      time.Sleep,
	}

	// Wire real API functions
	p.createMediaContainerFunc = igClient.CreateMediaContainer
	p.createCarouselContainerFunc = igClient.CreateCarouselContainer
	p.createSingleImageContainerFunc = igClient.CreateSingleImageContainer
	p.publishContainerFunc = igClient.PublishContainer
	p.pollContainerStatusFunc = igClient.PollContainerStatus
	p.getPermalinkFunc = igClient.GetPermalink
	p.uploadImagesFunc = hoster.UploadImages
	p.cleanupImagesFunc = hoster.CleanupImages
	p.waitForImagesFunc = waitForImageURLs

	return p
}

// Format formats a group of votes into Instagram-specific content (carousel images + caption).
func (p *InstagramPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	return FormatCarouselWithContacts(group, p.contactMapper)
}

// SetContactMapper configures mapped contacts for @mention tagging in generated captions.
func (p *InstagramPlatform) SetContactMapper(contactMapper *contacts.Mapper) {
	p.contactMapper = contactMapper
}

// Post publishes content to Instagram.
// In stub mode: logs preview, no real API calls.
// In real mode: uploads images to GitHub Pages, creates carousel via IG API, polls for PUBLISHED, cleans up.
// Returns a ref to the published media (none in stub mode), and
// shouldContinue=false when the post limit is reached.
func (p *InstagramPlatform) Post(content platforms.Content) (votelog.PostRef, bool, error) {
	igContent, ok := content.(*InstagramContent)
	if !ok {
		return votelog.PostRef{}, false, fmt.Errorf("unexpected content type for Instagram")
	}

	if p.stubMode {
		return p.postStub(igContent)
	}

	return p.postReal(igContent)
}

// postStub logs what would be posted without making any API calls.
func (p *InstagramPlatform) postStub(igContent *InstagramContent) (votelog.PostRef, bool, error) {
	fmt.Printf("📷 [Instagram stub] Would post %d image(s) with caption (%d chars):\n",
		len(igContent.Images), len([]rune(igContent.Caption)))

	preview := igContent.Caption
	if len([]rune(preview)) > 200 {
		preview = string([]rune(preview)[:200]) + "…"
	}
	fmt.Printf("   Caption preview: %s\n", preview)

	p.postsThisRun++
	return votelog.PostRef{}, p.postsThisRun < p.maxPostsPerRun, nil
}

// postReal publishes a carousel to Instagram:
// 1. Upload images to GitHub Pages
// 2. Create carousel item containers (one per image)
// 3. Create carousel container with all children + caption
// 4. Publish the carousel container
// 5. Poll until PUBLISHED or error
// 6. Look up the permalink
// 7. Clean up hosted images
func (p *InstagramPlatform) postReal(igContent *InstagramContent) (votelog.PostRef, bool, error) {
	imageCount := len(igContent.Images)

	// Generate unique filenames for this carousel
	names := make([]string, imageCount)
	ts := time.Now().UnixMilli()
	for i := range names {
		names[i] = fmt.Sprintf("carousel_%d_%d.jpg", ts, i)
	}

	// Step 1: Upload images to GitHub Pages
	fmt.Printf("📤 Uploading %d image(s) to GitHub Pages...\n", imageCount)
	imageURLs, err := p.uploadImagesFunc(igContent.Images, names)
	if err != nil {
		return votelog.PostRef{}, false, fmt.Errorf("uploading images: %w", err)
	}
	fmt.Printf("   ✅ Images hosted at %d URL(s)\n", len(imageURLs))

	// Step 2: Wait for GitHub Pages deployment by polling image URLs
	fmt.Print("⏳ Waiting for GitHub Pages deployment...\n")
	if err := p.waitForImagesFunc(imageURLs); err != nil {
		logHostedImagesWarning(names)
		return votelog.PostRef{}, false, fmt.Errorf("waiting for GitHub Pages: %w", err)
	}
	fmt.Print("   ✅ Images accessible\n")

	var publishID string
	if imageCount == 1 {
		// Single image post (carousels require 2+ images)
		fmt.Print("📦 Creating single image container...\n")
		containerID, err := p.createSingleImageContainerFunc(imageURLs[0], igContent.Caption)
		if err != nil {
			logHostedImagesWarning(names)
			return votelog.PostRef{}, false, fmt.Errorf("creating single image container: %w", err)
		}
		fmt.Printf("   📦 Container: %s\n", containerID)
		publishID = containerID
	} else {
		// Carousel post (2+ images)
		fmt.Printf("📦 Creating %d media container(s)...\n", imageCount)
		childIDs := make([]string, imageCount)
		for i, imageURL := range imageURLs {
			containerID, err := p.createMediaContainerFunc(imageURL)
			if err != nil {
				logHostedImagesWarning(names)
				return votelog.PostRef{}, false, fmt.Errorf("creating media container %d: %w", i, err)
			}
			childIDs[i] = containerID
			fmt.Printf("   📦 Container %d: %s\n", i, containerID)
		}

		fmt.Print("🎠 Creating carousel container...\n")
		carouselID, err := p.createCarouselContainerFunc(childIDs, igContent.Caption)
		if err != nil {
			logHostedImagesWarning(names)
			return votelog.PostRef{}, false, fmt.Errorf("creating carousel container: %w", err)
		}
		fmt.Printf("   🎠 Carousel container: %s\n", carouselID)
		publishID = carouselID
	}

	// Wait for container to be ready before publishing
	fmt.Print("⏳ Waiting for container to be ready...\n")
	if err := p.pollUntilReady(publishID); err != nil {
		logHostedImagesWarning(names)
		return votelog.PostRef{}, false, fmt.Errorf("waiting for container: %w", err)
	}

	// Publish
	fmt.Print("🚀 Publishing...\n")
	mediaID, err := p.publishContainerFunc(publishID)
	if err != nil {
		logHostedImagesWarning(names)
		return votelog.PostRef{}, false, fmt.Errorf("publishing: %w", err)
	}
	fmt.Printf("   ✅ Published! Media ID: %s\n", mediaID)

	// Poll for PUBLISHED status
	if err := p.pollUntilPublished(publishID); err != nil {
		log.Printf("⚠️  Polling error (media may still be published): %v", err)
		// Don't fail — the post may have succeeded
	}

	// The post is live from here on, so a failed lookup only costs the URL:
	// failing the run would leave the vote unposted and publish it again.
	post := votelog.PostRef{ID: mediaID}
	if permalink, err := p.getPermalinkFunc(mediaID); err != nil {
		log.Printf("⚠️  Permalink lookup failed (logging the media ID alone): %v", err)
	} else {
		post.URL = permalink
		fmt.Printf("   🔗 %s\n", permalink)
	}

	// Clean up hosted images
	fmt.Print("🧹 Cleaning up hosted images...\n")
	if err := p.cleanupImagesFunc(names); err != nil {
		// Log but don't fail — the post was successful
		log.Printf("⚠️  Cleanup error (images may remain hosted): %v", err)
	} else {
		fmt.Printf("   ✅ Cleaned up %d image(s)\n", imageCount)
	}

	p.postsThisRun++
	return post, p.postsThisRun < p.maxPostsPerRun, nil
}

// pollUntilPublished polls the container status until it reaches PUBLISHED, ERROR, or EXPIRED,
// or until the timeout is reached.
func (p *InstagramPlatform) pollUntilPublished(containerID string) error {
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		status, err := p.pollContainerStatusFunc(containerID)
		if err != nil {
			return fmt.Errorf("polling status: %w", err)
		}

		switch status {
		case igapi.StatusPublished:
			fmt.Printf("   ✅ Container status: %s\n", status)
			return nil
		case igapi.StatusFinished:
			fmt.Printf("   ✅ Container status: %s\n", status)
			return nil
		case igapi.StatusError:
			return fmt.Errorf("container status: ERROR")
		case igapi.StatusExpired:
			return fmt.Errorf("container status: EXPIRED")
		case igapi.StatusInProgress:
			fmt.Printf("   ⏳ Container status: %s, waiting...\n", status)
		default:
			fmt.Printf("   ⏳ Container status: %s, waiting...\n", status)
		}

		p.sleepFunc(pollInterval)
	}

	return fmt.Errorf("polling timed out after %v", pollTimeout)
}

// pollUntilReady polls the container status until it reaches FINISHED (ready to publish),
// or returns an error on ERROR/EXPIRED/timeout.
func (p *InstagramPlatform) pollUntilReady(containerID string) error {
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		status, err := p.pollContainerStatusFunc(containerID)
		if err != nil {
			return fmt.Errorf("polling status: %w", err)
		}

		switch status {
		case igapi.StatusFinished:
			fmt.Printf("   ✅ Container ready: %s\n", status)
			return nil
		case igapi.StatusPublished:
			fmt.Printf("   ✅ Container ready: %s\n", status)
			return nil
		case igapi.StatusError:
			return fmt.Errorf("container status: ERROR")
		case igapi.StatusExpired:
			return fmt.Errorf("container status: EXPIRED")
		default:
			fmt.Printf("   ⏳ Container status: %s, waiting...\n", status)
		}

		p.sleepFunc(pollInterval)
	}

	return fmt.Errorf("container not ready after %v", pollTimeout)
}

// logHostedImagesWarning logs a warning that images are left hosted for manual debugging.
func logHostedImagesWarning(names []string) {
	log.Printf("⚠️  Images left hosted for debugging: %v", names)
}

// MaxPostsPerRun returns the configured per-run posting limit.
func (p *InstagramPlatform) MaxPostsPerRun() int {
	return p.maxPostsPerRun
}

// Name returns the platform name.
func (p *InstagramPlatform) Name() string {
	return "Instagram"
}

// waitForImageURLs polls the given URLs until they all return HTTP 200, or the timeout is reached.
func waitForImageURLs(urls []string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(pagesDeploymentTimeout)

	for time.Now().Before(deadline) {
		allOK := true
		for _, u := range urls {
			resp, err := client.Head(u)
			if err != nil || resp.StatusCode != http.StatusOK {
				allOK = false
				break
			}
			_ = resp.Body.Close()
		}
		if allOK {
			return nil
		}
		time.Sleep(pagesDeploymentPollInterval)
	}

	return fmt.Errorf("images not accessible after %v", pagesDeploymentTimeout)
}
