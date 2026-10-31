package x

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/votes"
	"github.com/siiitschiii/parlinfo/pkg/xapi"
)

// XContent represents formatted content for X/Twitter
type XContent struct {
	thread []*XPost // [0] = root, [1:] = replies
}

// NewContent wraps an already formatted thread, so content built outside this
// package — a Geschäft announcement — posts through the same Post as a vote.
func NewContent(thread []*XPost) *XContent {
	return &XContent{thread: thread}
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
	}
	return sb.String()
}

// PostTweetFunc is the signature for posting a tweet. Defaults to xapi.PostTweet.
type PostTweetFunc func(apiKey, apiSecret, accessToken, accessSecret, message, inReplyToTweetID string) (string, error)

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
	}
}

// SetMaxChars overrides the per-post character limit (default: DefaultMaxChars).
func (p *XPlatform) SetMaxChars(n int) {
	p.maxChars = n
}

// Format formats a group of votes into X-specific content
func (p *XPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	thread := FormatVoteThread(group, p.contactMapper, p.maxChars)
	return &XContent{thread: thread}, nil
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
		root.Text, "",
	)
	if err != nil {
		return votelog.PostRef{}, false, fmt.Errorf("failed to post root: %w", err)
	}
	fmt.Printf("   🔗 %s\n", tweetURL(rootTweetID))

	// Post replies as a chain
	parentTweetID := rootTweetID
	for i, reply := range xContent.thread[1:] {
		tweetID, err := p.postTweetFunc(
			p.apiKey, p.apiSecret, p.accessToken, p.accessSecret,
			reply.Text, parentTweetID,
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
