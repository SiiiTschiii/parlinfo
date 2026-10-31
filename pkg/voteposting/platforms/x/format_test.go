package x

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// allThreadText joins all post texts in a thread for simple Contains assertions.
func allThreadText(thread []*XPost) string {
	var parts []string
	for _, p := range thread {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\n\n")
}

func TestFormatVoteThread_PreservesPostulatMotion(t *testing.T) {
	tests := []struct {
		name          string
		group         []votes.Vote
		expectedParts []string
	}{
		{
			name: "Single vote with Postulat in title",
			group: []votes.Vote{
				{
					SourceID:   "test-guid-1",
					Title:      "2025/100 Postulat von Reto Brüesch (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche",
					Date:       testfixtures.MustDate("2025-11-26"),
					Decision:   "abgelehnt",
					Yes:        intPtr(21),
					No:         intPtr(38),
					Abstention: intPtr(56),
					Absent:     intPtr(10),
					SourceURL:  testfixtures.VoteURL("test-guid-1"),
					Affair:     votes.Affair{Number: "2025/100"},
				},
			},
			expectedParts: []string{
				"Postulat",
				"von Reto Brüesch (SVP)",
				"Anpassung der Mindest- und Höchstarealfläche",
			},
		},
		{
			name: "Single vote with Motion in title",
			group: []votes.Vote{
				{
					SourceID:   "test-guid-2",
					Title:      "2025/200 Motion von Liv Mahrer (SP) vom 05.02.2025: Festsetzung der Selnaustrasse",
					Date:       testfixtures.MustDate("2025-11-26"),
					Decision:   "angenommen",
					Yes:        intPtr(90),
					No:         intPtr(30),
					Abstention: intPtr(0),
					Absent:     intPtr(5),
					SourceURL:  testfixtures.VoteURL("test-guid-2"),
					Affair:     votes.Affair{Number: "2025/200"},
				},
			},
			expectedParts: []string{
				"Motion",
				"von Liv Mahrer (SP)",
				"Festsetzung der Selnaustrasse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := FormatVoteThread(tt.group, nil, DefaultMaxChars)
			full := allThreadText(thread)
			t.Logf("Full output:\n%s", full)

			for _, part := range tt.expectedParts {
				if !strings.Contains(full, part) {
					t.Errorf("Expected thread to contain %q, but it didn't.\nFull output:\n%s", part, full)
				}
			}
		})
	}
}

func TestFormatVoteThread_AuswahlVote(t *testing.T) {
	tests := []struct {
		name             string
		group            []votes.Vote
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "Single Auswahl vote — no result prefix",
			group: []votes.Vote{
				{
					SourceID:  "auswahl-guid-1",
					Title:     "Weisung: Jugendwohnkredit 2025",
					Date:      testfixtures.MustDate("2026-03-04"),
					Decision:  "Auswahl A",
					Absent:    intPtr(10),
					ChoiceA:   intPtr(74),
					ChoiceB:   intPtr(28),
					ChoiceC:   intPtr(13),
					SourceURL: testfixtures.VoteURL("auswahl-guid-1"),
				},
			},
			shouldContain:    []string{"📊 A: 74 | B: 28 | C: 13 | Abwesend 10", "Jugendwohnkredit"},
			shouldNotContain: []string{"✅", "❌", "Angenommen", "Abgelehnt"},
		},
		{
			name: "Multi vote with Auswahl entry — no emoji before subtitle",
			group: []votes.Vote{
				{
					SourceID:   "guid-ja-nein",
					Title:      "Weisung: BZO",
					Subtitle:   "Änderungsantrag 9",
					Date:       testfixtures.MustDate("2026-02-25"),
					Decision:   "angenommen",
					Yes:        intPtr(62),
					No:         intPtr(51),
					Abstention: intPtr(0),
					Absent:     intPtr(12),
					SourceURL:  testfixtures.VoteURL("guid-ja-nein"),
				},
				{
					SourceID:  "guid-auswahl",
					Title:     "Weisung: BZO",
					Subtitle:  "Änderungsantrag 17, 1. Abstimmung",
					Date:      testfixtures.MustDate("2026-02-25"),
					Decision:  "Auswahl A",
					Absent:    intPtr(11),
					ChoiceA:   intPtr(50),
					ChoiceB:   intPtr(24),
					ChoiceC:   intPtr(40),
					SourceURL: testfixtures.VoteURL("guid-auswahl"),
				},
			},
			shouldContain: []string{
				"✅ Änderungsantrag 9",
				"Änderungsantrag 17, 1. Abstimmung",
				"📊 A: 50 | B: 24 | C: 40 | Abwesend 11",
			},
			shouldNotContain: []string{"❌ Änderungsantrag 17", "✅ Änderungsantrag 17"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread := FormatVoteThread(tt.group, nil, DefaultMaxChars)
			full := allThreadText(thread)
			t.Logf("Full output:\n%s", full)
			for _, part := range tt.shouldContain {
				if !strings.Contains(full, part) {
					t.Errorf("Expected thread to contain %q", part)
				}
			}
			for _, part := range tt.shouldNotContain {
				if strings.Contains(full, part) {
					t.Errorf("Expected thread NOT to contain %q", part)
				}
			}
		})
	}
}

func TestFormatVoteThread_SingleVoteStructure(t *testing.T) {
	group := []votes.Vote{
		{
			SourceID:   "struct-guid-1",
			Body:       "Gemeinderat ZH",
			Title:      "Weisung: Testvorlage",
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(30),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("struct-guid-1"),
		},
	}

	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	if len(thread) < 2 {
		t.Fatalf("expected root + at least 1 reply, got %d posts", len(thread))
	}

	// Root contains header, title, thread hint
	root := thread[0].Text
	if !strings.Contains(root, "Gemeinderat ZH") {
		t.Error("root should contain header")
	}
	if !strings.Contains(root, "Testvorlage") {
		t.Error("root should contain title")
	}
	if !strings.Contains(root, "👇 Details im Thread") {
		t.Error("root should contain thread hint")
	}

	// Last reply contains link
	if !strings.Contains(thread[1].Text, "🔗") {
		t.Error("first reply should contain link")
	}
}

func TestFormatVoteThread_MultiVoteStructure(t *testing.T) {
	group := []votes.Vote{
		{
			SourceID:   "multi-guid-1",
			Title:      "Weisung: Grosses Projekt",
			Subtitle:   "Antrag 1",
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "angenommen",
			Yes:        intPtr(60),
			No:         intPtr(40),
			Abstention: intPtr(10),
			Absent:     intPtr(15),
			SourceURL:  testfixtures.VoteURL("multi-guid-1"),
		},
		{
			SourceID:   "multi-guid-2",
			Title:      "Weisung: Grosses Projekt",
			Subtitle:   "Antrag 2",
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "abgelehnt",
			Yes:        intPtr(30),
			No:         intPtr(70),
			Abstention: intPtr(5),
			Absent:     intPtr(20),
			SourceURL:  testfixtures.VoteURL("multi-guid-2"),
		},
	}

	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	if len(thread) < 2 {
		t.Fatalf("expected root + at least 1 reply, got %d posts", len(thread))
	}

	root := thread[0].Text
	if !strings.Contains(root, "Grosses Projekt") {
		t.Error("root should contain title")
	}

	full := allThreadText(thread)
	if !strings.Contains(full, "Antrag 1") {
		t.Error("thread should contain first vote subtitle")
	}
	if !strings.Contains(full, "Antrag 2") {
		t.Error("thread should contain second vote subtitle")
	}

	if !strings.Contains(thread[1].Text, "🔗") {
		t.Error("first reply should contain link")
	}
	for i, post := range thread[1:] {
		if strings.Contains(post.Text, "https://") && strings.Contains(post.Text, "📊") {
			t.Errorf("reply %d mixes the links with a vote:\n%s", i+1, post.Text)
		}
	}
}

func TestFormatVoteThread_LinkPlacement(t *testing.T) {
	group := []votes.Vote{
		{
			SourceID:   "link-guid-1",
			Title:      "Weisung: Linktest",
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(30),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("link-guid-1"),
		},
	}

	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	// Link must NOT be in root
	if strings.Contains(thread[0].Text, "🔗") {
		t.Error("root should not contain link — link belongs in replies")
	}

	// Link must lead the replies. A lone vote's counts may share that reply,
	// since it carries no card.
	if !strings.HasPrefix(thread[1].Text, "🔗") {
		t.Errorf("first reply should open with the link, got: %s", thread[1].Text)
	}
}

func TestFormatVoteThread_RootTruncation(t *testing.T) {
	// Create a vote with a very long title that exceeds DefaultMaxChars
	longTitle := strings.Repeat("A", DefaultMaxChars+500)
	group := []votes.Vote{
		{
			SourceID:   "trunc-guid-1",
			Title:      longTitle,
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(30),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL("trunc-guid-1"),
		},
	}

	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	root := thread[0].Text
	// Weighted length, not bytes: bytes are not what X enforces, and charging
	// them made every umlaut and emoji shrink the usable post.
	if got := weightedLen(root); got > DefaultMaxChars {
		t.Errorf("root post exceeds DefaultMaxChars: %d > %d", got, DefaultMaxChars)
	}
	if !strings.Contains(root, "…") {
		t.Error("truncated root should contain '…'")
	}
}

// TestFormatVoteThread_WahlgeschaeftTruncation pins that truncateText's own
// appended "…" is budgeted for, not just the text before it. A budget that
// stops one weighted character short of charLimit still overruns once
// truncateText adds the ellipsis on top — see the fixed
// buildWahlgeschaeftPost.
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

	thread := FormatVoteThread(group, nil, DefaultMaxChars)
	if len(thread) != 1 {
		t.Fatalf("a Wahlgeschäft is one post, got %d", len(thread))
	}

	post := thread[0].Text
	if got := weightedLen(post); got > DefaultMaxChars {
		t.Errorf("Wahlgeschäft post exceeds DefaultMaxChars: %d > %d", got, DefaultMaxChars)
	}
	if !strings.Contains(post, "…") {
		t.Error("truncated Wahlgeschäft post should contain '…'")
	}
}

// The truncation arithmetic reserves room for the business-type line, so a root
// that loses it is both missing the line and shorter than it was allowed to be.
func TestFormatVoteThread_TruncatedMultiVoteRootKeepsTheBusinessType(t *testing.T) {
	longTitle := strings.Repeat("A", DefaultMaxChars+500)
	var group []votes.Vote
	for i := 0; i < 2; i++ {
		group = append(group, votes.Vote{
			SourceID:   fmt.Sprintf("trunc-multi-%d", i),
			Title:      longTitle,
			Date:       testfixtures.MustDate("2026-01-15"),
			Decision:   "angenommen",
			Yes:        intPtr(80),
			No:         intPtr(30),
			Abstention: intPtr(5),
			Absent:     intPtr(10),
			SourceURL:  testfixtures.VoteURL(fmt.Sprintf("trunc-multi-%d", i)),
			Affair:     votes.Affair{Type: "Vorlage"},
		})
	}

	root := FormatVoteThread(group, nil, DefaultMaxChars)[0].Text
	if !strings.Contains(root, "Vorlage\n") {
		t.Errorf("truncated multi-vote root dropped the business type:\n%s", root)
	}
	if got := weightedLen(root); got > DefaultMaxChars {
		t.Errorf("root post exceeds DefaultMaxChars: %d > %d", got, DefaultMaxChars)
	}
}

func TestFormatVoteThread_EmptyVotes(t *testing.T) {
	thread := FormatVoteThread(nil, nil, DefaultMaxChars)
	if thread != nil {
		t.Errorf("expected nil for empty group, got %d posts", len(thread))
	}
}

// Helper function for tests
func intPtr(i int) *int {
	return &i
}

func TestFormatVoteThread_SingleVoteWithFraktion(t *testing.T) {
	group := testfixtures.SingleVoteAngenommen()
	thread := FormatVoteThread(group, nil, DefaultMaxChars)

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

	// Verify each post is within char limit, by X's counting rather than bytes
	for i, post := range thread {
		if got := weightedLen(post.Text); got > DefaultMaxChars {
			t.Errorf("post %d exceeds %d chars: %d\n%s", i, DefaultMaxChars, got, post.Text)
		}
	}
}

func TestFormatVoteThread_MultiVoteWithFraktion(t *testing.T) {
	group := testfixtures.MultiVoteGroup()
	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	// Each of the 2 group should have its own Fraktion entry
	count := strings.Count(full, "🏛️ Fraktionen")
	if count != 2 {
		t.Errorf("expected 2 Fraktion breakdown entries, got %d", count)
	}

	// Verify each post is within char limit, by X's counting rather than bytes
	for i, post := range thread {
		if got := weightedLen(post.Text); got > DefaultMaxChars {
			t.Errorf("post %d exceeds %d chars: %d\n%s", i, DefaultMaxChars, got, post.Text)
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
	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	full := allThreadText(thread)
	t.Logf("Full thread (%d posts):\n%s", len(thread), full)

	if strings.Contains(full, "🏛️ Fraktionen") {
		t.Error("thread should NOT contain Fraktion breakdown when Stimmabgaben is empty")
	}
}

func TestFormatVoteThread_AuswahlWithFraktion(t *testing.T) {
	group := testfixtures.AuswahlVote()
	thread := FormatVoteThread(group, nil, DefaultMaxChars)

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
			thread := FormatVoteThread(tt.group, nil, DefaultMaxChars)
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

// A body whose titles carry no names — Kanton Zürich's never do — is named only
// on the label line, so tagging the title alone would tag nobody.
func TestFormatVoteThread_TagsAuthorsOnTheLabelLine(t *testing.T) {
	group := testfixtures.KantonsratMemberBusiness()

	contactsFile := filepath.Join(t.TempDir(), "contacts.yaml")
	err := os.WriteFile(contactsFile, []byte(`version: "1.0"
contacts:
  - name: Tobias Weidmann
    x:
      - url: https://x.com/tobiasweidmann
        verified: true
`), 0o600)
	if err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	mapper, err := contacts.LoadContacts(contactsFile)
	if err != nil {
		t.Fatalf("load contacts: %v", err)
	}

	thread := FormatVoteThread(group, mapper, DefaultMaxChars)
	if len(thread) == 0 {
		t.Fatal("no posts")
	}

	if !strings.Contains(thread[0].Text, "Postulat von Tobias Weidmann @tobiasweidmann (SVP)") {
		t.Errorf("the author on the label line was not tagged\n%s", thread[0].Text)
	}
}

// The title is what a reader cannot do without, so a long signatory list must
// never be the reason it gets cut. The names that do not fit move into the
// thread instead.
func TestFormatVoteThread_LongSignatoryListNeverTruncatesTheTitle(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()
	title := group[0].Title

	thread := FormatVoteThread(group, nil, DefaultMaxChars)
	if len(thread) < 2 {
		t.Fatalf("got %d posts, want a root and at least one reply", len(thread))
	}

	if !strings.Contains(thread[0].Text, title) {
		t.Errorf("the title was cut to make room for names:\n%s", thread[0].Text)
	}
	if strings.Contains(thread[0].Text, "…") {
		t.Errorf("root carries an ellipsis, so something was truncated:\n%s", thread[0].Text)
	}
	if weightedLen(thread[0].Text) > DefaultMaxChars {
		t.Errorf("root is %d chars, over the %d limit", weightedLen(thread[0].Text), DefaultMaxChars)
	}

	// Everyone is still named across the thread.
	all := allThreadText(thread)
	for _, a := range group[0].Affair.Authors {
		if !strings.Contains(all, a.Name) {
			t.Errorf("%s is named nowhere in the thread", a.Name)
		}
	}
	if !strings.Contains(all, "Weitere Unterzeichnende") {
		t.Errorf("the shed signatories got no block of their own:\n%s", all)
	}
}

// With room to spare every signatory is named in the root, and no separate
// block is needed at all.
func TestFormatVoteThread_PremiumRootNamesEverySignatory(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()

	thread := FormatVoteThread(group, nil, 2000)

	for _, a := range group[0].Affair.Authors {
		if !strings.Contains(thread[0].Text, a.Name) {
			t.Errorf("%s is missing from a root with room for them:\n%s", a.Name, thread[0].Text)
		}
	}
	if strings.Contains(allThreadText(thread), "Weitere Unterzeichnende") {
		t.Error("a signatory block was added even though the root held everyone")
	}
}

// A single signatory is the ordinary case and must read as plain prose, with no
// "u. a." and no extra block.
func TestFormatVoteThread_SingleSignatory(t *testing.T) {
	group := testfixtures.KantonsratMemberBusiness()

	thread := FormatVoteThread(group, nil, DefaultMaxChars)

	if !strings.Contains(thread[0].Text, "Postulat von Tobias Weidmann (SVP)") {
		t.Errorf("want the lone signatory named plainly:\n%s", thread[0].Text)
	}
	all := allThreadText(thread)
	if strings.Contains(all, "u. a.") || strings.Contains(all, "Weitere Unterzeichnende") {
		t.Errorf("one signatory should need no overflow machinery:\n%s", all)
	}
}

// A member shed from the root for length has done nothing to deserve losing
// their tag. X finds no mentions on its own, so the signatory block has to be
// tagged like the root's own line.
func TestFormatVoteThread_TagsDeferredSignatories(t *testing.T) {
	group := testfixtures.KantonsratCoSignedBusiness()

	contactsFile := filepath.Join(t.TempDir(), "contacts.yaml")
	err := os.WriteFile(contactsFile, []byte(`version: "1.0"
contacts:
  - name: Judith Anna Stofer
    x:
      - url: https://x.com/jastofer
        verified: true
`), 0o600)
	if err != nil {
		t.Fatalf("write contacts file: %v", err)
	}
	mapper, err := contacts.LoadContacts(contactsFile)
	if err != nil {
		t.Fatalf("load contacts: %v", err)
	}

	thread := FormatVoteThread(group, mapper, DefaultMaxChars)
	all := allThreadText(thread)

	if !strings.Contains(all, "Weitere Unterzeichnende") {
		t.Fatalf("this fixture should shed at 280 chars:\n%s", all)
	}
	if !strings.Contains(all, "Judith Anna Stofer @jastofer") {
		t.Errorf("a deferred signatory was named but not tagged:\n%s", all)
	}
}

func TestFormatVoteThread_TagsFraktionInSubtitle(t *testing.T) {
	mapper := contacts.NewMapperForTest([]contacts.Contact{
		{Name: "AL Stadt Zürich", Kind: contacts.KindOrganization, X: contacts.VerifiedAccounts("https://x.com/al_zuerich")},
	})
	group := []votes.Vote{{
		SourceID:     "sub-1",
		Jurisdiction: "zurich-city",
		Title:        "2025/354 Weisung",
		Subtitle:     "Antrag der AL-Fraktion zu Dispositivziffer 1",
		Date:         testfixtures.MustDate("2025-11-26"),
		Decision:     "angenommen",
		Yes:          intPtr(60),
		No:           intPtr(50),
		Abstention:   intPtr(0),
	}}
	thread := FormatVoteThread(group, mapper, DefaultMaxChars)
	if got := allThreadText(thread); !strings.Contains(got, "AL-Fraktion @al_zuerich") {
		t.Errorf("subtitle Fraktion not tagged:\n%s", got)
	}
}
