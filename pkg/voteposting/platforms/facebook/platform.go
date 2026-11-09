package facebook

import (
	"fmt"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/fbapi"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// UploadPhotoFunc is the signature for uploading an unpublished photo. Defaults
// to fbapi.Client.UploadPhoto.
type UploadPhotoFunc func(jpeg []byte, filename, caption string) (string, error)

// PublishPhotoPostFunc is the signature for publishing a post with attached
// photos. Defaults to fbapi.Client.PublishPhotoPost.
type PublishPhotoPostFunc func(message string, photoIDs []string) (string, error)

// FacebookPlatform implements the platforms.Platform interface for a Facebook Page
type FacebookPlatform struct {
	postsThisRun   int
	maxPostsPerRun int
	stubMode       bool // true when no credentials are configured

	// Injectable functions for testing
	uploadPhotoFunc      UploadPhotoFunc
	publishPhotoPostFunc PublishPhotoPostFunc
}

// NewFacebookPlatform creates a Facebook platform in stub mode: Post logs what
// it would publish and calls no API. Used by the preview and dry-run tools.
func NewFacebookPlatform(maxPostsPerRun int) *FacebookPlatform {
	return &FacebookPlatform{
		maxPostsPerRun: maxPostsPerRun,
		stubMode:       true,
	}
}

// NewFacebookPlatformWithCredentials creates a Facebook platform that publishes
// to the given Page with a Page access token.
func NewFacebookPlatformWithCredentials(pageID, accessToken string, maxPostsPerRun int) *FacebookPlatform {
	client := fbapi.NewClient(pageID, accessToken)
	return &FacebookPlatform{
		maxPostsPerRun:       maxPostsPerRun,
		uploadPhotoFunc:      client.UploadPhoto,
		publishPhotoPostFunc: client.PublishPhotoPost,
	}
}

// Format formats a group of votes into a single Page post
func (p *FacebookPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	return FormatPost(group)
}

// Post publishes content to the Page: each card is uploaded unpublished, then
// one post attaches them all. Unlike Instagram nothing is hosted first — Facebook
// takes the JPEG bytes directly.
//
// A failure after some uploads leaves unpublished photos behind. Meta deletes
// those itself after about a day, and the vote is not marked as posted, so the
// next run starts the post again from scratch.
//
// Returns a ref to the published post (none in stub mode), and
// shouldContinue=false when the post limit is reached.
func (p *FacebookPlatform) Post(content platforms.Content) (votelog.PostRef, bool, error) {
	fbContent, ok := content.(*FacebookContent)
	if !ok {
		return votelog.PostRef{}, false, fmt.Errorf("unexpected content type for Facebook")
	}

	var post votelog.PostRef
	if p.stubMode {
		fmt.Printf("📘 [Facebook stub] Would post %d image(s) with a message of %d chars\n",
			len(fbContent.Images), len([]rune(fbContent.Message)))
	} else {
		postID, err := p.publish(fbContent)
		if err != nil {
			return votelog.PostRef{}, false, err
		}
		post = votelog.PostRef{ID: postID, URL: fbapi.PostURL(postID)}
	}

	p.postsThisRun++
	return post, p.postsThisRun < p.maxPostsPerRun, nil
}

// publish uploads the cards and publishes the post, returning its ID.
func (p *FacebookPlatform) publish(c *FacebookContent) (string, error) {
	fmt.Printf("📤 Uploading %d image(s) to Facebook...\n", len(c.Images))
	ts := time.Now().UnixMilli()
	photoIDs := make([]string, len(c.Images))
	for i, img := range c.Images {
		id, err := p.uploadPhotoFunc(img, fmt.Sprintf("carousel_%d_%d.jpg", ts, i), c.Captions[i])
		if err != nil {
			return "", fmt.Errorf("uploading image %d: %w", i, err)
		}
		photoIDs[i] = id
	}

	postID, err := p.publishPhotoPostFunc(c.Message, photoIDs)
	if err != nil {
		return "", fmt.Errorf("publishing Facebook post: %w", err)
	}
	fmt.Printf("✅ Facebook post created (id: %s)\n", postID)
	fmt.Printf("   🔗 %s\n", fbapi.PostURL(postID))
	return postID, nil
}

// MaxPostsPerRun returns the configured per-run posting limit.
func (p *FacebookPlatform) MaxPostsPerRun() int {
	return p.maxPostsPerRun
}

// Name returns the platform name
func (p *FacebookPlatform) Name() string {
	return "Facebook"
}
