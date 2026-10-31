package imagegen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestGenerateCarousel_ValidJPEG(t *testing.T) {
	fixtures := testfixtures.AllFixtures()
	for name, group := range fixtures {
		t.Run(name, func(t *testing.T) {
			images, err := GenerateCarousel(group)
			if err != nil {
				t.Fatalf("GenerateCarousel failed: %v", err)
			}
			if len(images) == 0 {
				t.Fatal("expected at least one image")
			}
			// Single vote: 1 combined image. Multi-vote: 1 title + 1 result per
			// vote. A Wahlgeschäft is one text card however many roll calls it
			// holds, because a result card would render a headcount as a result.
			var expected int
			switch {
			case votes.IsWahlgeschaeftGroup(group), len(group) == 1:
				expected = 1
			default:
				expected = 1 + len(group)
			}
			if len(images) != expected {
				t.Errorf("expected %d images, got %d", expected, len(images))
			}
			for i, imgData := range images {
				cfg, err := jpeg.DecodeConfig(bytes.NewReader(imgData))
				if err != nil {
					t.Errorf("image %d: not valid JPEG: %v", i, err)
					continue
				}
				if cfg.Width != 1080 || cfg.Height != 1350 {
					t.Errorf("image %d: expected 1080x1350, got %dx%d", i, cfg.Width, cfg.Height)
				}
				// Check file size < 500KB
				if len(imgData) > 500*1024 {
					t.Errorf("image %d: size %d bytes exceeds 500KB", i, len(imgData))
				}
			}
		})
	}
}

// Every card carries alt text, and it says what that card shows: the title on
// the first card, each vote's own counts on its result card.
func TestGenerateCards_AltTextFollowsTheCards(t *testing.T) {
	fixtures := testfixtures.AllFixtures()
	// Its 1,739-character vote label would push the counts past the cap if the
	// label did not give way first.
	fixtures["instagram-long-multi-vote-truncation"] = testfixtures.InstagramLongMultiVoteTruncation()
	for name, group := range fixtures {
		t.Run(name, func(t *testing.T) {
			cards, err := GenerateCards(group)
			if err != nil {
				t.Fatalf("GenerateCards failed: %v", err)
			}
			images, err := GenerateCarousel(group)
			if err != nil {
				t.Fatalf("GenerateCarousel failed: %v", err)
			}
			if len(cards) != len(images) {
				t.Fatalf("got %d cards, want one per carousel image (%d)", len(cards), len(images))
			}
			for i, c := range cards {
				if strings.TrimSpace(c.Alt) == "" {
					t.Errorf("card %d has no alt text", i)
				}
				if n := len([]rune(c.Alt)); n > maxAltRunes {
					t.Errorf("card %d alt text is %d runes, over the %d cap", i, n, maxAltRunes)
				}
			}

			title := []rune(voteformat.CleanVoteTitle(group[0].Title))
			if len(title) > 40 {
				title = title[:40] // a long title is shortened, never dropped
			}
			if len(title) > 0 && !strings.Contains(cards[0].Alt, string(title)) {
				t.Errorf("first card's alt text does not name the business %q:\n%s", string(title), cards[0].Alt)
			}
			if len(cards) == len(group)+1 {
				for i, v := range group {
					// The overview lists every vote's result, as the card does.
					// Where ten of them crowd the cap their labels are cut
					// shorter, but every vote keeps its numbered line.
					line, ok := formatSummaryLine(i+1, v, len(group))
					if ok && len(group) <= 4 && !strings.Contains(cards[0].Alt, line) {
						t.Errorf("overview alt text lacks vote %d's summary %q:\n%s", i+1, line, cards[0].Alt)
					}
					if ok && !strings.Contains(cards[0].Alt, fmt.Sprintf("\n%d. ", i+1)) {
						t.Errorf("overview alt text does not list vote %d:\n%s", i+1, cards[0].Alt)
					}
					counts := voteformat.FormatVoteCountsLong(voteformat.CountsOf(v))
					if !strings.Contains(cards[i+1].Alt, counts) {
						t.Errorf("result card %d alt text lacks its counts %q:\n%s", i+1, counts, cards[i+1].Alt)
					}
				}
			}
			// An Auswahl card prints the chosen option; so must its description.
			for i, v := range group {
				outcome := auswahlOutcome(v)
				if outcome == "" || votes.IsWahlgeschaeftGroup(group) {
					continue
				}
				card := cards[0]
				if len(cards) == len(group)+1 {
					card = cards[i+1]
				}
				if !strings.Contains(card.Alt, outcome) {
					t.Errorf("vote %d alt text lacks the Auswahl outcome %q:\n%s", i+1, outcome, card.Alt)
				}
			}
		})
	}
}

func TestAssembleAlt_LabelGivesWayToResult(t *testing.T) {
	result := "📊 130 Ja | 44 Nein"
	got := assembleAlt(strings.Repeat("Antrag ", 500), result)
	if !strings.HasSuffix(got, result) {
		t.Errorf("the result did not survive the cap:\n%s", got)
	}
	if n := len([]rune(got)); n > maxAltRunes {
		t.Errorf("alt text is %d runes, over the %d cap", n, maxAltRunes)
	}
	// A result longer than the cap still leaves the label its floor.
	got = assembleAlt(strings.Repeat("Titel ", 200), strings.Repeat("1. ✅ Ziffer\n", 200))
	if !strings.HasPrefix(got, strings.Repeat("Titel ", 40)) {
		t.Errorf("the label lost its floor:\n%s", got)
	}
	if n := len([]rune(got)); n > maxAltRunes {
		t.Errorf("alt text is %d runes, over the %d cap", n, maxAltRunes)
	}
	if got := assembleAlt("Titel", "Ergebnis"); got != "Titel\n\nErgebnis" {
		t.Errorf("short alt text changed: %q", got)
	}
}

func TestCapRunes(t *testing.T) {
	if got := capRunes("Abstimmung", 20); got != "Abstimmung" {
		t.Errorf("short text changed: %q", got)
	}
	if got := capRunes("Abstimmungsergebnis", 6); got != "Absti…" {
		t.Errorf("capRunes = %q, want %q", got, "Absti…")
	}
}

func TestGenerateCarousel_Empty(t *testing.T) {
	_, err := GenerateCarousel(nil)
	if err == nil {
		t.Fatal("expected error for empty group")
	}
}

func TestLayoutResultCard_WrapsLongSubtitle(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	group := testfixtures.MultiVoteGroup()
	shortVote := group[0]
	shortVote.Subtitle = "Kurz"

	longVote := group[0]
	longVote.Subtitle = "Änderungsantrag zur Teilrevision der Gemeindeordnung mit zusätzlichen Bestimmungen zur Stadtentwicklung und Raumplanung"

	bg := SelectColor("zurich-city", shortVote.Affair.Number)

	shortCur := newCursor(0, imgHeight)
	layoutResultCard(nil, shortCur, bandInset(shortVote), &shortVote, bg, fonts, 1, 2)

	longCur := newCursor(0, imgHeight)
	layoutResultCard(nil, longCur, bandInset(longVote), &longVote, bg, fonts, 2, 2)

	if longCur.contentHeight() <= shortCur.contentHeight() {
		t.Fatalf("expected wrapped long subtitle to use more vertical space (short=%d, long=%d)", shortCur.contentHeight(), longCur.contentHeight())
	}
}

func TestLayoutTitleCard_WrapsLongSummaryLine(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	shortVotes := testfixtures.MultiVoteGroup()
	shortVotes[0].Subtitle = "Einleitung"
	shortVotes[1].Subtitle = "Schluss"

	longVotes := testfixtures.MultiVoteGroup()
	longVotes[0].Subtitle = "Einleitung"
	longVotes[1].Subtitle = "Schlussabstimmung mit zusätzlichen Bestimmungen zur Neuordnung der Kompetenzen im Bereich Stadtentwicklung und Raumplanung"

	bg := SelectColor("zurich-city", shortVotes[0].Affair.Number)

	shortCur := newCursor(0, imgHeight)
	layoutTitleCard(nil, shortCur, bandInset(shortVotes[0]), shortVotes, bg, fonts)

	longCur := newCursor(0, imgHeight)
	layoutTitleCard(nil, longCur, bandInset(longVotes[0]), longVotes, bg, fonts)

	if longCur.contentHeight() <= shortCur.contentHeight() {
		t.Fatalf("expected wrapped long summary line to use more vertical space (short=%d, long=%d)", shortCur.contentHeight(), longCur.contentHeight())
	}
}

func TestLayoutCombinedCard_AbstimmungsgegenstandPrefix(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	base := testfixtures.SingleVoteAngenommen()[0]
	bg := SelectColor("zurich-city", base.Affair.Number)

	// Non-Schlussabstimmung Abstimmungstitel: prepended inline in front of the
	// title, the same way the title card does it.
	withPrefix := base
	withPrefix.Subtitle = "Dringlicherklärung"
	prefixCur := newCursor(0, imgHeight)
	_, prefixLines, err := layoutCombinedCard(nil, prefixCur, 0, &withPrefix, bg, fonts)
	if err != nil {
		t.Fatalf("layoutCombinedCard failed: %v", err)
	}
	if got := strings.Join(prefixLines, " "); !strings.HasPrefix(got, "Dringlicherklärung: ") {
		t.Fatalf("expected title to start with %q, got %q", "Dringlicherklärung: ", got)
	}

	// The ballot type stays out of it: the card would otherwise open
	// "Dringlicherklärung · Ausgabenbremse: …", gluing a fact about the ballot
	// onto a fact about the business. It gets a line with the counts instead.
	withType := withPrefix
	withType.Type = "Ausgabenbremse"
	typeCur := newCursor(0, imgHeight)
	_, typeLines, err := layoutCombinedCard(nil, typeCur, 0, &withType, bg, fonts)
	if err != nil {
		t.Fatalf("layoutCombinedCard failed: %v", err)
	}
	if got := strings.Join(typeLines, " "); strings.Contains(got, "Ausgabenbremse") {
		t.Fatalf("expected the ballot type to stay out of the title, got %q", got)
	}
	if typeCur.contentHeight() <= prefixCur.contentHeight() {
		t.Fatalf("expected the ballot type to take a line above the counts (untyped=%d, typed=%d)",
			prefixCur.contentHeight(), typeCur.contentHeight())
	}

	// Schlussabstimmung Abstimmungstitel: no prefix added.
	schluss := base
	schluss.Subtitle = "Schlussabstimmung"
	schlussCur := newCursor(0, imgHeight)
	_, schlussLines, err := layoutCombinedCard(nil, schlussCur, 0, &schluss, bg, fonts)
	if err != nil {
		t.Fatalf("layoutCombinedCard failed: %v", err)
	}
	if got := strings.Join(schlussLines, " "); strings.Contains(got, "Schlussabstimmung:") {
		t.Fatalf("expected no prefix for Schlussabstimmung, got %q", got)
	}

	// Empty Abstimmungstitel: no prefix added.
	none := base
	none.Subtitle = ""
	noneCur := newCursor(0, imgHeight)
	_, noneLines, err := layoutCombinedCard(nil, noneCur, 0, &none, bg, fonts)
	if err != nil {
		t.Fatalf("layoutCombinedCard failed: %v", err)
	}
	if got := strings.Join(noneLines, " "); strings.Contains(got, ": ") && strings.HasPrefix(got, "Dringlicherklärung") {
		t.Fatalf("expected no prefix for empty Abstimmungstitel, got %q", got)
	}
}

func TestSelectColor_Deterministic(t *testing.T) {
	c1 := SelectColor("zurich-city", "2025/100")
	c2 := SelectColor("zurich-city", "2025/100")
	if c1 != c2 {
		t.Error("same input should produce same color")
	}
}

func TestSelectColor_DifferentInputs(t *testing.T) {
	c1 := SelectColor("zurich-city", "2025/100")
	c2 := SelectColor("zurich-city", "2025/101")
	// Different inputs should (usually) produce different colors
	// This is probabilistic but with our palette it's very likely
	if c1 == c2 {
		t.Log("warning: different inputs produced same color (possible but unlikely)")
	}
}

func TestFormatSummaryLine_NumberingAndTruncation(t *testing.T) {
	vote := votes.Vote{
		Subtitle: "Antrag SP sehr lange Beschreibung mit noch mehr Details für die Übersicht auf der Titelfolie und weiteren Erläuterungen zur Kompetenzordnung im Bereich Stadtentwicklung und Raumplanung",
		Decision: "angenommen",
	}

	line, ok := formatSummaryLine(2, vote, 3)
	if !ok {
		t.Fatal("expected summary line to be generated")
	}
	if !strings.HasPrefix(line, "2. ✅ ") {
		t.Fatalf("expected numbered line with emoji prefix, got %q", line)
	}
	if !strings.Contains(line, "…") {
		t.Fatalf("expected truncated subtitle with ellipsis, got %q", line)
	}
}

func TestFormatSummaryLine_AuswahlVote(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	vote := votes.Vote{
		Subtitle: "Änderungsantrag 17",
		Decision: "Auswahl A",
		Absent:   intPtr(11),
		ChoiceA:  intPtr(50),
		ChoiceB:  intPtr(24),
		ChoiceC:  intPtr(40),
	}

	line, ok := formatSummaryLine(2, vote, 3)
	if !ok {
		t.Fatal("expected summary line to be generated")
	}
	if !strings.HasPrefix(line, "2. [A] ") {
		t.Fatalf("expected Auswahl summary line to show bracket label, got %q", line)
	}
	if strings.Contains(line, "❌") || strings.Contains(line, "✅") {
		t.Fatalf("Auswahl summary line should not contain a result emoji, got %q", line)
	}
}

func TestFormatProgressBadge(t *testing.T) {
	if got := formatProgressBadge(2, 3); got != "2/3" {
		t.Fatalf("expected 2/3, got %q", got)
	}
	if got := formatProgressBadge(0, 3); got != "" {
		t.Fatalf("expected empty badge for invalid index, got %q", got)
	}
	if got := formatProgressBadge(1, 1); got != "" {
		t.Fatalf("expected empty badge for single vote, got %q", got)
	}
}

func TestDrawFraktionTable_AddsRowSpacing(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	fraktionCounts := map[string]*voteformat.FraktionCounts{
		"SP":  {Counts: map[string]int{"Ja": 20, "Nein": 5}},
		"FDP": {Counts: map[string]int{"Ja": 10, "Nein": 15}},
	}

	startY := 100
	cur := newCursor(startY, 600)
	drawFraktionTable(nil, cur, fraktionCounts, SelectColor("zurich-city", "2025/100"), fonts.partyBold, fonts.partyNum)

	rowHeight := lineHeight(fonts.partyNum)
	rowGap := int(float64(rowHeight) * fraktionRowGapFactor)
	expectedY := startY + 3*rowHeight + 2*rowGap // header + header-gap + 2 rows + 1 gap between rows
	if cur.y != expectedY {
		t.Fatalf("expected y=%d, got %d", expectedY, cur.y)
	}
}

func TestDrawFraktionTable_LimitsRowsWhenSpaceIsTight(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	fraktionCounts := map[string]*voteformat.FraktionCounts{}
	for i := 0; i < 12; i++ {
		fraktionCounts[fmt.Sprintf("Fraktion-%d", i)] = &voteformat.FraktionCounts{
			Counts: map[string]int{"Ja": 1},
		}
	}

	rowHeight := lineHeight(fonts.partyNum)
	rowGap := int(float64(rowHeight) * fraktionRowGapFactor)
	rowStride := rowHeight + rowGap
	maxRows := 3

	// The clamp is a drawing-pass guard, not a measuring one: the measuring
	// pass has to report the card's true height so renderCard can
	// centre it without pushing rows out of the frame. So this needs a real
	// destination image to exercise it.
	customImgHeight := padding + rowHeight + rowGap + maxRows*rowStride
	bg := SelectColor("zurich-city", "2025/100")
	cur := newCursor(0, customImgHeight)
	drawFraktionTable(newImage(bg), cur, fraktionCounts, bg, fonts.partyBold, fonts.partyNum)

	expectedY := rowHeight + rowGap + maxRows*rowHeight + (maxRows-1)*rowGap
	if cur.y != expectedY {
		t.Fatalf("expected y=%d with max %d rows, got %d", expectedY, maxRows, cur.y)
	}
}

// TestMeasureMixedTextMatchesWhatIsDrawn pins the centring arithmetic to the
// faces actually used.
//
// The verdict emoji renders in NotoEmoji 72 while the text around it is gobold
// 64, so measuring an emoji with the text face reports a glyph that is never
// drawn — 48px against the real 91px. A card whose verdict was nothing but "❌"
// came out 21px right of centre, plainly off against the title beneath it.
func TestMeasureMixedTextMatchesWhatIsDrawn(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet: %v", err)
	}

	for _, emoji := range []string{"❌", "✅"} {
		mixed := measureMixedText(fonts.verdict, fonts.emojiVerdict, emoji)
		drawn := font.MeasureString(fonts.emojiVerdict, emoji).Ceil()
		if mixed != drawn {
			t.Errorf("measureMixedText(%q) = %d, but it is drawn %d wide", emoji, mixed, drawn)
		}
		if textOnly := font.MeasureString(fonts.verdict, emoji).Ceil(); textOnly == drawn {
			t.Skipf("faces now agree on %q; this test no longer proves anything", emoji)
		}
	}
}

// TestMeasureMixedTextLeavesPlainTextAlone checks the fix did not move every
// other string on the card.
func TestMeasureMixedTextLeavesPlainTextAlone(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet: %v", err)
	}

	const plain = "Mitteilungen"
	got := measureMixedText(fonts.verdict, fonts.emojiVerdict, plain)
	want := font.MeasureString(fonts.verdict, plain).Ceil()
	if got != want {
		t.Errorf("measureMixedText(%q) = %d, want %d — plain text must measure as before", plain, got, want)
	}
}

// TestCardsKeepEveryFraktion checks that no card drops rows from its party
// breakdown.
//
// drawFraktionTable stops at imgHeight-padding, so a card laid out too low
// simply loses its last Fraktionen — with nothing on the card to say any were
// dropped, and the breakdown is the one part of a vote a reader cannot
// reconstruct from the caption. The check mirrors what renderCard does: it
// measures the card, centres it the way the real render does, and asserts the
// drawing pass covers the same height the measuring pass reported. A dropped
// row makes the drawn card shorter than the measured one.
func TestCardsKeepEveryFraktion(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	eachCard(t, fonts, func(t *testing.T, c card, bg color.RGBA) {
		inset := c.inset(fonts)
		dry := newCursor(inset, imgHeight)
		mustLayout(t, c, nil, dry, inset)

		startY := centredStart(inset, dry.contentHeight())
		if bottom := startY + dry.contentHeight(); bottom > imgHeight-padding {
			t.Errorf("%s: content ends at y=%d, past the %dpx bottom limit", c.name, bottom, imgHeight-padding)
		}

		real := newCursor(startY, imgHeight)
		mustLayout(t, c, newImage(bg), real, inset)
		if real.contentHeight() != dry.contentHeight() {
			t.Errorf("%s: drawn card is %dpx tall against %dpx measured — the table dropped rows",
				c.name, real.contentHeight(), dry.contentHeight())
		}
	})
}

// TestCardsClearTheBodyBand checks the margin under the band.
//
// A card whose content is centred but never charged for that margin fills the
// frame instead: on the 03.12.2025 Postulat the verdict emoji sat 2px below the
// band, reading as though it were part of it. titleBudget charges the title for
// the margin, so a card that tall gives up a font size and keeps its breathing
// room — which is what this asserts, for every card of every fixture.
func TestCardsClearTheBodyBand(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	eachCard(t, fonts, func(t *testing.T, c card, _ color.RGBA) {
		inset := c.inset(fonts)
		dry := newCursor(inset, imgHeight)
		mustLayout(t, c, nil, dry, inset)

		startY := centredStart(inset, dry.contentHeight())
		if gap := startY - inset; gap < padding {
			t.Errorf("%s: content starts %dpx below the band, under the %dpx margin (card is %dpx tall)",
				c.name, gap, padding, dry.contentHeight())
		}
	})
}

// eachCard runs fn over every card every fixture produces — the cards
// carouselCards hands GenerateCarousel — as a subtest per fixture.
func eachCard(t *testing.T, fonts *fontSet, fn func(*testing.T, card, color.RGBA)) {
	t.Helper()

	for name, group := range testfixtures.AllFixtures() {
		t.Run(name, func(t *testing.T) {
			bg := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)
			for _, c := range carouselCards(group, bg, fonts) {
				fn(t, c, bg)
			}
		})
	}
}

// mustLayout runs one layout pass of a card, failing the test on an error.
func mustLayout(t *testing.T, c card, img *image.RGBA, cur *layoutCursor, inset int) {
	t.Helper()
	if err := c.layout(img, cur, inset); err != nil {
		t.Fatalf("%s: layout failed: %v", c.name, err)
	}
}

// TestResultCardHeadingClearsTheCornerRow checks that a result card's heading
// never shares a row with the date and badge in its corners.
//
// The heading is centred, so a long one wraps to the full column width and
// reaches both corners. On a card tall enough for centredStart to clamp the
// content to the top margin, its first line would land on the corner row unless
// the row is reserved. A full Fraktion roster under a heading that wraps to
// several lines is that card.
func TestResultCardHeadingClearsTheCornerRow(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}

	v := testfixtures.ExtremeTitleFullRoster()[0]
	v.Subtitle = strings.Repeat("Änderungsantrag zu Artikel 12 Absatz 3 der Gemeindeordnung ", 4)
	if voteformat.FormatVoteDate(v.Date) == "" {
		t.Fatal("fixture has no date, so no corner row to clear")
	}
	bg := SelectColor(v.Jurisdiction, v.Affair.Number)

	// The second result card of a five-vote group, as GenerateCarousel builds it.
	c := carouselCards([]votes.Vote{v, v, v, v, v}, bg, fonts)[2]
	inset := c.inset(fonts)
	dry := newCursor(inset, imgHeight)
	mustLayout(t, c, nil, dry, inset)
	startY := centredStart(inset, dry.contentHeight())

	cornerBottom := bandInset(v) + padding + lineHeight(fonts.small)
	if startY < cornerBottom {
		t.Errorf("heading starts at y=%d, inside the corner row ending at y=%d", startY, cornerBottom)
	}

	real := newCursor(startY, imgHeight)
	mustLayout(t, c, newImage(bg), real, inset)
	if real.contentHeight() != dry.contentHeight() {
		t.Errorf("drawn card is %dpx tall against %dpx measured — the table dropped rows",
			real.contentHeight(), dry.contentHeight())
	}
}

// TestFitTitle_EllipsisesRatherThanOverflowing checks the last resort: a title
// too long to fit even at titleFontMin gives up its tail, not the results.
func TestFitTitle_EllipsisesRatherThanOverflowing(t *testing.T) {
	maxWidth := imgWidth - 2*padding

	short := "Postulat betreffend Anpassung der Mindestarealfläche"
	_, lines, err := fitTitle(short, maxWidth, 600)
	if err != nil {
		t.Fatalf("fitTitle failed: %v", err)
	}
	if got := strings.Join(lines, " "); got != short {
		t.Errorf("a title with room to spare was altered: got %q", got)
	}
	if strings.HasSuffix(strings.Join(lines, " "), "…") {
		t.Error("a title that fits must not be ellipsised")
	}

	long := strings.Repeat("Rahmenkredit für ein dreijähriges Pilotprojekt zur Schaffung einer Überbrückungshilfe ", 8)
	face, lines, err := fitTitle(long, maxWidth, 200)
	if err != nil {
		t.Fatalf("fitTitle failed: %v", err)
	}
	if h := titleBlockHeight(face, len(lines)); h > 200 {
		t.Errorf("title block is %dpx tall, over the 200px budget", h)
	}
	if !strings.HasSuffix(lines[len(lines)-1], "…") {
		t.Errorf("expected the cut title to end in an ellipsis, got %q", lines[len(lines)-1])
	}
	for i, line := range lines {
		if w := font.MeasureString(face, line).Ceil(); w > maxWidth {
			t.Errorf("line %d is %dpx wide, over the %dpx text column", i, w, maxWidth)
		}
	}
}

// TestWrapText_SplitsWordsWiderThanTheColumn checks the case that has no space
// to wrap at: a compound wider than the text column is broken across lines
// rather than drawn past both edges of it.
func TestWrapText_SplitsWordsWiderThanTheColumn(t *testing.T) {
	face, err := loadFace(goregular.TTF, titleFontMax)
	if err != nil {
		t.Fatalf("loadFace failed: %v", err)
	}
	maxWidth := imgWidth - 2*padding

	monster := strings.Repeat("Übertragungsverordnung", 6)
	text := "Beschluss betreffend " + monster
	lines := wrapText(face, text, maxWidth)

	for i, line := range lines {
		if w := font.MeasureString(face, line).Ceil(); w > maxWidth {
			t.Errorf("line %d is %dpx wide, over the %dpx text column: %q", i, w, maxWidth, line)
		}
	}
	// Splitting must not lose or invent characters: the wrapped lines carry the
	// same runes, and the only breaks added are line breaks.
	if got := strings.Join(lines, ""); strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(text, " ", "") {
		t.Errorf("wrapped text does not reconstruct the input:\n got %q\nwant %q", got, text)
	}

	// An ordinary title still wraps exactly as it did.
	plain := "Postulat betreffend Anpassung der Mindestarealfläche bei der Liegenschaftenverwaltung"
	for i, line := range wrapText(face, plain, maxWidth) {
		if w := font.MeasureString(face, line).Ceil(); w > maxWidth {
			t.Errorf("plain line %d is %dpx wide, over the %dpx column", i, w, maxWidth)
		}
	}
}

// TestFitTitle_KeepsOversizedTokensInsideTheColumn checks the width bound on
// the path that does not ellipsise: a title short enough to be accepted at the
// first face it tries, but carrying a token wider than the column.
func TestFitTitle_KeepsOversizedTokensInsideTheColumn(t *testing.T) {
	maxWidth := imgWidth - 2*padding
	title := "Beschluss betreffend " + strings.Repeat("Übertragungsverordnung", 6)

	face, lines, err := fitTitle(title, maxWidth, 600)
	if err != nil {
		t.Fatalf("fitTitle failed: %v", err)
	}
	if strings.HasSuffix(lines[len(lines)-1], "…") {
		t.Error("a title with room to spare must not be ellipsised")
	}
	for i, line := range lines {
		if w := font.MeasureString(face, line).Ceil(); w > maxWidth {
			t.Errorf("line %d is %dpx wide, over the %dpx text column: %q", i, w, maxWidth, line)
		}
	}
}

// TestFitTitle_ShrinksBeforeSplittingAWideWord checks which way out of an
// over-wide word fitTitle takes.
//
// Splitting keeps the word inside the column at any size, so a height-only fit
// test is satisfied at 42pt and the word is broken mid-compound. Most real
// cases have a cheaper escape: German administrative compounds overrun a 960px
// column at 42pt and fit whole a couple of steps down. Shrinking is the better
// trade, and the split stays for words no size can hold — which is what
// TestFitTitle_KeepsOversizedTokensInsideTheColumn covers.
func TestFitTitle_ShrinksBeforeSplittingAWideWord(t *testing.T) {
	maxWidth := imgWidth - 2*padding
	const long = "Grundstücksverkehrsgenehmigungszuständigkeitsübertragungsverordnung"

	biggest, err := loadFace(goregular.TTF, titleFontMax)
	if err != nil {
		t.Fatalf("loadFace failed: %v", err)
	}
	smallest, err := loadFace(goregular.TTF, titleFontMin)
	if err != nil {
		t.Fatalf("loadFace failed: %v", err)
	}
	// The premise: too wide at the top size, fine at the bottom one. Without
	// both halves the test would prove nothing about the choice being made.
	if w := font.MeasureString(biggest, long).Ceil(); w <= maxWidth {
		t.Fatalf("word is only %dpx at %.0fpt; it must overrun the %dpx column", w, titleFontMax, maxWidth)
	}
	if w := font.MeasureString(smallest, long).Ceil(); w > maxWidth {
		t.Fatalf("word is %dpx at %.0fpt; it must fit the %dpx column", w, titleFontMin, maxWidth)
	}

	face, lines, err := fitTitle("Postulat betreffend die "+long+" im Kanton", maxWidth, 600)
	if err != nil {
		t.Fatalf("fitTitle failed: %v", err)
	}
	for i, line := range lines {
		if w := font.MeasureString(face, line).Ceil(); w > maxWidth {
			t.Errorf("line %d is %dpx wide, over the %dpx text column: %q", i, w, maxWidth, line)
		}
	}
	var whole bool
	for _, line := range lines {
		if strings.Contains(line, long) {
			whole = true
		}
	}
	if !whole {
		t.Errorf("expected the compound set whole at a smaller size, got %q", lines)
	}
}

// TestFitTitle_BudgetTooSmallForOneLine checks the degenerate case: even a
// budget with no room for a single line yields one ellipsised line rather than
// an empty title.
func TestFitTitle_BudgetTooSmallForOneLine(t *testing.T) {
	long := strings.Repeat("Rahmenkredit für das dreijährige Pilotprojekt Überbrückungshilfe ", 4)
	_, lines, err := fitTitle(long, imgWidth-2*padding, 0)
	if err != nil {
		t.Fatalf("fitTitle failed: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected exactly one line, got %d: %q", len(lines), lines)
	}
	if !strings.HasSuffix(lines[0], "…") {
		t.Errorf("expected an ellipsis, got %q", lines[0])
	}
}

// TestAppendEllipsis_TrimsPunctuation checks the cut lands on a word, not on
// the comma that happened to follow it.
func TestAppendEllipsis_TrimsPunctuation(t *testing.T) {
	face, err := loadFace(goregular.TTF, titleFontMin)
	if err != nil {
		t.Fatalf("loadFace failed: %v", err)
	}
	if got := appendEllipsis(face, "gültigem Aufenthaltsstatus,", imgWidth-2*padding); got != "gültigem Aufenthaltsstatus…" {
		t.Errorf("got %q, want %q", got, "gültigem Aufenthaltsstatus…")
	}
}

// The card sets the type, the signatories and the subject in one column. Text
// posts name everyone; the card names as many as the font ladder can hold, and
// only then starts dropping — because the subject is what a reader cannot do
// without.
func TestCardTitleNamesEveryoneItCanFit(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()

	_, lines, err := cardTitle(group, voteformat.CleanVoteTitle(group[0].Title),
		imgWidth-2*padding, 900)
	if err != nil {
		t.Fatalf("cardTitle: %v", err)
	}
	set := strings.Join(lines, " ")

	for _, a := range group[0].Affair.Authors {
		if !strings.Contains(set, a.Name) {
			t.Errorf("%s is missing from a card with room for them:\n%s", a.Name, set)
		}
	}
	if strings.Contains(set, "u. a.") {
		t.Errorf("names were dropped from a card that had room:\n%s", set)
	}
	if strings.Contains(set, "…") {
		t.Errorf("the card cut its own title:\n%s", set)
	}
}

// Squeezed hard, the card gives up names rather than the subject — and says so
// with "u. a." rather than silently dropping people.
func TestCardTitleShedsNamesBeforeTheSubject(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()
	subject := voteformat.CleanVoteTitle(group[0].Title)

	// A budget below what the font ladder can absorb. The card holds all five
	// names down to about 120px, so this is a deliberately extreme squeeze —
	// which is the point: shedding is the last resort, not the usual path.
	_, lines, err := cardTitle(group, subject, imgWidth-2*padding, 100)
	if err != nil {
		t.Fatalf("cardTitle: %v", err)
	}
	set := strings.Join(lines, " ")

	named := 0
	for _, a := range group[0].Affair.Authors {
		if strings.Contains(set, a.Name) {
			named++
		}
	}
	if named == len(group[0].Affair.Authors) {
		t.Fatalf("nothing was shed, so this budget is not tight enough to test:\n%s", set)
	}
	if named == 0 {
		t.Errorf("every name was dropped; the first signatory should survive longest:\n%s", set)
	}
	if !strings.Contains(set, "u. a.") {
		t.Errorf("names were dropped without saying so:\n%s", set)
	}
	if !strings.Contains(set, "Hannah Pfalzgraf") {
		t.Errorf("the first signatory was shed before the co-signatories:\n%s", set)
	}
	// The subject survives the squeeze that cost the names.
	if strings.Contains(set, "…") {
		t.Errorf("the subject was cut even though names were available to shed:\n%s", set)
	}
}

// Business filed by the government has no signatories, and the card is exactly
// as it was before authorship existed.
func TestCardTitleWithoutAuthors(t *testing.T) {
	group := testfixtures.KantonsratVote()
	subject := voteformat.CleanVoteTitle(group[0].Title)

	_, lines, err := cardTitle(group, subject, imgWidth-2*padding, 900)
	if err != nil {
		t.Fatalf("cardTitle: %v", err)
	}
	if set := strings.Join(lines, " "); strings.Contains(set, " von ") {
		t.Errorf("a card with no authors grew a 'von' clause:\n%s", set)
	}
}
