package x

import (
	"fmt"
	"log"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/votes"
	"github.com/siiitschiii/parlinfo/pkg/xapi"
)

// XContent represents formatted content for X/Twitter
type XContent struct {
	thread []*XPost // [0] = root, [1:] = replies
}

// String returns the text representation for logging/preview
func (c *XContent) String() string {
	var sb strings.Builder
	for i, post := range c.thread {
		if i == 0 {
			sb.WriteString(post.Text)
		} else {
			sb.WriteString(fmt.Sprintf("\n  ↳ Reply %d:\n%s", i, post.Text))
		}
		if len(post.Images) > 0 {
			sb.WriteString(fmt.Sprintf("\n  🖼️ %d image(s)", len(post.Images)))
		}
	}
	return sb.String()
}

// PostTweetFunc is the signature for posting a tweet. Defaults to xapi.PostTweet.
type PostTweetFunc func(apiKey, apiSecret, accessToken, accessSecret, message, inReplyToTweetID string, mediaIDs []string) (string, error)

// UploadImageFunc is the signature for uploading an image. Defaults to xapi.UploadImage.
type UploadImageFunc func(apiKey, apiSecret, accessToken, accessSecret string, jpeg []byte) (string, error)

// SetAltTextFunc is the signature for describing uploaded media. Defaults to xapi.SetAltText.
type SetAltTextFunc func(apiKey, apiSecret, accessToken, accessSecret, mediaID, alt string) error

// GenerateCardsFunc is the signature for rendering a group's cards. Defaults to imagegen.GenerateCards.
type GenerateCardsFunc func(group []votes.Vote) ([]imagegen.Card, error)

// XPlatform implements the Platform interface for X/Twitter
type XPlatform struct {
	apiKey         string
	apiSecret      string
	accessToken    string
	accessSecret   string
	contactMapper  *contacts.Mapper
	postsThisRun   int
	maxPostsPerRun int
	maxChars       int           // per-post character limit
	postTweetFunc  PostTweetFunc // injectable for testing

	uploadImageFunc   UploadImageFunc   // injectable for testing
	setAltTextFunc    SetAltTextFunc    // injectable for testing
	generateCardsFunc GenerateCardsFunc // injectable for testing
}

// NewXPlatform creates a new X platform poster
func NewXPlatform(
	apiKey, apiSecret, accessToken, accessSecret string,
	contactMapper *contacts.Mapper,
	maxPostsPerRun int,
) *XPlatform {
	return &XPlatform{
		apiKey:         apiKey,
		apiSecret:      apiSecret,
		accessToken:    accessToken,
		accessSecret:   accessSecret,
		contactMapper:  contactMapper,
		maxPostsPerRun: maxPostsPerRun,
		maxChars:       DefaultMaxChars,
		postsThisRun:   0,
		postTweetFunc:  xapi.PostTweet,

		uploadImageFunc:   xapi.UploadImage,
		setAltTextFunc:    xapi.SetAltText,
		generateCardsFunc: imagegen.GenerateCards,
	}
}

// SetMaxChars overrides the per-post character limit (default: DefaultMaxChars).
func (p *XPlatform) SetMaxChars(n int) {
	p.maxChars = n
}

// Format formats a group of votes into an X thread and attaches the group's
// carousel cards to it, the first four on the root (see platforms.SpreadCards).
//
// A card that fails to render costs the thread its images, never its text: the
// text carries every fact the cards do, and the tags and links besides.
func (p *XPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	thread := FormatVoteThread(group, p.contactMapper, p.maxChars)
	if p.generateCardsFunc == nil || len(thread) == 0 {
		return &XContent{thread: thread}, nil
	}
	cards, err := p.generateCardsFunc(group)
	if err != nil {
		log.Printf("Warning: posting to X without images, rendering cards failed: %v", err)
		return &XContent{thread: thread}, nil
	}
	for i, images := range platforms.SpreadCards(cards, len(thread), xapi.MaxMediaPerTweet) {
		if i == len(thread) {
			thread = append(thread, &XPost{})
		}
		thread[i].Images = images
	}
	return &XContent{thread: thread}, nil
}

// uploadImages uploads a post's cards, describes each, and returns their media
// IDs. A card that fails to upload or to take its description is left out with
// a warning rather than failing the thread, for the same reason Format falls
// back to text. An undescribed card is not posted: on an image-only reply it
// would be the whole post, with nothing a screen reader could read.
func (p *XPlatform) uploadImages(cards []imagegen.Card) []string {
	var ids []string
	for _, c := range cards {
		id, err := p.uploadImageFunc(p.apiKey, p.apiSecret, p.accessToken, p.accessSecret, c.JPEG)
		if err != nil {
			log.Printf("Warning: posting without one image, upload failed: %v", err)
			continue
		}
		if err := p.setAltTextFunc(p.apiKey, p.apiSecret, p.accessToken, p.accessSecret, id, c.Alt); err != nil {
			log.Printf("Warning: posting without image %s, setting its alt text failed: %v", id, err)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// Post posts a thread to X/Twitter (root post + reply chain) and returns a
// ref to the root tweet.
// Returns shouldContinue=false when the post limit is reached.
func (p *XPlatform) Post(content platforms.Content) (votelog.PostRef, bool, error) {
	xContent, ok := content.(*XContent)
	if !ok {
		return votelog.PostRef{}, false, fmt.Errorf("unexpected content type for X")
	}

	if len(xContent.thread) == 0 {
		return votelog.PostRef{}, false, fmt.Errorf("empty thread")
	}

	// Post the root
	root := xContent.thread[0]
	rootTweetID, err := p.postTweetFunc(
		p.apiKey, p.apiSecret, p.accessToken, p.accessSecret,
		root.Text, "", p.uploadImages(root.Images),
	)
	if err != nil {
		return votelog.PostRef{}, false, fmt.Errorf("failed to post root: %w", err)
	}
	fmt.Printf("   🔗 %s\n", tweetURL(rootTweetID))

	// Post replies as a chain
	parentTweetID := rootTweetID
	for i, reply := range xContent.thread[1:] {
		mediaIDs := p.uploadImages(reply.Images)
		if reply.Text == "" && len(mediaIDs) == 0 {
			// An image-only reply whose images all failed has nothing left to say.
			continue
		}
		tweetID, err := p.postTweetFunc(
			p.apiKey, p.apiSecret, p.accessToken, p.accessSecret,
			reply.Text, parentTweetID, mediaIDs,
		)
		if err != nil {
			return votelog.PostRef{}, false, fmt.Errorf("failed to post reply %d: %w", i+1, err)
		}
		fmt.Printf("   🔗 %s\n", tweetURL(tweetID))
		parentTweetID = tweetID
	}

	p.postsThisRun++
	shouldContinue := p.postsThisRun < p.maxPostsPerRun

	return votelog.PostRef{ID: rootTweetID, URL: tweetURL(rootTweetID)}, shouldContinue, nil
}

// tweetURL returns the public URL of a tweet. The /i/ form resolves without
// the account's handle, so it survives a rename.
func tweetURL(tweetID string) string {
	return "https://x.com/i/status/" + tweetID
}

// MaxPostsPerRun returns the configured per-run posting limit.
func (p *XPlatform) MaxPostsPerRun() int {
	return p.maxPostsPerRun
}

// Name returns the platform name
func (p *XPlatform) Name() string {
	return "X"
}
