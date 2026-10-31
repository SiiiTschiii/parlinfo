// Package instagram formats Geschäft announcements as a single Instagram image
// with a caption, posted through the vote platform's Post.
package instagram

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	igvote "github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// Formatter returns a geschaeftposting.Formatter tagging members known to
// contactMapper, which may be nil.
func Formatter(contactMapper *contacts.Mapper) geschaeftposting.Formatter {
	return func(g zurichapi.Geschaeft) (platforms.Content, error) {
		return Format(g, contactMapper)
	}
}

// Format renders the card and the caption.
func Format(g zurichapi.Geschaeft, contactMapper *contacts.Mapper) (*igvote.InstagramContent, error) {
	if !geschaeftposting.Supported(g) {
		return nil, fmt.Errorf("%s: Geschaeftsart %q is not announced", g.GRNr, g.Geschaeftsart)
	}

	date := ""
	if g.Beginn.Text != "" {
		date = "Eingereicht am " + g.Beginn.Text
	}
	img, err := imagegen.GenerateGeschaeftCard(imagegen.GeschaeftCard{
		Kind:      g.Geschaeftsart,
		Number:    g.GRNr,
		Dringlich: g.Dringlich,
		Title:     geschaeftposting.Title(g),
		Signers:   strings.TrimPrefix(geschaeftposting.SignerLine(g, 0), "✍️ "),
		Date:      date,
		Body:      geschaeftposting.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("generating Geschäft card: %w", err)
	}

	return &igvote.InstagramContent{Images: [][]byte{img}, Caption: Caption(g, contactMapper)}, nil
}

// Caption is the root post and the explainer in one, since Instagram has no
// threads, with every signer named — the 2200-character caption has room the
// text platforms do not.
//
// Links go in as text even though Instagram will not make them clickable: a
// reader can still copy them, and the Geschäft page is the only place the
// Vorstoss's own wording is published.
func Caption(g zurichapi.Geschaeft, contactMapper *contacts.Mapper) string {
	a, _ := geschaeftposting.ArtOf(g)

	signers := geschaeftposting.SignerLine(g, 0)
	if contactMapper != nil {
		signers = contactMapper.TagInstagramHandlesInText(signers)
	}
	var meta []string
	for _, l := range []string{signers, geschaeftposting.DateLine(g)} {
		if l != "" {
			meta = append(meta, l)
		}
	}

	parts := []string{geschaeftposting.Headline(g), geschaeftposting.Title(g)}
	if len(meta) > 0 {
		parts = append(parts, strings.Join(meta, "\n"))
	}
	parts = append(parts,
		a.Explanation,
		a.NextSteps,
		"🔗 "+geschaeftposting.Link(g)+"\n📖 "+geschaeftposting.ArtenURL,
	)
	return strings.Join(parts, "\n\n")
}
