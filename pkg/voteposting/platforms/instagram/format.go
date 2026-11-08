package instagram

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// maxCaptionChars is Instagram's caption character limit.
const maxCaptionChars = 2200

// maxCarouselImages is the maximum number of images in an Instagram carousel.
const maxCarouselImages = 10

// Captions are intentionally German because post copy targets Zurich municipal council followers.
const captionTruncatedNoticeLine = "ℹ️ Gekürzt – weitere Teilabstimmungen im Link."

// InstagramContent implements platforms.Content for Instagram
type InstagramContent struct {
	Images  [][]byte // JPEG-encoded carousel images
	Caption string   // caption text accompanying the carousel
}

// String returns the text representation for logging/preview
func (c *InstagramContent) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📸 Instagram carousel: %d image(s)\n\n", len(c.Images)))
	sb.WriteString(c.Caption)
	return sb.String()
}

// FormatCarousel generates carousel images and builds the caption text for an Instagram post.
func FormatCarousel(group []votes.Vote) (*InstagramContent, error) {
	return FormatCarouselWithContacts(group, nil)
}

// FormatCarouselWithContacts generates carousel images and builds the caption text for an Instagram post,
// including mapped Instagram @mentions where supported by the title text.
func FormatCarouselWithContacts(group []votes.Vote, contactMapper *contacts.Mapper) (*InstagramContent, error) {
	if len(group) == 0 {
		return nil, fmt.Errorf("no votes provided")
	}

	// Generate carousel images
	images, err := imagegen.GenerateCarousel(group)
	if err != nil {
		return nil, fmt.Errorf("generating carousel images: %w", err)
	}

	// Enforce Instagram's 10-image carousel cap
	if len(images) > maxCarouselImages {
		images = images[:maxCarouselImages]
	}

	// Build caption text
	caption := buildCaption(group, contactMapper)

	return &InstagramContent{
		Images:  images,
		Caption: caption,
	}, nil
}

// buildCaption creates the caption text for an Instagram carousel post.
// The body is voteformat.LongFormBody, shared with Facebook, plus the link block.
func buildCaption(group []votes.Vote, contactMapper *contacts.Mapper) string {
	// A Wahlgeschäft gets its own caption: the title, the sentence saying we
	// have no result, and the links — no counts, no verdict emoji. Its only
	// votes are attendance roll calls, so this must run before anything below
	// touches voteformat.CountsOf, FormatVoteCounts* or a verdict emoji, none of
	// which mean anything for a headcount. It is checked on the whole group
	// because a second ballot round adds a second roll call to the same
	// business. The carousel image is handled the same way, in
	// imagegen.GenerateCarousel.
	// No handles are tagged on this path, deliberately: the only name in the
	// text belongs to the member being replaced, not to anyone elected.
	if votes.IsWahlgeschaeftGroup(group) {
		header := fmt.Sprintf("🗳️ %s\n\n", voteformat.PostHeadline(group))
		return header + voteformat.WahlgeschaeftBody(group) + voteformat.LinkLine(group)
	}

	var tags voteformat.Tags
	if contactMapper != nil {
		tags.Heading = func(text, jurisdiction string) string {
			return contactMapper.TagInstagramFraktionen(contactMapper.TagInstagramHandlesInText(text), jurisdiction)
		}
		tags.SubVote = contactMapper.TagInstagramFraktionen
	}
	// The next step rides with the links, in the tail a caption cut for length
	// keeps: it is one line, and on a ten-vote group it would otherwise be the
	// first thing the cut removes.
	tail := voteformat.LinkLine(group)
	if next := voteformat.NextStepLine(group); next != "" {
		tail = next + "\n\n" + strings.TrimLeft(tail, "\n")
	}
	return buildCaptionWithPreservedLink(voteformat.LongFormBody(group, tags), tail)
}

func buildCaptionWithPreservedLink(body, linkBlock string) string {
	body = strings.TrimRight(body, "\n")
	linkLine := strings.TrimLeft(linkBlock, "\n")
	caption := body + "\n\n" + linkLine

	// Truncate if over Instagram's character limit
	if len([]rune(caption)) > maxCaptionChars {
		tailWithNotice := captionTruncatedNoticeLine + "\n" + linkLine
		tailWithNoticeWithSeparator := "\n" + tailWithNotice

		if len([]rune(tailWithNotice)) > maxCaptionChars {
			// Extremely defensive fallback: keep at least the link if notice+link ever exceed the platform limit.
			linkRunes := []rune(linkLine)
			if len(linkRunes) <= maxCaptionChars {
				return linkLine
			}
			return string(linkRunes[:maxCaptionChars-1]) + "…"
		}

		availableBodyRunes := maxCaptionChars - len([]rune(tailWithNoticeWithSeparator))
		if availableBodyRunes <= 0 {
			// No room left for body text; publish only truncation notice + link.
			return tailWithNotice
		}

		body = truncateRunesWithEllipsis(body, availableBodyRunes)
		caption = body + tailWithNoticeWithSeparator
	}

	return caption
}

func truncateRunesWithEllipsis(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}
