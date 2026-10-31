package instagram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestFormatCarousel_EmptyVotes(t *testing.T) {
	_, err := FormatCarousel(nil)
	if err == nil {
		t.Error("expected error for nil group")
	}

	_, err = FormatCarousel([]votes.Vote{})
	if err == nil {
		t.Error("expected error for empty group")
	}
}

func TestFormatCarousel_SingleVote(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	// Single vote: 1 combined image
	if len(content.Images) != 1 {
		t.Errorf("expected 1 image for single vote, got %d", len(content.Images))
	}

	// Caption must contain key parts
	for _, part := range []string{
		"🗳️ Gemeinderat ZH",
		"Angenommen",
		"✅",
		"📊",
		"Ja",
		"Nein",
		"🔗",
	} {
		if !strings.Contains(content.Caption, part) {
			t.Errorf("caption missing %q\nFull caption:\n%s", part, content.Caption)
		}
	}
}

func TestFormatCarousel_MultiVote(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	// Multi-vote: 1 title card + 1 per vote = 3
	expectedImages := 1 + len(group)
	if len(content.Images) != expectedImages {
		t.Errorf("expected %d images for multi-vote, got %d", expectedImages, len(content.Images))
	}

	// Caption must contain vote subtitles
	for _, part := range []string{
		"Einleitungsartikel",
		"Schlussabstimmung",
		"🔗",
	} {
		if !strings.Contains(content.Caption, part) {
			t.Errorf("caption missing %q\nFull caption:\n%s", part, content.Caption)
		}
	}
}

func TestFormatCarousel_CaptionWithinLimit(t *testing.T) {
	// Test all fixtures to ensure no caption exceeds Instagram's 2200 char limit
	for name, group := range testfixtures.AllFixtures() {
		t.Run(name, func(t *testing.T) {
			content, err := FormatCarousel(group)
			if err != nil {
				t.Fatalf("FormatCarousel error: %v", err)
			}

			runeCount := len([]rune(content.Caption))
			if runeCount > maxCaptionChars {
				t.Errorf("caption exceeds %d chars: %d\nCaption:\n%s", maxCaptionChars, runeCount, content.Caption)
			}
		})
	}
}

func TestFormatCarousel_CarouselImageCap(t *testing.T) {
	// ten-vote-stress-test has 10 group → would produce 11 images (1 title + 10 results)
	// Instagram carousel cap is 10, so it should be trimmed
	group := testfixtures.TenVoteStressTest()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	if len(content.Images) > maxCarouselImages {
		t.Errorf("expected at most %d images (carousel cap), got %d", maxCarouselImages, len(content.Images))
	}
}

func TestFormatCarousel_AllFixtures(t *testing.T) {
	for name, group := range testfixtures.AllFixtures() {
		t.Run(name, func(t *testing.T) {
			content, err := FormatCarousel(group)
			if err != nil {
				t.Fatalf("FormatCarousel error: %v", err)
			}

			if len(content.Images) == 0 {
				t.Error("expected at least 1 image")
			}

			if len(content.Images) > maxCarouselImages {
				t.Errorf("images exceed carousel cap: %d > %d", len(content.Images), maxCarouselImages)
			}

			if content.Caption == "" {
				t.Error("caption should not be empty")
			}

			// Caption should always contain a link
			if !strings.Contains(content.Caption, "🔗") {
				t.Errorf("caption should contain a link\n%s", content.Caption)
			}

			// Verify images are valid JPEG (check first 2 bytes: FF D8)
			for i, img := range content.Images {
				if len(img) < 2 || img[0] != 0xFF || img[1] != 0xD8 {
					t.Errorf("image %d is not a valid JPEG", i)
				}
			}
		})
	}
}

func TestFormatCarousel_CaptionContainsVoteLink(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	if !strings.Contains(content.Caption, "gemeinderat-zuerich.ch") {
		t.Errorf("caption should contain vote link\n%s", content.Caption)
	}
}

func TestFormatCarousel_LongMultiVoteCaptionPreservesLink(t *testing.T) {
	group := testfixtures.InstagramLongMultiVoteTruncation()

	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	expectedLink := group[0].GroupURL
	expectedLinkLine := "🔗 " + expectedLink

	if len([]rune(content.Caption)) > maxCaptionChars {
		t.Errorf("caption exceeds %d chars: %d", maxCaptionChars, len([]rune(content.Caption)))
	}

	if !strings.Contains(content.Caption, expectedLinkLine) {
		t.Errorf("caption should contain full link line %q\n%s", expectedLinkLine, content.Caption)
	}

	if !strings.Contains(content.Caption, captionTruncatedNoticeLine) {
		t.Errorf("caption should contain truncation notice %q\n%s", captionTruncatedNoticeLine, content.Caption)
	}

	// The link block, credit line included, survives truncation intact.
	linkBlock := strings.TrimLeft(voteformat.LinkLine(group), "\n")
	if !strings.HasSuffix(content.Caption, linkBlock) {
		t.Errorf("caption should end with the link block %q\n%s", linkBlock, content.Caption)
	}
}

func TestFormatCarousel_ShortCaptionHasNoTruncationNotice(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	if strings.Contains(content.Caption, captionTruncatedNoticeLine) {
		t.Errorf("short caption should not contain truncation notice\n%s", content.Caption)
	}
}

func TestFormatCarousel_CaptionContainsFraktionBreakdown(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	if !strings.Contains(content.Caption, "🏛️ Fraktionen") {
		t.Errorf("caption should contain Fraktion breakdown\n%s", content.Caption)
	}
}

func TestFormatCarousel_AuswahlVoteNoResultEmoji(t *testing.T) {
	group := testfixtures.AuswahlVote()
	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel error: %v", err)
	}

	// Auswahl group should not have ✅/❌ in caption
	if strings.Contains(content.Caption, "✅") || strings.Contains(content.Caption, "❌") {
		t.Errorf("Auswahl vote caption should not contain result emoji\n%s", content.Caption)
	}

	// But should contain A/B/C counts
	if !strings.Contains(content.Caption, "A:") || !strings.Contains(content.Caption, "B:") {
		t.Errorf("Auswahl vote caption should contain A/B counts\n%s", content.Caption)
	}
}

func TestContentString(t *testing.T) {
	content := &InstagramContent{
		Images:  make([][]byte, 3),
		Caption: "Test caption",
	}

	s := content.String()
	if !strings.Contains(s, "3 image(s)") {
		t.Errorf("String() should include image count, got: %s", s)
	}
	if !strings.Contains(s, "Test caption") {
		t.Errorf("String() should include caption, got: %s", s)
	}
}

func TestFormatCarouselWithContacts_TagsInstagramHandlesInTitle(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	group[0].Title = "Postulat von Anna Graff"

	contactsFile := filepath.Join(t.TempDir(), "contacts.yaml")
	err := os.WriteFile(contactsFile, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    instagram:
      - url: https://www.instagram.com/annagraff_/
        verified: true
`), 0o600)
	if err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	mapper, err := contacts.LoadContacts(contactsFile)
	if err != nil {
		t.Fatalf("load contacts: %v", err)
	}

	content, err := FormatCarouselWithContacts(group, mapper)
	if err != nil {
		t.Fatalf("FormatCarouselWithContacts error: %v", err)
	}

	if !strings.Contains(content.Caption, "Anna Graff @annagraff_") {
		t.Errorf("expected caption to include tagged Instagram handle\n%s", content.Caption)
	}
}

func intPtr(i int) *int {
	return &i
}

func TestFormatCarousel_SingleVoteSubtitlePrefix(t *testing.T) {
	tests := []struct {
		name             string
		group            []votes.Vote
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "Single vote non-Schlussabstimmung prepends Abstimmungsgegenstand",
			group: []votes.Vote{
				{
					SourceID:   "test-guid-dring",
					Title:      "2026/244 Motion von Dr. Jonas Keller (SP): Erhalt Konzertlokale",
					Subtitle:   "2026_0244 Dringlicherklärung",
					Date:       testfixtures.MustDate("2026-05-27"),
					Decision:   "angenommen",
					Yes:        intPtr(66),
					No:         intPtr(0),
					Abstention: intPtr(0),
					Absent:     intPtr(59),
					SourceURL:  testfixtures.VoteURL("test-guid-dring"),
				},
			},
			shouldContain:    []string{"Dringlicherklärung\n"},
			shouldNotContain: []string{},
		},
		{
			name: "Single vote Schlussabstimmung does NOT prepend",
			group: []votes.Vote{
				{
					SourceID:   "test-guid-schluss",
					Title:      "2026/244 Motion von Dr. Jonas Keller (SP): Erhalt Konzertlokale",
					Subtitle:   "2026_0244 Schlussabstimmung",
					Date:       testfixtures.MustDate("2026-05-27"),
					Decision:   "angenommen",
					Yes:        intPtr(66),
					No:         intPtr(0),
					Abstention: intPtr(0),
					Absent:     intPtr(59),
					SourceURL:  testfixtures.VoteURL("test-guid-schluss"),
				},
			},
			shouldContain:    []string{},
			shouldNotContain: []string{"Schlussabstimmung\n"},
		},
		{
			name: "Single vote empty Abstimmungstitel does NOT prepend",
			group: []votes.Vote{
				{
					SourceID:   "test-guid-empty",
					Title:      "2026/244 Motion von Dr. Jonas Keller (SP): Erhalt Konzertlokale",
					Subtitle:   "",
					Date:       testfixtures.MustDate("2026-05-27"),
					Decision:   "angenommen",
					Yes:        intPtr(66),
					No:         intPtr(0),
					Abstention: intPtr(0),
					Absent:     intPtr(59),
					SourceURL:  testfixtures.VoteURL("test-guid-empty"),
				},
			},
			shouldContain:    []string{},
			shouldNotContain: []string{"\n\n\n"},
		},
		{
			name:             "Multi-vote does NOT prepend subtitle to caption",
			group:            testfixtures.MultiVoteGroup(),
			shouldContain:    []string{},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := FormatCarousel(tt.group)
			if err != nil {
				t.Fatalf("FormatCarousel error: %v", err)
			}
			caption := content.Caption
			t.Logf("Caption:\n%s", caption)

			for _, s := range tt.shouldContain {
				if !strings.Contains(caption, s) {
					t.Errorf("Expected caption to contain %q", s)
				}
			}
			for _, s := range tt.shouldNotContain {
				if strings.Contains(caption, s) {
					t.Errorf("Expected caption NOT to contain %q", s)
				}
			}
		})
	}
}

// A body whose titles carry no names — Kanton Zürich's never do — is named only
// on the label line, so tagging the title alone would tag nobody.
func TestFormatCarouselWithContacts_TagsAuthorsOnTheLabelLine(t *testing.T) {
	group := testfixtures.KantonsratMemberBusiness()

	contactsFile := filepath.Join(t.TempDir(), "contacts.yaml")
	err := os.WriteFile(contactsFile, []byte(`version: "1.0"
contacts:
  - name: Tobias Weidmann
    instagram:
      - url: https://www.instagram.com/tobiasweidmann/
        verified: true
`), 0o600)
	if err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	mapper, err := contacts.LoadContacts(contactsFile)
	if err != nil {
		t.Fatalf("load contacts: %v", err)
	}

	content, err := FormatCarouselWithContacts(group, mapper)
	if err != nil {
		t.Fatalf("FormatCarouselWithContacts error: %v", err)
	}

	if !strings.Contains(content.Caption, "Postulat von Tobias Weidmann @tobiasweidmann (SVP)") {
		t.Errorf("the author on the label line was not tagged\n%s", content.Caption)
	}
}

// The caption has room for a paragraph, so it never sheds a name: everyone who
// signed is in the one block a reader sees.
func TestFormatCarousel_CaptionNamesEverySignatory(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()

	content, err := FormatCarousel(group)
	if err != nil {
		t.Fatalf("FormatCarousel: %v", err)
	}

	for _, a := range group[0].Affair.Authors {
		if !strings.Contains(content.Caption, a.Name) {
			t.Errorf("%s is missing from the caption:\n%s", a.Name, content.Caption)
		}
	}
	if strings.Contains(content.Caption, "u. a.") {
		t.Errorf("the caption shortened a list it had room for:\n%s", content.Caption)
	}
	if strings.Contains(content.Caption, "Weitere Unterzeichnende") {
		t.Errorf("the caption split a list it had room for:\n%s", content.Caption)
	}
}
