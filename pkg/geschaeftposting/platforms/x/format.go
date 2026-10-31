// Package x formats Geschäft announcements as X threads, posted through the
// vote platform's Post.
package x

import (
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	xvote "github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// Formatter returns a geschaeftposting.Formatter for posts of at most
// charLimit, tagging members known to contactMapper, which may be nil.
func Formatter(contactMapper *contacts.Mapper, charLimit int) geschaeftposting.Formatter {
	return func(g zurichapi.Geschaeft) (platforms.Content, error) {
		return xvote.NewContent(FormatThread(g, contactMapper, charLimit)), nil
	}
}

// FormatThread builds the announcement: a root post with the news and the
// Geschäft link, then the explainer as a reply.
//
// Unlike Bluesky, the full URL goes in the text: X charges 23 for any link, and
// renders no link that is not in the text.
func FormatThread(g zurichapi.Geschaeft, contactMapper *contacts.Mapper, charLimit int) []*xvote.XPost {
	var tag func(string) string
	if contactMapper != nil {
		tag = contactMapper.TagXHandlesInText
	}
	root := geschaeftposting.FitRoot(g, charLimit, xvote.WeightedLen, tag, "🔗 "+geschaeftposting.Link(g))

	thread := []*xvote.XPost{{Text: root}}
	a, _ := geschaeftposting.ArtOf(g)
	link := "\n\n📖 " + geschaeftposting.ArtenURL
	if whole := geschaeftposting.Explainer(g) + link; xvote.WeightedLen(whole) <= charLimit {
		return append(thread, &xvote.XPost{Text: whole})
	}
	// At 280 the explainer does not fit beside the link, so it splits where
	// its two halves meet rather than mid-sentence.
	return append(thread, &xvote.XPost{Text: a.Explanation}, &xvote.XPost{Text: a.NextSteps + link})
}
