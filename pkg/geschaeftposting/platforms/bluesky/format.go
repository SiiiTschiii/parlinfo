// Package bluesky formats Geschäft announcements as Bluesky threads, posted
// through the vote platform's Post.
package bluesky

import (
	"strings"
	"unicode/utf8"

	"github.com/siiitschiii/parlinfo/pkg/bskyapi"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	bsky "github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// maxGraphemes is the Bluesky post limit.
const maxGraphemes = 300

// artenLabel is the visible text of the link to the council's explanation of
// every Geschäftsart.
const artenLabel = "Geschäftsarten erklärt"

// Formatter returns a geschaeftposting.Formatter tagging members known to
// contactMapper, which may be nil.
func Formatter(contactMapper *contacts.Mapper) geschaeftposting.Formatter {
	return func(g zurichapi.Geschaeft) (platforms.Content, error) {
		return bsky.NewContent(FormatThread(g, contactMapper)), nil
	}
}

// FormatThread builds the announcement: a root post with the news, then the
// explainer as a reply.
//
// Links are written as short labels carrying a link facet, not as raw URLs.
// Bluesky counts every grapheme of a URL, and the Geschäft page alone is 93 —
// a third of the post, taken from the title. A facet makes the label as
// clickable as the URL would have been.
func FormatThread(g zurichapi.Geschaeft, contactMapper *contacts.Mapper) []*bsky.BlueskyPost {
	label := linkLabel(g)
	rootText := geschaeftposting.FitRoot(g, maxGraphemes, graphemeLen, nil, "🔗 "+label)
	root := &bsky.BlueskyPost{Text: rootText, Facets: linkFacet(rootText, label, geschaeftposting.Link(g))}
	if contactMapper != nil {
		root.Mentions = contactMapper.FindBlueskyMentions(rootText)
	}

	thread := []*bsky.BlueskyPost{root}
	for _, text := range explainerTexts(g) {
		thread = append(thread, &bsky.BlueskyPost{Text: text, Facets: linkFacet(text, artenLabel, geschaeftposting.ArtenURL)})
	}
	return thread
}

// linkLabel names the Geschäft by its GR number, which is what the council's
// own pages and the press use to refer to it.
func linkLabel(g zurichapi.Geschaeft) string {
	if g.GRNr == "" {
		return "Zum Geschäft"
	}
	return "GR Nr. " + g.GRNr
}

// explainerTexts is the explainer reply, split in two where it outgrows one
// post. Both of today's types fit in one; the split is for the next type whose
// procedure takes more words to say.
func explainerTexts(g zurichapi.Geschaeft) []string {
	a, _ := geschaeftposting.ArtOf(g)
	link := "\n\n📖 " + artenLabel
	if whole := geschaeftposting.Explainer(g) + link; graphemeLen(whole) <= maxGraphemes {
		return []string{whole}
	}
	return []string{a.Explanation, a.NextSteps + link}
}

// linkFacet links the last occurrence of label in text to uri. The last,
// because the label closes the post and a title could contain the same words.
func linkFacet(text, label, uri string) []bskyapi.Facet {
	idx := strings.LastIndex(text, label)
	if idx < 0 {
		return nil
	}
	return []bskyapi.Facet{bskyapi.LinkFacet(idx, idx+len(label), uri)}
}

// graphemeLen counts runes, which overcounts emoji with variation selectors —
// the safe direction for a limit the API enforces.
func graphemeLen(s string) int {
	return utf8.RuneCountInString(s)
}
