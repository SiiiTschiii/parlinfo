package bluesky

import (
	"fmt"
	"log"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/bskyapi"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// BlueskyContent implements platforms.Content for Bluesky
type BlueskyContent struct {
	thread []*BlueskyPost // [0] = root, [1:] = replies
}

// NewContent wraps an already formatted thread, so content built outside this
// package — a Geschäft announcement — posts through the same Post as a vote.
func NewContent(thread []*BlueskyPost) *BlueskyContent {
	return &BlueskyContent{thread: thread}
}

// String returns the text representation for logging/preview
func (c *BlueskyContent) String() string {
	var sb strings.Builder
	for i, post := range c.thread {
		if i == 0 {
			sb.WriteString(post.Text)
		} else {
			sb.WriteString(fmt.Sprintf("\n  ↳ Reply %d:\n%s", i, post.Text))
		}
		// Show detected mentions in preview
		for _, m := range post.Mentions {
			name := post.Text[m.ByteStart:m.ByteEnd]
			sb.WriteString(fmt.Sprintf("\n  📎 @%s → %s", m.Handle, name))
		}
	}
	return sb.String()
}

// CreateRecordFunc is the signature for creating a Bluesky post. Defaults to bskyapi.CreateRecord.
type CreateRecordFunc func(session *bskyapi.Session, text string, facets []bskyapi.Facet, replyTo *bskyapi.ReplyRef) (*bskyapi.PostRef, error)

// CreateSessionFunc is the signature for creating a Bluesky session. Defaults to bskyapi.CreateSession.
type CreateSessionFunc func(handle, password string) (*bskyapi.Session, error)

// ResolveHandleFunc is the signature for resolving a Bluesky handle to DID. Defaults to bskyapi.ResolveHandle.
type ResolveHandleFunc func(handle string) (string, error)

// BlueskyPlatform implements the platforms.Platform interface for Bluesky
type BlueskyPlatform struct {
	handle            string
	password          string
	session           *bskyapi.Session
	contactMapper     *contacts.Mapper
	postsThisRun      int
	maxPostsPerRun    int
	didCache          map[string]string // cache handle -> DID resolutions
	createRecordFunc  CreateRecordFunc  // injectable for testing
	createSessionFunc CreateSessionFunc // injectable for testing
	resolveHandleFunc ResolveHandleFunc // injectable for testing
}

// NewBlueskyPlatform creates a new Bluesky platform poster
func NewBlueskyPlatform(handle, password string, maxPostsPerRun int, contactMapper *contacts.Mapper) *BlueskyPlatform {
	return &BlueskyPlatform{
		handle:            handle,
		password:          password,
		maxPostsPerRun:    maxPostsPerRun,
		contactMapper:     contactMapper,
		postsThisRun:      0,
		didCache:          make(map[string]string),
		createRecordFunc:  bskyapi.CreateRecord,
		createSessionFunc: bskyapi.CreateSession,
		resolveHandleFunc: bskyapi.ResolveHandle,
	}
}

// ensureSession creates or reuses a Bluesky session
func (p *BlueskyPlatform) ensureSession() error {
	if p.session != nil {
		return nil
	}

	session, err := p.createSessionFunc(p.handle, p.password)
	if err != nil {
		return fmt.Errorf("failed to authenticate with Bluesky: %w", err)
	}

	p.session = session
	fmt.Printf("🔑 Authenticated as %s (DID: %s)\n", session.Handle, session.DID)
	return nil
}

// Format formats a group of votes into a Bluesky thread
func (p *BlueskyPlatform) Format(group []votes.Vote) (platforms.Content, error) {
	thread := FormatVoteThread(group, p.contactMapper)
	return &BlueskyContent{thread: thread}, nil
}

// Post posts a thread to Bluesky (root post + reply chain) and returns a ref
// to the root post: its at:// URI as the ID, and its bsky.app URL.
// Returns shouldContinue=false when the post limit is reached
func (p *BlueskyPlatform) Post(content platforms.Content) (votelog.PostRef, bool, error) {
	bskyContent, ok := content.(*BlueskyContent)
	if !ok {
		return votelog.PostRef{}, false, fmt.Errorf("unexpected content type for Bluesky")
	}

	if len(bskyContent.thread) == 0 {
		return votelog.PostRef{}, false, fmt.Errorf("empty thread")
	}

	// Authenticate lazily on first post
	if err := p.ensureSession(); err != nil {
		return votelog.PostRef{}, false, err
	}

	// Post the root
	root := bskyContent.thread[0]
	p.resolveMentionFacets(root)
	rootRef, err := p.createRecordFunc(p.session, root.Text, root.Facets, nil)
	if err != nil {
		return votelog.PostRef{}, false, fmt.Errorf("failed to post root: %w", err)
	}
	fmt.Printf("✅ Root post created (uri: %s)\n", rootRef.URI)
	rootURL := postURL(rootRef.URI)
	fmt.Printf("   🔗 %s\n", rootURL)

	// Post replies as a chain
	parentRef := rootRef
	for i, reply := range bskyContent.thread[1:] {
		p.resolveMentionFacets(reply)
		replyRef := &bskyapi.ReplyRef{
			Root:   *rootRef,
			Parent: *parentRef,
		}

		ref, err := p.createRecordFunc(p.session, reply.Text, reply.Facets, replyRef)
		if err != nil {
			return votelog.PostRef{}, false, fmt.Errorf("failed to post reply %d: %w", i+1, err)
		}
		fmt.Printf("  ↳ Reply %d created (uri: %s)\n", i+1, ref.URI)
		parentRef = ref
	}

	p.postsThisRun++
	shouldContinue := p.postsThisRun < p.maxPostsPerRun

	return votelog.PostRef{ID: rootRef.URI, URL: rootURL}, shouldContinue, nil
}

// postURL turns a post's at://{did}/app.bsky.feed.post/{rkey} URI into its
// bsky.app URL. It addresses the profile by DID rather than handle, because a
// handle can change and a DID cannot. A URI of any other shape yields "".
func postURL(uri string) string {
	rest, ok := strings.CutPrefix(uri, "at://")
	if !ok {
		return ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[1] != "app.bsky.feed.post" || parts[0] == "" || parts[2] == "" {
		return ""
	}
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", parts[0], parts[2])
}

// MaxPostsPerRun returns the configured per-run posting limit.
func (p *BlueskyPlatform) MaxPostsPerRun() int {
	return p.maxPostsPerRun
}

// Name returns the platform name
func (p *BlueskyPlatform) Name() string {
	return "Bluesky"
}

// resolveMentionFacets resolves Bluesky mentions to DIDs and adds mention facets.
// Uses a cache to avoid redundant API calls.
func (p *BlueskyPlatform) resolveMentionFacets(post *BlueskyPost) {
	for _, mention := range post.Mentions {
		did, err := p.resolveHandleCached(mention.Handle)
		if err != nil {
			log.Printf("Warning: could not resolve Bluesky handle %q: %v", mention.Handle, err)
			continue
		}
		post.Facets = append(post.Facets, bskyapi.MentionFacet(mention.ByteStart, mention.ByteEnd, did))
	}
}

// resolveHandleCached resolves a handle to DID, caching results
func (p *BlueskyPlatform) resolveHandleCached(handle string) (string, error) {
	if did, ok := p.didCache[handle]; ok {
		return did, nil
	}

	did, err := p.resolveHandleFunc(handle)
	if err != nil {
		return "", err
	}

	p.didCache[handle] = did
	return did, nil
}
