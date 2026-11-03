package platforms

import (
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Content represents platform-specific formatted content
type Content interface {
	// String returns a text representation for logging/preview
	String() string
}

// Platform represents a social media platform that can post vote content
type Platform interface {
	// Format converts vote groups into platform-specific content
	Format(group []votes.Vote) (Content, error)

	// Post publishes content to the platform and returns a ref to the
	// published post (the root, for a thread), which is recorded in the vote
	// log. A zero ref means nothing was published, as in stub mode.
	//
	// A non-zero ref returned together with an error means the post is live
	// but incomplete — a thread whose root went out and whose reply failed. The
	// caller records the vote as posted anyway, because publishing it a second
	// time is worse than a thread missing a card. A platform must therefore
	// return the ref the moment it has published anything that cannot be taken
	// back, and the zero ref only while nothing is public.
	// Returns shouldContinue=false if posting limit is reached
	Post(content Content) (post votelog.PostRef, shouldContinue bool, err error)

	// MaxPostsPerRun returns the maximum number of root posts per run.
	// Used by dry-run mode to simulate the same limit as real posting.
	MaxPostsPerRun() int

	// Name returns the platform identifier (e.g., "X", "Instagram")
	Name() string
}
