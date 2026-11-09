package bluesky

import (
	"strconv"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// --- Helper Functions ---

func intPtr(i int) *int {
	return &i
}

// sampleVote returns a minimal Abstimmung for testing.
func sampleVote(title, result string, ja, nein, enth, abw int) votes.Vote {
	return votes.Vote{
		SourceID:   "vote-guid-1",
		Body:       "Gemeinderat ZH",
		Title:      title,
		SessionID:  "sitzung-guid-1",
		Date:       testfixtures.MustDate("2025-06-15"),
		Decision:   result,
		Yes:        intPtr(ja),
		No:         intPtr(nein),
		Abstention: intPtr(enth),
		Absent:     intPtr(abw),
		SourceURL:  testfixtures.VoteURL("vote-guid-1"),
		GroupURL:   testfixtures.TraktandumURL("sitzung-guid-1", "trakt-guid-1"),
		Affair:     votes.Affair{Number: "2025/100", Title: title, ID: "geschaeft-guid-1", URL: testfixtures.GeschaeftURL("geschaeft-guid-1")},
	}
}

// --- Tests ---

func TestFormatVoteThread_EmptyVotes(t *testing.T) {
	result := FormatVoteThread(nil, nil)
	if result != nil {
		t.Errorf("expected nil for empty group, got %d posts", len(result))
	}

	result = FormatVoteThread([]votes.Vote{}, nil)
	if result != nil {
		t.Errorf("expected nil for empty slice, got %d posts", len(result))
	}
}

func TestFormatVoteThread_SingleVote(t *testing.T) {
	group := []votes.Vote{
		sampleVote("Postulat von Reto Brüesch (SVP): Anpassung der Mindestfläche", "angenommen", 90, 30, 0, 5),
	}
	thread := FormatVoteThread(group, nil)

	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts (root + reply), got %d", len(thread))
	}

	root := thread[0]
	// Root must contain header, result, title, and thread hint
	for _, part := range []string{
		"🗳️ Gemeinderat ZH",
		"Abstimmung vom 15.06.2025",
		"✅",
		"Angenommen",
		"Anpassung der Mindestfläche",
		"👇 Details im Thread",
	} {
		if !strings.Contains(root.Text, part) {
			t.Errorf("root post missing %q\nFull root:\n%s", part, root.Text)
		}
	}

	if graphemeLen(root.Text) > maxGraphemes {
		t.Errorf("root post exceeds %d graphemes: %d\n%s", maxGraphemes, graphemeLen(root.Text), root.Text)
	}

	lastReply := thread[len(thread)-1]
	// Last reply must contain vote counts and link
	for _, part := range []string{
		"90 Ja",
		"30 Nein",
		"0 Enth.",
		"5 Abw.",
		"🔗",
	} {
		if !strings.Contains(lastReply.Text, part) {
			t.Errorf("reply missing %q\nFull reply:\n%s", part, lastReply.Text)
		}
	}

	for i, post := range thread {
		if graphemeLen(post.Text) > maxGraphemes {
			t.Errorf("post %d exceeds %d graphemes: %d\n%s", i, maxGraphemes, graphemeLen(post.Text), post.Text)
		}
	}
}

func TestFormatVoteThread_RejectedVote(t *testing.T) {
	group := []votes.Vote{
		sampleVote("Antrag: Festsetzung der Selnaustrasse", "abgelehnt", 20, 95, 5, 5),
	}
	thread := FormatVoteThread(group, nil)

	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(thread))
	}

	root := thread[0]
	if !strings.Contains(root.Text, "❌") {
		t.Errorf("rejected vote should have ❌ emoji\nFull root:\n%s", root.Text)
	}
	if !strings.Contains(root.Text, "Abgelehnt") {
		t.Errorf("rejected vote should say Abgelehnt\nFull root:\n%s", root.Text)
	}
}

func TestFormatVoteThread_VeryLongTitle(t *testing.T) {
	longTitle := "Schlussabstimmung über die bereinigten Dispositivziffern " +
		"zum Objektkredit von 350 Millionen Franken für das Projekt Erweiterung " +
		"und Neugestaltung des Hauptbahnhofs Zürich mit unterirdischer Durchmesserlinie " +
		"und ergänzenden Massnahmen zur Verbesserung der Verkehrsinfrastruktur im Grossraum Zürich " +
		"inklusive der notwendigen Anpassungen an die bestehende urbane Planung"

	group := []votes.Vote{
		sampleVote(longTitle, "angenommen", 80, 30, 5, 10),
	}
	thread := FormatVoteThread(group, nil)

	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(thread))
	}

	root := thread[0]
	if graphemeLen(root.Text) > maxGraphemes {
		t.Errorf("root exceeds %d graphemes with long title: %d\n%s", maxGraphemes, graphemeLen(root.Text), root.Text)
	}
	if !strings.Contains(root.Text, "…") {
		t.Errorf("expected truncation ellipsis in root for long title\nFull root:\n%s", root.Text)
	}
	if !strings.Contains(root.Text, "🗳️ Gemeinderat ZH") {
		t.Errorf("root missing header\n%s", root.Text)
	}
	if !strings.Contains(root.Text, "👇 Details im Thread") {
		t.Errorf("root missing thread hint\n%s", root.Text)
	}

	// First reply should go straight to vote counts (no title repetition)
	firstReply := thread[1]
	if !strings.Contains(firstReply.Text, "📊") {
		t.Errorf("first reply should contain vote counts, not a title repeat\nFull reply:\n%s", firstReply.Text)
	}
	if strings.Contains(firstReply.Text, "Schlussabstimmung über") {
		t.Errorf("first reply must not repeat the title\nFull reply:\n%s", firstReply.Text)
	}
}

// TestFormatVoteThread_WahlgeschaeftTruncation pins that truncateText's own
// appended "…" is budgeted for, not just the text before it. A budget that
// stops one grapheme short of maxGraphemes still overruns once truncateText
// adds the ellipsis on top — see the fixed buildWahlgeschaeftPosts.
func TestFormatVoteThread_WahlgeschaeftTruncation(t *testing.T) {
	longAmt := strings.Repeat("Mitglied Obergericht ", 30)
	group := []votes.Vote{
		{
			SourceID: "wahlgeschaeft-trunc-1",
			Title:    "Wahl " + longAmt + "für Roland Schmid",
			Type:     "Anwesenheitsermittlung",
			Date:     testfixtures.MustDate("2026-07-06"),
			Affair:   votes.Affair{Type: "Wahl"},
		},
	}

	thread := FormatVoteThread(group, nil)
	if len(thread) == 0 {
		t.Fatal("want a post, got none")
	}

	post := thread[0].Text
	if got := graphemeLen(post); got > maxGraphemes {
		t.Errorf("Wahlgeschäft post exceeds maxGraphemes: %d > %d", got, maxGraphemes)
	}
	if !strings.Contains(post, "…") {
		t.Error("truncated Wahlgeschäft post should contain '…'")
	}
}

// The truncation arithmetic reserves room for the business-type line, so a root
// that loses it is both missing the line and shorter than it was allowed to be.
func TestFormatVoteThread_TruncatedMultiVoteRootKeepsTheBusinessType(t *testing.T) {
	longTitle := strings.Repeat("A", maxGraphemes+200)

	var group []votes.Vote
	for i := 0; i < 2; i++ {
		v := sampleVote(longTitle, "angenommen", 80, 30, 5, 10)
		v.SourceID = "trunc-multi-" + strconv.Itoa(i)
		v.Affair.Type = "Vorlage"
		group = append(group, v)
	}

	root := FormatVoteThread(group, nil)[0]
	if !strings.Contains(root.Text, "Vorlage\n") {
		t.Errorf("truncated multi-vote root dropped the business type:\n%s", root.Text)
	}
	if graphemeLen(root.Text) > maxGraphemes {
		t.Errorf("root exceeds %d graphemes: %d\n%s", maxGraphemes, graphemeLen(root.Text), root.Text)
	}
}

func TestFormatVoteThread_MultipleVotes(t *testing.T) {
	group := []votes.Vote{
		{
			SourceID:   "vote-1",
			Title:      "Gesamtrevision der Gemeindeordnung",
			SessionID:  "sitzung-1",
			Subtitle:   "Einleitungsartikel",
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "angenommen",
			Yes:        intPtr(90),
			No:         intPtr(20),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("vote-1"),
			GroupURL:   testfixtures.TraktandumURL("sitzung-1", "trakt-1"),
			Affair:     votes.Affair{Number: "2025/100", Title: "Gesamtrevision der Gemeindeordnung", ID: "geschaeft-1", URL: testfixtures.GeschaeftURL("geschaeft-1")},
		},
		{
			SourceID:   "vote-2",
			Title:      "Gesamtrevision der Gemeindeordnung",
			SessionID:  "sitzung-1",
			Subtitle:   "Schlussabstimmung",
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "abgelehnt",
			Yes:        intPtr(40),
			No:         intPtr(70),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("vote-2"),
			GroupURL:   testfixtures.TraktandumURL("sitzung-1", "trakt-1"),
			Affair:     votes.Affair{Number: "2025/100", Title: "Gesamtrevision der Gemeindeordnung", ID: "geschaeft-1", URL: testfixtures.GeschaeftURL("geschaeft-1")},
		},
	}

	thread := FormatVoteThread(group, nil)
	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(thread))
	}

	root := thread[0]
	if !strings.Contains(root.Text, "Gesamtrevision der Gemeindeordnung") {
		t.Errorf("root missing title\n%s", root.Text)
	}
	if !strings.Contains(root.Text, "👇 Details im Thread") {
		t.Errorf("root missing thread hint\n%s", root.Text)
	}

	allReplies := ""
	for _, post := range thread[1:] {
		allReplies += post.Text + "\n"
	}

	for _, part := range []string{
		"Einleitungsartikel",
		"Schlussabstimmung",
		"90 Ja",
		"40 Ja",
		"70 Nein",
		"🔗",
	} {
		if !strings.Contains(allReplies, part) {
			t.Errorf("replies missing %q\nAll replies:\n%s", part, allReplies)
		}
	}

	for i, post := range thread {
		if graphemeLen(post.Text) > maxGraphemes {
			t.Errorf("post %d exceeds %d graphemes: %d\n%s", i, maxGraphemes, graphemeLen(post.Text), post.Text)
		}
	}
}

func TestFormatVoteThread_LinkFacetOnFirstReply(t *testing.T) {
	group := []votes.Vote{
		sampleVote("Budget 2026", "angenommen", 100, 15, 5, 5),
	}
	thread := FormatVoteThread(group, nil)
	firstReply := thread[1]
	if len(firstReply.Facets) == 0 {
		t.Errorf("first reply should have a link facet\nText: %s", firstReply.Text)
	}
}

func TestFormatVoteThread_GenericAntragUsesGeschaeftTitle(t *testing.T) {
	group := []votes.Vote{
		{
			SourceID:   "vote-1",
			Title:      "Postulat von Max Müller (FDP): Bessere Veloinfrastruktur",
			SessionID:  "sitzung-1",
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(35),
			Abstention: intPtr(5),
			Absent:     intPtr(5),
			SourceURL:  testfixtures.GeschaeftURL("geschaeft-1"),
			GroupURL:   testfixtures.GeschaeftURL("geschaeft-1"),
			Affair:     votes.Affair{Number: "2025/200", Title: "Postulat von Max Müller (FDP): Bessere Veloinfrastruktur", ID: "geschaeft-1", URL: testfixtures.GeschaeftURL("geschaeft-1")},
		},
	}

	thread := FormatVoteThread(group, nil)
	root := thread[0]
	if !strings.Contains(root.Text, "Bessere Veloinfrastruktur") {
		t.Errorf("expected GeschaeftTitel to be used for generic Antrag title\nFull root:\n%s", root.Text)
	}
}

func TestFormatVoteThread_AllPostsWithinLimit(t *testing.T) {
	var group []votes.Vote
	// Stress test: many group to force multiple reply posts
	for i := 0; i < 10; i++ {
		group = append(group, votes.Vote{
			SourceID:   "vote-guid-" + string(rune('a'+i)),
			Title:      "Gesamtrevision der Gemeindeordnung",
			SessionID:  "sitzung-1",
			Subtitle:   "Ziffer " + string(rune('A'+i)),
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80 + i),
			No:         intPtr(30 - i),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("vote-guid-" + string(rune('a'+i))),
			GroupURL:   testfixtures.TraktandumURL("sitzung-1", "trakt-1"),
			Affair:     votes.Affair{Number: "2025/100", Title: "Gesamtrevision der Gemeindeordnung", ID: "geschaeft-1", URL: testfixtures.GeschaeftURL("geschaeft-1")},
		})
	}

	thread := FormatVoteThread(group, nil)
	if len(thread) < 3 {
		t.Errorf("expected at least 3 posts for 10 group, got %d", len(thread))
	}

	for i, post := range thread {
		gl := graphemeLen(post.Text)
		if gl > maxGraphemes {
			t.Errorf("post %d exceeds %d graphemes: %d\n%s", i, maxGraphemes, gl, post.Text)
		}
	}

	if !strings.Contains(thread[1].Text, "🔗") {
		t.Errorf("first reply missing link\n%s", thread[1].Text)
	}
}

func TestFormatVoteThread_WithMentions(t *testing.T) {
	// Load real contacts for mention matching
	mapper := mustLoadTestMapper(t)

	group := []votes.Vote{
		{
			SourceID:   "mention-test",
			Title:      "Postulat von Anna Graff (SP): Bessere Sicherheit",
			SessionID:  "sitzung-1",
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(30),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("mention-test"),
			GroupURL:   testfixtures.TraktandumURL("sitzung-1", "trakt-1"),
			Affair:     votes.Affair{Number: "2025/100", Title: "Bessere Sicherheit", ID: "geschaeft-1", URL: testfixtures.GeschaeftURL("geschaeft-1")},
		},
	}

	thread := FormatVoteThread(group, mapper)
	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(thread))
	}

	root := thread[0]

	// Root text should still contain the name (NOT replaced with @handle)
	if !strings.Contains(root.Text, "Anna Graff") {
		t.Errorf("root should contain politician name\nFull root:\n%s", root.Text)
	}

	// Root should have mention(s) with correct handle
	if len(root.Mentions) == 0 {
		t.Fatalf("root should have mentions, got 0")
	}

	found := false
	for _, m := range root.Mentions {
		if m.Handle == "annagraff.bsky.social" {
			found = true
			// Verify byte offsets point to "Anna Graff" in the text
			extracted := root.Text[m.ByteStart:m.ByteEnd]
			if extracted != "Anna Graff" {
				t.Errorf("mention byte range extracts %q, want %q", extracted, "Anna Graff")
			}
		}
	}
	if !found {
		t.Errorf("expected mention for annagraff.bsky.social, got: %v", root.Mentions)
	}
}

func TestFormatVoteThread_NilMapper(t *testing.T) {
	group := []votes.Vote{
		sampleVote("Postulat von Anna Graff (SP): Test", "angenommen", 80, 30, 5, 10),
	}
	thread := FormatVoteThread(group, nil)

	root := thread[0]
	if len(root.Mentions) != 0 {
		t.Errorf("nil mapper should produce no mentions, got %d", len(root.Mentions))
	}
}

// mustLoadTestMapper creates a contacts mapper with a few test contacts
func mustLoadTestMapper(t *testing.T) *contacts.Mapper {
	t.Helper()
	// Use the real contacts file if available, otherwise build a minimal one
	mapper, err := contacts.LoadContacts("../../../../data/zurich-city/contacts.yaml")
	if err != nil {
		t.Skipf("contacts.yaml not available: %v", err)
	}
	return mapper
}

// allThreadText joins all post texts in a thread for simple Contains assertions.
func allThreadText(thread []*BlueskyPost) string {
	var parts []string
	for _, p := range thread {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\n\n")
}

func TestFormatVoteThread_SingleVoteWithFraktion(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	thread := FormatVoteThread(group, nil)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	if len(thread) < 2 {
		t.Fatalf("expected at least 2 posts (root + replies), got %d", len(thread))
	}

	// Fraktion breakdown must appear somewhere in the thread
	if !strings.Contains(full, "🏛️ Fraktionen") {
		t.Error("thread should contain Fraktion breakdown header")
	}

	// All 7 factions must be present
	for _, faction := range []string{"SP", "SVP", "FDP", "GLP", "AL", "Die Mitte", "Grüne"} {
		if !strings.Contains(full, faction) {
			t.Errorf("thread should contain faction %q", faction)
		}
	}

	// Header should be Ja/Nein format
	if !strings.Contains(full, "(Ja/Nein/Enth/Abw)") {
		t.Error("header should be (Ja/Nein/Enth/Abw)")
	}

	// Verify each post is within grapheme limit
	for i, post := range thread {
		if graphemeLen(post.Text) > maxGraphemes {
			t.Errorf("post %d exceeds %d graphemes: %d\n%s", i, maxGraphemes, graphemeLen(post.Text), post.Text)
		}
	}
}

func TestFormatVoteThread_MultiVoteWithFraktion(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	thread := FormatVoteThread(group, nil)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	// Each of the 2 group should have its own Fraktion entry
	count := strings.Count(full, "🏛️ Fraktionen")
	if count != 2 {
		t.Errorf("expected 2 Fraktion breakdown entries, got %d", count)
	}

	// Verify each post is within grapheme limit
	for i, post := range thread {
		if graphemeLen(post.Text) > maxGraphemes {
			t.Errorf("post %d exceeds %d graphemes: %d\n%s", i, maxGraphemes, graphemeLen(post.Text), post.Text)
		}
	}
}

func TestFormatVoteThread_NoStimmabgaben(t *testing.T) {
	// Use a vote with no Stimmabgaben to test the no-Fraktion path
	group := []votes.Vote{
		{
			SourceID:   "objguid-nostimm",
			Title:      "Test ohne Stimmabgaben",
			SessionID:  "sitzung-nostimm",
			Date:       testfixtures.MustDate("2025-06-15"),
			Decision:   "abgelehnt",
			Yes:        intPtr(20),
			No:         intPtr(95),
			Abstention: intPtr(5),
			Absent:     intPtr(5),
			SourceURL:  testfixtures.VoteURL("objguid-nostimm"),
			GroupURL:   testfixtures.TraktandumURL("sitzung-nostimm", "trakt-nostimm"),
			Affair:     votes.Affair{Number: "2025/999", Title: "Test ohne Stimmabgaben", ID: "geschaeft-nostimm", URL: testfixtures.GeschaeftURL("geschaeft-nostimm")},
		},
	}
	thread := FormatVoteThread(group, nil)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	if strings.Contains(full, "🏛️ Fraktionen") {
		t.Error("thread should NOT contain Fraktion breakdown when Stimmabgaben is empty")
	}
}

func TestFormatVoteThread_AuswahlWithFraktion(t *testing.T) {
	group := testfixtures.AuswahlVote()
	thread := FormatVoteThread(group, nil)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	if !strings.Contains(full, "🏛️ Fraktionen") {
		t.Error("thread should contain Fraktion breakdown")
	}

	// Auswahl vote should have A/B/C/Abw header, NOT Ja/Nein
	if !strings.Contains(full, "(A/B/C/Abw)") {
		t.Error("Auswahl vote header should be (A/B/C/Abw)")
	}
	if strings.Contains(full, "(Ja/Nein/Enth/Abw)") {
		t.Error("Auswahl vote header should NOT be (Ja/Nein/Enth/Abw)")
	}
}

func TestFormatVoteThread_SingleVoteSubtitlePrefix(t *testing.T) {
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
			name:             "Multi-vote does NOT prepend subtitle to root",
			group:            testfixtures.MultiVoteGroup(),
			shouldContain:    []string{},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := FormatVoteThread(tt.group, nil)
			root := thread[0].Text
			t.Logf("Root post:\n%s", root)

			for _, s := range tt.shouldContain {
				if !strings.Contains(root, s) {
					t.Errorf("Expected root to contain %q", s)
				}
			}
			for _, s := range tt.shouldNotContain {
				if strings.Contains(root, s) {
					t.Errorf("Expected root NOT to contain %q", s)
				}
			}
		})
	}
}

// Bluesky's root is tighter than X's, so it sheds sooner — but never the title.
func TestFormatVoteThread_LongSignatoryListNeverTruncatesTheTitle(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()

	thread := FormatVoteThread(group, nil)
	if len(thread) < 2 {
		t.Fatalf("got %d posts, want a root and at least one reply", len(thread))
	}

	if !strings.Contains(thread[0].Text, group[0].Title) {
		t.Errorf("the title was cut to make room for names:\n%s", thread[0].Text)
	}
	if graphemeLen(thread[0].Text) > maxGraphemes {
		t.Errorf("root is %d graphemes, over the %d limit", graphemeLen(thread[0].Text), maxGraphemes)
	}

	var all strings.Builder
	for _, p := range thread {
		all.WriteString(p.Text)
	}
	for _, a := range group[0].Affair.Authors {
		if !strings.Contains(all.String(), a.Name) {
			t.Errorf("%s is named nowhere in the thread", a.Name)
		}
	}
}

// TestFormatVoteThread_EveryLinkIsFaceted guards the failure mode that has no
// visible symptom on our side: Bluesky renders a URL as plain text unless a
// facet covers it, so a link block that grew a third entry while the faceting
// stayed on one would publish two URLs a reader has to copy by hand.
//
// The links may be spread over several posts (see linkChunks), so this counts
// facets across the whole thread rather than on the last reply.
func TestFormatVoteThread_EveryLinkIsFaceted(t *testing.T) {
	for name, group := range map[string][]votes.Vote{
		"single vote": testfixtures.KantonsratVote(),
		"group":       testfixtures.KantonsratMultiVote(),
	} {
		t.Run(name, func(t *testing.T) {
			links := voteformat.GroupLinks(group)
			if len(links) != 3 {
				t.Fatalf("fixture carries %d links, want the cantonal three", len(links))
			}

			faceted := map[string]int{}
			for _, post := range FormatVoteThread(group, nil) {
				for _, f := range post.Facets {
					covered := post.Text[f.Index.ByteStart:f.Index.ByteEnd]
					for _, feat := range f.Features {
						if feat.URI != covered {
							t.Errorf("facet covers %q but points at %q", covered, feat.URI)
						}
					}
					faceted[covered]++
				}
			}

			for _, l := range links {
				if faceted[l.URL] != 1 {
					t.Errorf("%q faceted %d times, want exactly once", l.URL, faceted[l.URL])
				}
			}
		})
	}
}

// TestFormatVoteThread_CantonPostsWithinLimit covers what the existing stress
// test does not: the cantonal link block is around 380 graphemes on Bluesky,
// which no single post can hold. An over-length post is rejected by the API
// mid-thread, leaving a published root with no results under it.
func TestFormatVoteThread_CantonPostsWithinLimit(t *testing.T) {
	for name, group := range testfixtures.AllFixtures() {
		if len(group) == 0 || group[0].Jurisdiction != "zurich-canton" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			for i, post := range FormatVoteThread(group, nil) {
				if gl := graphemeLen(post.Text); gl > maxGraphemes {
					t.Errorf("post %d is %d graphemes, over the %d limit:\n%s",
						i, gl, maxGraphemes, post.Text)
				}
			}
		})
	}
}

// TestFormatVoteThread_NoDuplicateFacetOffsets pins the faceting half of the
// duplicate-link problem: buildLinkFacets locates a URL with strings.Index, so
// two identical URLs in one block would both facet the first occurrence,
// leaving overlapping ranges and the second copy unlinked. voteformat drops the
// duplicate; this fails if it ever stops.
func TestFormatVoteThread_NoDuplicateFacetOffsets(t *testing.T) {
	const segment = "https://zh.recapp.ch/shareparl?agendaItemUid=i&segmentUid=s"
	group := []votes.Vote{{
		SourceID: "no-affair", Title: "Mitteilungen", Body: "Kantonsrat ZH",
		Date: testfixtures.MustDate("2026-08-31"),
		Yes:  intPtr(100), No: intPtr(50), Abstention: intPtr(0), Absent: intPtr(30),
		SourceURL: segment, GroupURL: segment, VideoURL: segment,
		SessionURL: "https://www.kantonsrat.zh.ch/ratsbetrieb/sitzungenundprotokolle/?d=1",
	}}

	seen := map[int]bool{}
	for _, post := range FormatVoteThread(group, nil) {
		for _, f := range post.Facets {
			if seen[f.Index.ByteStart] {
				t.Errorf("two facets start at byte %d\n%s", f.Index.ByteStart, post.Text)
			}
			seen[f.Index.ByteStart] = true
		}
	}
}

// The links lead the thread, on a reply of their own in a multi-vote group so
// that no vote's reply — which carries that vote's card — also carries them.
func TestFormatVoteThread_LinksLeadTheThread(t *testing.T) {
	group := testfixtures.TenVoteStressTest()
	thread := FormatVoteThread(group, nil)

	if strings.Contains(thread[0].Text, "https://") {
		t.Errorf("root carries a link:\n%s", thread[0].Text)
	}
	if !strings.Contains(thread[1].Text, "🔗") {
		t.Errorf("first reply should carry the links:\n%s", thread[1].Text)
	}
	for i, post := range thread[1:] {
		hasLink := strings.Contains(post.Text, "https://")
		hasVote := strings.Contains(post.Text, "📊")
		if hasLink && hasVote {
			t.Errorf("reply %d mixes the links with a vote:\n%s", i+1, post.Text)
		}
	}
	if strings.Contains(thread[len(thread)-1].Text, "https://") {
		t.Errorf("the last reply still carries a link:\n%s", thread[len(thread)-1].Text)
	}
}
