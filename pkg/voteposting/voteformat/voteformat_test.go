package voteformat

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestCleanVoteTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple title with Geschäft number and Postulat",
			input:    "2025/369 Postulat von Reto Brüesch (SVP) und Jean-Marc Jung (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche",
			expected: "Postulat von Reto Brüesch (SVP) und Jean-Marc Jung (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche",
		},
		{
			name:     "Title with Motion",
			input:    "2025/370 Motion von Liv Mahrer (SP) vom 05.02.2025: Festsetzung der Selnaustrasse",
			expected: "Motion von Liv Mahrer (SP) vom 05.02.2025: Festsetzung der Selnaustrasse",
		},
		{
			name:     "Title without type word",
			input:    "2024/431 Anpassung der Bau- und Zonenordnung",
			expected: "Anpassung der Bau- und Zonenordnung",
		},
		{
			name:     "Title with newlines",
			input:    "2025/369 Postulat von\nReto Brüesch\r\n(SVP) vom 05.03.2025",
			expected: "Postulat von Reto Brüesch (SVP) vom 05.03.2025",
		},
		{
			name:     "Title with extra spaces",
			input:    "2025/369   Postulat  von   Reto   Brüesch",
			expected: "Postulat von Reto Brüesch",
		},
		{
			name:     "Title without Geschäft number",
			input:    "Postulat von Reto Brüesch (SVP) vom 05.03.2025",
			expected: "Postulat von Reto Brüesch (SVP) vom 05.03.2025",
		},
		{
			name:     "Empty title",
			input:    "",
			expected: "",
		},
		{
			name:     "Only Geschäft number",
			input:    "2025/369",
			expected: "2025/369",
		},
		{
			name:     "Title with der/die/das prefix",
			input:    "2025/369 der SP-, AL- und Die Mitte/EVP-Fraktion vom 05.02.2025: Abgeltung der Kosten",
			expected: "der SP-, AL- und Die Mitte/EVP-Fraktion vom 05.02.2025: Abgeltung der Kosten",
		},
		{
			name:     "Real example - should preserve Postulat",
			input:    "2025/100 Postulat von Reto Brüesch (SVP) und Jean-Marc Jung (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche im Rahmen der geplanten BZO-Revision",
			expected: "Postulat von Reto Brüesch (SVP) und Jean-Marc Jung (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche im Rahmen der geplanten BZO-Revision",
		},
		{
			name:     "Real API data - Postulat with carriage returns (was correct before)",
			input:    "2024/588\r\nPostulat von Urs Riklin (Grüne) und Dr. Tamara Bosshardt (SP) vom 18.12.2024:\r\nBarrierefreie und familiengerechte öffentliche Toiletten, Anpassung der Raumstandards von Schul- und Sportanlagen",
			expected: "Postulat von Urs Riklin (Grüne) und Dr. Tamara Bosshardt (SP) vom 18.12.2024: Barrierefreie und familiengerechte öffentliche Toiletten, Anpassung der Raumstandards von Schul- und Sportanlagen",
		},
		{
			name:     "Real API data - Motion with carriage returns (was incorrect - cutoff bug)",
			input:    "2025/51\r\nMotion von Liv Mahrer (SP), Marco Denoth (SP), Beat Oberholzer (GLP) und 3 Mitunterzeichnenden vom 05.02.2025:\r\nFestsetzung der Selnaustrasse zwischen Sihlstrasse und Stauffacherbrücke als Strassenraum mit einer dem Platz- oder Strassenraum zugewandten Erdgeschossnutzung, Änderung der Bau- und Zonenordnung (BZO)",
			expected: "Motion von Liv Mahrer (SP), Marco Denoth (SP), Beat Oberholzer (GLP) und 3 Mitunterzeichnenden vom 05.02.2025: Festsetzung der Selnaustrasse zwischen Sihlstrasse und Stauffacherbrücke als Strassenraum mit einer dem Platz- oder Strassenraum zugewandten Erdgeschossnutzung, Änderung der Bau- und Zonenordnung (BZO)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanVoteTitle(tt.input)
			if result != tt.expected {
				t.Errorf("CleanVoteTitle() failed\ninput:    %q\nexpected: %q\ngot:      %q", tt.input, tt.expected, result)
			}
		})
	}
}

func TestCleanVoteSubtitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Subtitle with slash pattern",
			input:    "2025/369 Abstimmung über Postulat",
			expected: "Abstimmung über Postulat",
		},
		{
			name:     "Subtitle with underscore pattern",
			input:    "2025_0369 Abstimmung über Motion",
			expected: "Abstimmung über Motion",
		},
		{
			name:     "Subtitle with newlines",
			input:    "2025/369 Abstimmung\nüber\r\nPostulat",
			expected: "Abstimmung über Postulat",
		},
		{
			name:     "Subtitle without number",
			input:    "Abstimmung über Postulat",
			expected: "Abstimmung über Postulat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanVoteSubtitle(tt.input)
			if result != tt.expected {
				t.Errorf("CleanVoteSubtitle() failed\ninput:    %q\nexpected: %q\ngot:      %q", tt.input, tt.expected, result)
			}
		})
	}
}

func ptr(n int) *int { return &n }

func TestIsAuswahlVote(t *testing.T) {
	tests := []struct {
		name     string
		counts   VoteCounts
		expected bool
	}{
		{
			name:     "standard Ja/Nein vote",
			counts:   VoteCounts{Ja: ptr(86), Nein: ptr(13), Enthaltung: ptr(12), Abwesend: ptr(14)},
			expected: false,
		},
		{
			name:     "Auswahl A/B/C vote",
			counts:   VoteCounts{Ja: ptr(0), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(11), A: ptr(50), B: ptr(24), C: ptr(40)},
			expected: true,
		},
		{
			name:     "Auswahl A/B only",
			counts:   VoteCounts{Abwesend: ptr(10), A: ptr(75), B: ptr(40)},
			expected: true,
		},
		{
			name:     "all nil (unsupported, but not Auswahl)",
			counts:   VoteCounts{},
			expected: false,
		},
		{
			name:     "all zero (unsupported, but not Auswahl)",
			counts:   VoteCounts{A: ptr(0), B: ptr(0)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuswahlVote(tt.counts)
			if got != tt.expected {
				t.Errorf("IsAuswahlVote() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsUnsupportedVoteType(t *testing.T) {
	tests := []struct {
		name     string
		counts   VoteCounts
		expected bool
	}{
		{
			name:     "standard vote with results",
			counts:   VoteCounts{Ja: ptr(107), Nein: ptr(8), Enthaltung: ptr(0), Abwesend: ptr(10)},
			expected: false,
		},
		{
			name:     "auswahl vote with A/B/C",
			counts:   VoteCounts{Ja: ptr(0), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(11), A: ptr(50), B: ptr(24), C: ptr(40)},
			expected: false,
		},
		{
			name:     "all nil (no fields parsed)",
			counts:   VoteCounts{},
			expected: true,
		},
		{
			name:     "all zeros (unknown format)",
			counts:   VoteCounts{Ja: ptr(0), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(11)},
			expected: true,
		},
		{
			name:     "only Abwesend non-zero does not make it supported",
			counts:   VoteCounts{Abwesend: ptr(15)},
			expected: true,
		},
		{
			name:     "single Ja vote is enough",
			counts:   VoteCounts{Ja: ptr(1)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnsupportedVoteType(tt.counts)
			if got != tt.expected {
				t.Errorf("IsUnsupportedVoteType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatVoteCounts(t *testing.T) {
	tests := []struct {
		name     string
		counts   VoteCounts
		expected string
	}{
		{
			name:     "standard vote short labels",
			counts:   VoteCounts{Ja: ptr(80), Nein: ptr(30), Enthaltung: ptr(5), Abwesend: ptr(10)},
			expected: "📊 80 Ja | 30 Nein | 5 Enth. | 10 Abw.",
		},
		{
			name:     "standard vote nil counts treated as zero",
			counts:   VoteCounts{Ja: ptr(99), Nein: ptr(12), Enthaltung: nil, Abwesend: nil},
			expected: "📊 99 Ja | 12 Nein | 0 Enth. | 0 Abw.",
		},
		{
			name:     "auswahl vote with A/B/C (example 7c90673c)",
			counts:   VoteCounts{Ja: ptr(0), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(11), A: ptr(50), B: ptr(24), C: ptr(40)},
			expected: "📊 A: 50 | B: 24 | C: 40 | Abw. 11",
		},
		{
			name:     "auswahl vote — only D and E used",
			counts:   VoteCounts{Abwesend: ptr(5), D: ptr(60), E: ptr(55)},
			expected: "📊 D: 60 | E: 55 | Abw. 5",
		},
		{
			name:     "all zero (unsupported) falls back to standard format",
			counts:   VoteCounts{Ja: ptr(0), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(0)},
			expected: "📊 0 Ja | 0 Nein | 0 Enth. | 0 Abw.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatVoteCounts(tt.counts)
			if got != tt.expected {
				t.Errorf("FormatVoteCounts()\nexpected: %q\ngot:      %q", tt.expected, got)
			}
		})
	}
}

func TestFormatVoteCountsLong(t *testing.T) {
	tests := []struct {
		name     string
		counts   VoteCounts
		expected string
	}{
		{
			name:     "standard vote long labels",
			counts:   VoteCounts{Ja: ptr(107), Nein: ptr(8), Enthaltung: ptr(0), Abwesend: ptr(10)},
			expected: "📊 107 Ja | 8 Nein | 0 Enthaltung | 10 Abwesend",
		},
		{
			name:     "auswahl vote long labels",
			counts:   VoteCounts{Abwesend: ptr(11), A: ptr(50), B: ptr(24), C: ptr(40)},
			expected: "📊 A: 50 | B: 24 | C: 40 | Abwesend 11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatVoteCountsLong(tt.counts)
			if got != tt.expected {
				t.Errorf("FormatVoteCountsLong()\nexpected: %q\ngot:      %q", tt.expected, got)
			}
		})
	}
}

func TestIsSchlussabstimmung(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"contains Schlussabstimmung", "2026_0244 Schlussabstimmung", true},
		{"contains Schlussabstimmung with detail", "Schlussabstimmung über die Dispositivziffer 1", true},
		{"case insensitive", "2026_0244 schlussabstimmung", true},
		{"mixed case", "SCHLUSSABSTIMMUNG", true},
		{"Dringlicherklärung", "2026_0244 Dringlicherklärung", false},
		{"empty string", "", false},
		{"unrelated text", "Antrag 1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSchlussabstimmung(tt.input)
			if got != tt.expected {
				t.Errorf("IsSchlussabstimmung(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSingleVoteSubtitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Dringlicherklärung with number", "2026_0244 Dringlicherklärung", "Dringlicherklärung"},
		{"Schlussabstimmung returns empty", "2026_0244 Schlussabstimmung", ""},
		{"empty returns empty", "", ""},
		{"whitespace-only after strip returns empty", "  ", ""},
		{"no number prefix", "Dringlicherklärung", "Dringlicherklärung"},
		{"Schlussabstimmung case insensitive", "schlussabstimmung über Ziffer 1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := singleVoteSubtitle(tt.input)
			if got != tt.expected {
				t.Errorf("singleVoteSubtitle(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSubVoteLabelNamesTheVoteType(t *testing.T) {
	// The real 15.06.2026 Glattalbahn group: five votes, second-precision
	// timestamps, no per-vote title of any kind.
	at1130 := time.Date(2026, 6, 15, 11, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		subtitle  string
		voteType  string
		date      time.Time
		index     int
		groupSize int
		expected  string
	}{
		// Kanton Zürich publishes no per-vote title, so the ordinal carries the
		// label — and, in a group, the sitting time as well.
		{"quorum on the ordinal fallback", "", "Quorum", time.Time{}, 1, 1, "Abstimmung 2 · Quorum"},
		{"normal keeps the bare ordinal", "", "Normal", time.Time{}, 0, 1, "Abstimmung 1"},
		{"missing type keeps the bare ordinal", "", "", time.Time{}, 0, 1, "Abstimmung 1"},

		// The clock disambiguates the ordinal; the type qualifies the whole
		// label. Both appear, in that order.
		{"clock and type together", "", "Quorum", at1130, 1, 5, "Abstimmung 2 (11:30) · Quorum"},
		{"clock without a type", "", "Normal", at1130, 0, 5, "Abstimmung 1 (11:30)"},

		// Stadt Zürich titles each vote and uses the same type vocabulary. A
		// source-supplied title needs no clock, but still takes the type.
		{"quorum after a subtitle", "Schlussabstimmung", "Quorum", at1130, 0, 5, "Schlussabstimmung · Quorum"},

		{"Auswahl types stay unlabelled", "", "Gleichgerichtete Anträge mit 4 Optionen", time.Time{}, 0, 1, "Abstimmung 1"},
		{"cup is labelled", "", "Cup-Abstimmung", time.Time{}, 2, 1, "Abstimmung 3 · Cup-Abstimmung"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := votes.Vote{Subtitle: tt.subtitle, Type: tt.voteType, Date: tt.date}
			if got := SubVoteLabel(v, tt.index, tt.groupSize); got != tt.expected {
				t.Errorf("SubVoteLabel(subtitle=%q, type=%q, %d, %d) = %q, want %q",
					tt.subtitle, tt.voteType, tt.index, tt.groupSize, got, tt.expected)
			}
		})
	}
}

func intPtr(n int) *int { return &n }

func TestIsDecisionConsistent(t *testing.T) {
	tests := []struct {
		name            string
		schlussresultat string
		ja              *int
		nein            *int
		wantConsistent  bool
	}{
		{
			// Reproduces the 2025/217 bug: API returned "Ja" but counts show Nein > Ja.
			name:            "API says Ja but Nein > Ja — inconsistent",
			schlussresultat: "Ja",
			ja:              intPtr(41),
			nein:            intPtr(75),
			wantConsistent:  false,
		},
		{
			name:            "API says Angenommen but Nein > Ja — inconsistent",
			schlussresultat: "Angenommen",
			ja:              intPtr(41),
			nein:            intPtr(75),
			wantConsistent:  false,
		},
		{
			name:            "API says Ja and Ja > Nein — consistent",
			schlussresultat: "Ja",
			ja:              intPtr(75),
			nein:            intPtr(41),
			wantConsistent:  true,
		},
		{
			name:            "API says Nein with Nein > Ja — consistent",
			schlussresultat: "Nein",
			ja:              intPtr(41),
			nein:            intPtr(75),
			wantConsistent:  true,
		},
		{
			name:            "Tie: Ja == Nein — consistent (no contradiction)",
			schlussresultat: "Ja",
			ja:              intPtr(60),
			nein:            intPtr(60),
			wantConsistent:  true,
		},
		{
			name:            "Nil ja count — consistent (cannot validate)",
			schlussresultat: "Ja",
			ja:              nil,
			nein:            intPtr(75),
			wantConsistent:  true,
		},
		{
			name:            "Nil nein count — consistent (cannot validate)",
			schlussresultat: "Ja",
			ja:              intPtr(41),
			nein:            nil,
			wantConsistent:  true,
		},
		{
			name:            "Auswahl result — always consistent",
			schlussresultat: "Auswahl A",
			ja:              intPtr(0),
			nein:            intPtr(99),
			wantConsistent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDecisionConsistent(tt.schlussresultat, tt.ja, tt.nein)
			if got != tt.wantConsistent {
				t.Errorf("IsDecisionConsistent(%q, ja=%v, nein=%v) = %v, want %v",
					tt.schlussresultat, tt.ja, tt.nein, got, tt.wantConsistent)
			}
		})
	}
}

func TestPostHeadline(t *testing.T) {
	day := time.Date(2026, 7, 6, 10, 21, 43, 0, time.UTC)

	tests := []struct {
		name  string
		group []votes.Vote
		want  string
	}{
		{
			name:  "named body and known date",
			group: []votes.Vote{{Body: "Kantonsrat ZH", Date: day}},
			want:  "Kantonsrat ZH | Abstimmung vom 06.07.2026 (10:21)",
		},
		{
			// Stadt Zürich supplies sitting dates at midnight, so a clock there
			// would invent a precision the source does not have.
			name:  "a midnight date carries no clock",
			group: []votes.Vote{{Body: "Gemeinderat Zürich", Date: time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)}},
			want:  "Gemeinderat Zürich | Abstimmung vom 06.07.2026",
		},
		{
			// A group spans several times; its entries carry them instead.
			name: "a multi-vote group keeps the date alone",
			group: []votes.Vote{
				{Body: "Kantonsrat ZH", Date: day},
				{Body: "Kantonsrat ZH", Date: day.Add(9 * time.Minute)},
			},
			want: "Kantonsrat ZH | Abstimmung vom 06.07.2026",
		},
		{
			// Votes with an unparseable date are deliberately kept rather than
			// discarded, so this is reachable — and "Abstimmung vom " trailing
			// into nothing is not an acceptable way to render it.
			name:  "unknown date drops the date clause",
			group: []votes.Vote{{Body: "Kantonsrat ZH"}},
			want:  "Kantonsrat ZH | Abstimmung",
		},
		{
			name:  "unnamed body omits the chamber",
			group: []votes.Vote{{Date: day}},
			want:  "Abstimmung vom 06.07.2026 (10:21)",
		},
		{
			name:  "empty group",
			group: nil,
			want:  "Abstimmung",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PostHeadline(tc.group); got != tc.want {
				t.Errorf("PostHeadline = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSubVoteLabel(t *testing.T) {
	timed := time.Date(2026, 6, 29, 14, 32, 11, 0, time.UTC)
	midnight := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		vote      votes.Vote
		index     int
		groupSize int
		want      string
	}{
		{
			name:      "a titled vote uses its own title",
			vote:      votes.Vote{Subtitle: "Schlussabstimmung", Date: timed},
			index:     1,
			groupSize: 3,
			want:      "Schlussabstimmung",
		},
		{
			name:      "an untitled vote in a group falls back to ordinal and time",
			vote:      votes.Vote{Date: timed},
			index:     1,
			groupSize: 3,
			want:      "Abstimmung 2 (14:32)",
		},
		{
			// A lone vote is already named by the post's own headline; a clock
			// reading next to it distinguishes it from nothing.
			name:      "a group of one gets no time",
			vote:      votes.Vote{Date: timed},
			index:     0,
			groupSize: 1,
			want:      "Abstimmung 1",
		},
		{
			// PARIS dates are midnight. Printing "(00:00)" would claim a
			// precision the source never supplied.
			name:      "a date without a clock time gets no time",
			vote:      votes.Vote{Date: midnight},
			index:     0,
			groupSize: 4,
			want:      "Abstimmung 1",
		},
		{
			name:      "a zero date gets no time",
			vote:      votes.Vote{},
			index:     2,
			groupSize: 4,
			want:      "Abstimmung 3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SubVoteLabel(tc.vote, tc.index, tc.groupSize); got != tc.want {
				t.Errorf("SubVoteLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

// A result card prints the vote's time in its corner, so its heading drops the
// clock that SubVoteLabel gives the text posts — and nothing else.
func TestCardSubVoteLabelLeavesTheClockToTheCorner(t *testing.T) {
	timed := time.Date(2026, 6, 29, 14, 32, 11, 0, time.UTC)

	untitled := votes.Vote{Date: timed, Type: "Quorum"}
	if got := SubVoteLabel(untitled, 1, 3); got != "Abstimmung 2 (14:32) · Quorum" {
		t.Fatalf("SubVoteLabel = %q, the text posts' label changed", got)
	}
	if got := CardSubVoteLabel(untitled, 1, 3); got != "Abstimmung 2 · Quorum" {
		t.Errorf("CardSubVoteLabel = %q, want %q", got, "Abstimmung 2 · Quorum")
	}

	titled := votes.Vote{Subtitle: "Schlussabstimmung", Date: timed}
	if got := CardSubVoteLabel(titled, 1, 3); got != "Schlussabstimmung" {
		t.Errorf("CardSubVoteLabel = %q, want the vote's own title", got)
	}
}

func TestFormatVoteDateTime(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"date and clock", time.Date(2026, 6, 15, 11, 29, 43, 0, time.UTC), "15.06.2026 11:29"},
		// PARIS dates are midnight: the date stands alone rather than "00:00".
		{"midnight is a date alone", time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), "15.06.2025"},
		{"unknown date", time.Time{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatVoteDateTime(tt.date); got != tt.want {
				t.Errorf("FormatVoteDateTime(%v) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

// A vote type nobody has taught the formatters about must not reach a reader.
// The cases below are the real ones: Kanton Zürich serves a null type for
// attendance determinations and for the occasional genuine quorum vote, and
// both look like ordinary lopsided Ja/Nein tallies once rendered.
func TestIsHandledVoteType(t *testing.T) {
	tests := []struct {
		voteType string
		want     bool
	}{
		{"Normal", true},
		{"Quorum", true},
		// The option count varies, so the whole family matches by prefix.
		{"Gleichgerichtete Anträge mit 3 Optionen", true},
		{"Gleichgerichtete Anträge mit 4 Optionen", true},

		// An unset type is the case this guard exists for: it is what Kanton
		// Zürich still serves for Anwesenheitsermittlung, which is a roll call
		// and not a vote at all.
		{"", false},
		{"   ", false},
		// Knockout rounds between more than two proposals. The source reports no
		// aggregate counts for these, so there is nothing correct to render.
		{"Cup-Abstimmung", false},
		{"Something The Source Just Invented", false},
	}

	for _, tt := range tests {
		t.Run(tt.voteType, func(t *testing.T) {
			if got := IsHandledVoteType(tt.voteType); got != tt.want {
				t.Errorf("IsHandledVoteType(%q) = %v, want %v", tt.voteType, got, tt.want)
			}
		})
	}
}

// A quorum vote counts supporters against a threshold and has no Nein to cast.
// Rendering it in the standard four-part line prints "0 Nein | 0 Enth." for
// positions nobody could take, and files every non-supporter under "Abwesend" —
// which the official record shows is wrong for most of them.
func TestQuorumCountsDropThePhantomColumns(t *testing.T) {
	// The real 15.06.2026 Ausgabenbremse: 129 supporters, 51 not.
	quorum := VoteCounts{
		Ja: ptr(129), Nein: ptr(0), Enthaltung: ptr(0), Abwesend: ptr(51),
		Type: "Quorum",
	}
	const want = "📊 129 Zustimmungen | 51 ohne Zustimmung"

	// Both platforms render the same line: it is already shorter than the
	// standard one, so there is nothing to abbreviate for the tight limit.
	if got := FormatVoteCounts(quorum); got != want {
		t.Errorf("FormatVoteCounts() = %q, want %q", got, want)
	}
	if got := FormatVoteCountsLong(quorum); got != want {
		t.Errorf("FormatVoteCountsLong() = %q, want %q", got, want)
	}

	// Upstream #179 may start mapping the Kantonsrat's "Nicht abgestimmt" onto
	// no instead of absent. Reading only Ja and Abwesend would then drop those
	// members from the line — 46 of them on the vote above. Both buckets mean
	// "did not support", so they are summed and the total stays whole.
	afterUpstreamFix := VoteCounts{
		Ja: ptr(128), Nein: ptr(46), Enthaltung: ptr(0), Abwesend: ptr(6),
		Type: "Quorum",
	}
	if got := FormatVoteCounts(afterUpstreamFix); got != "📊 128 Zustimmungen | 52 ohne Zustimmung" {
		t.Errorf("counts would lose the Nein bucket: %q", got)
	}

	// An ordinary vote is untouched, including one that happens to be lopsided.
	normal := VoteCounts{
		Ja: ptr(130), Nein: ptr(44), Enthaltung: ptr(0), Abwesend: ptr(6),
		Type: "Normal",
	}
	if got := FormatVoteCounts(normal); !strings.Contains(got, "Nein") {
		t.Errorf("FormatVoteCounts() dropped Nein from a Normal vote: %q", got)
	}
}

// The faction table has the same problem as the summary line, and is fixed at
// the aggregation step so the image renderer inherits it.
func TestQuorumFraktionColumns(t *testing.T) {
	v := votes.Vote{
		Type: "Quorum",
		MemberVotes: []votes.MemberVote{
			{Fraktion: "SVP", Choice: "Abwesend"},
			{Fraktion: "SVP", Choice: "Abwesend"},
			{Fraktion: "SP", Choice: "Ja"},
		},
	}

	got := FormatFraktionBreakdown(AggregateFraktionCounts(v))

	if !strings.Contains(got, "(Zust./ohne)") {
		t.Errorf("expected the two-column quorum header, got:\n%s", got)
	}
	for _, unwanted := range []string{"Nein", "Enth", "Abw"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("quorum breakdown still names %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "SVP 0/2") || !strings.Contains(got, "SP 1/0") {
		t.Errorf("unexpected counts:\n%s", got)
	}

	// Same forward-compatibility concern as the summary line: a "Nein" choice
	// joins the non-supporters instead of opening a third column that would
	// split one group in two.
	withNein := votes.Vote{
		Type: "Quorum",
		MemberVotes: []votes.MemberVote{
			{Fraktion: "SVP", Choice: "Nein"},
			{Fraktion: "SVP", Choice: "Abwesend"},
			{Fraktion: "SP", Choice: "Ja"},
		},
	}
	got = FormatFraktionBreakdown(AggregateFraktionCounts(withNein))
	if !strings.Contains(got, "(Zust./ohne)") || !strings.Contains(got, "SVP 0/2") {
		t.Errorf("a Nein choice should join the non-supporters:\n%s", got)
	}
}

// TestQuorumTallyFoldsNeinIntoTheUnsupportedBucket pins the arithmetic every
// surface of a post shares.
//
// While every quorum vote the source typed happened to report Nein=0, reading
// Abwesend alone looked correct, and the generated image did exactly that. The
// 17.08.2026 Ausgabenbremse reports 141 Ja to 1 Nein with 38 absent, and the
// card then said "38 ohne" under a Fraktion table whose own column summed to
// 39 — the image contradicting itself.
func TestQuorumTallyFoldsNeinIntoTheUnsupportedBucket(t *testing.T) {
	tests := []struct {
		name                     string
		ja, nein, abwesend       *int
		wantSupport, wantWithout int
	}{
		{"Ausgabenbremse with a Nein cast", intPtr(141), intPtr(1), intPtr(38), 141, 39},
		{"no Nein reported", intPtr(128), intPtr(0), intPtr(52), 128, 52},
		{"nothing reported at all", nil, nil, nil, 0, 0},
		{"absent unreported", intPtr(80), intPtr(2), nil, 80, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := VoteCounts{Ja: tt.ja, Nein: tt.nein, Abwesend: tt.abwesend, Type: "Quorum"}
			support, without := QuorumTally(c)
			if support != tt.wantSupport || without != tt.wantWithout {
				t.Errorf("QuorumTally = (%d, %d), want (%d, %d)",
					support, without, tt.wantSupport, tt.wantWithout)
			}
		})
	}
}

// TestQuorumCaptionAgreesWithTheTally guards the two against drifting apart
// again, now that they share one source.
func TestQuorumCaptionAgreesWithTheTally(t *testing.T) {
	c := VoteCounts{Ja: intPtr(141), Nein: intPtr(1), Enthaltung: intPtr(0), Abwesend: intPtr(38), Type: "Quorum"}

	_, without := QuorumTally(c)
	caption := formatQuorumCounts(c)

	if !strings.Contains(caption, fmt.Sprintf("%d ohne Zustimmung", without)) {
		t.Errorf("caption %q does not state the tally's %d", caption, without)
	}
}

// TestGroupPrefixLineNamesTheKindOfBusiness pins the label line that tells a
// reader what they are looking at. A Postulat is a demand addressed to the
// Regierungsrat, not a decision taken, and the tally alone does not say so.
func TestGroupPrefixLineNamesTheKindOfBusiness(t *testing.T) {
	vote := func(affairType, subtitle, voteType string) votes.Vote {
		return votes.Vote{
			Subtitle: subtitle,
			Type:     voteType,
			Affair:   votes.Affair{Type: affairType},
		}
	}

	tests := []struct {
		name  string
		group []votes.Vote
		want  string
	}{
		{
			name:  "the business type alone",
			group: []votes.Vote{vote("Postulat", "", "Normal")},
			want:  "Postulat",
		},
		{
			name:  "a government bill is named too",
			group: []votes.Vote{vote("Vorlage", "", "Normal")},
			want:  "Vorlage",
		},
		{
			// The ballot type stays out: "Vorlage · Ausgabenbremse" glued a fact
			// about the Geschäft to a fact about this ballot. It goes with the
			// counts, which is what it explains.
			name:  "the ballot type stays out of it",
			group: []votes.Vote{vote("Vorlage", "", "Ausgabenbremse")},
			want:  "Vorlage",
		},
		{
			name:  "the ballot type does not join a subtitle either",
			group: []votes.Vote{vote("Vorlage", "Dringlicherklärung", "Quorum")},
			want:  "Vorlage · Dringlicherklärung",
		},
		{
			// Stadt Zürich reports no business type, so its line is unchanged.
			name:  "no business type leaves the existing prefix",
			group: []votes.Vote{vote("", "Änderungsantrag zu Ziffer 1", "Normal")},
			want:  "Änderungsantrag zu Ziffer 1",
		},
		{
			// Which question was put is meaningless when several share the post;
			// the per-vote headings carry that instead.
			name: "a multi-vote group keeps only the business type",
			group: []votes.Vote{
				vote("Vorlage", "Schlussabstimmung", "Normal"),
				vote("Vorlage", "", "Ausgabenbremse"),
			},
			want: "Vorlage",
		},
		{
			name:  "nothing to say",
			group: []votes.Vote{vote("", "", "Normal")},
			want:  "",
		},
		{
			name:  "no votes",
			group: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupPrefixLine(tt.group); got != tt.want {
				t.Errorf("GroupPrefixLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestGroupPrefixLineNamesWhoFiledIt pins the other half of the label line.
//
// Stadt Zürich's titles carry their submitters and Kanton Zürich's do not, so
// without this a cantonal post names nobody — and a post that names nobody tags
// nobody, however well curated the contacts file is.
func TestGroupPrefixLineNamesWhoFiledIt(t *testing.T) {
	vote := func(affairType, subtitle string, authors ...votes.Author) votes.Vote {
		return votes.Vote{
			Subtitle: subtitle,
			Affair:   votes.Affair{Type: affairType, Authors: authors},
		}
	}

	weidmann := votes.Author{Name: "Tobias Weidmann", Party: "SVP"}
	koch := votes.Author{Name: "Nadia Koch", Party: "GLP"}

	tests := []struct {
		name  string
		group []votes.Vote
		want  string
	}{
		{
			name:  "one author",
			group: []votes.Vote{vote("Postulat", "", weidmann)},
			want:  "Postulat von Tobias Weidmann (SVP)",
		},
		{
			name:  "two are joined as German prose",
			group: []votes.Vote{vote("Motion", "", weidmann, koch)},
			want:  "Motion von Tobias Weidmann (SVP) und Nadia Koch (GLP)",
		},
		{
			// The Abstimmungsgegenstand still comes last: it identifies the
			// question, not the business or who filed it.
			name:  "the subtitle keeps its place after the authors",
			group: []votes.Vote{vote("Motion", "Dringlicherklärung", weidmann)},
			want:  "Motion von Tobias Weidmann (SVP) · Dringlicherklärung",
		},
		{
			// Government business: the Regierungsrat and its Direktionen are
			// not people and are not named here.
			name:  "no authors leaves the line as it was",
			group: []votes.Vote{vote("Vorlage", "")},
			want:  "Vorlage",
		},
		{
			// "von Tobias Weidmann (SVP)" alone reads as a sentence missing its
			// subject. The two arrive from the same call, so this is a guard.
			name:  "authors without a business type are not rendered alone",
			group: []votes.Vote{vote("", "", weidmann)},
			want:  "",
		},
		{
			// Several votes on one business share the post, and the authors are
			// a property of the business rather than of any one vote.
			name:  "a group is labelled once, from its business",
			group: []votes.Vote{vote("Motion", "", weidmann), vote("Motion", "", weidmann)},
			want:  "Motion von Tobias Weidmann (SVP)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupPrefixLine(tt.group); got != tt.want {
				t.Errorf("GroupPrefixLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWahlgeschaeftBody pins what an election post says: the business title
// exactly as the parliament wrote it, and one sentence about our data. In
// particular the "für <Name>" tail survives intact — it names the member being
// replaced, and editing official wording is the thing this post does not do.
func TestWahlgeschaeftBody(t *testing.T) {
	group := []votes.Vote{{
		Title:  "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann",
		Type:   votes.AttendanceType,
		Affair: votes.Affair{Type: votes.WahlAffairType},
	}}

	want := "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann\n\n" + WahlgeschaeftNoResult
	if got := WahlgeschaeftBody(group); got != want {
		t.Errorf("WahlgeschaeftBody() =\n%q\nwant\n%q", got, want)
	}

	if got := WahlgeschaeftBody(nil); got != WahlgeschaeftNoResult {
		t.Errorf("WahlgeschaeftBody(nil) = %q, want the bare sentence", got)
	}
}

// TestPostHeadline_Wahlgeschaeft pins that an election post is not labelled an
// Abstimmung and carries no clock time: the only timestamp available is the
// roll call's, which is not when anything was decided.
func TestPostHeadline_Wahlgeschaeft(t *testing.T) {
	group := []votes.Vote{{
		Body:   "Kantonsrat ZH",
		Date:   time.Date(2026, 9, 14, 8, 19, 0, 0, time.UTC),
		Title:  "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann",
		Type:   votes.AttendanceType,
		Affair: votes.Affair{Type: votes.WahlAffairType},
	}}

	want := "Kantonsrat ZH | Wahlgeschäft vom 14.09.2026"
	if got := PostHeadline(group); got != want {
		t.Errorf("PostHeadline() = %q, want %q", got, want)
	}
}

func TestAuthorList(t *testing.T) {
	tests := []struct {
		name    string
		authors []votes.Author
		want    string
	}{
		{name: "nobody", want: ""},
		{
			name:    "three take a comma and a und",
			authors: []votes.Author{{Name: "A", Party: "SP"}, {Name: "B", Party: "FDP"}, {Name: "C", Party: "AL"}},
			want:    "A (SP), B (FDP) und C (AL)",
		},
		{
			// The party is what tells two politicians of the same name apart,
			// but its absence is not a reason to drop the name here — the
			// adapter has already decided who may be named.
			name:    "a missing party leaves the bare name",
			authors: []votes.Author{{Name: "A"}},
			want:    "A",
		},
		{
			name:    "an empty name is skipped rather than rendered as a party alone",
			authors: []votes.Author{{Name: "  ", Party: "SP"}, {Name: "B", Party: "FDP"}},
			want:    "B (FDP)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AuthorList(tt.authors, 0); got != tt.want {
				t.Errorf("AuthorList() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFitAuthorPrefix pins what gives way when a signatory list and a title
// compete for one post: never the title.
func TestFitAuthorPrefix(t *testing.T) {
	five := []votes.Author{
		{Name: "Hannah Pfalzgraf", Party: "SP"},
		{Name: "Yvonne Bürgin", Party: "Die Mitte"},
		{Name: "Andrea Gisler", Party: "GLP"},
		{Name: "Silvia Rigoni", Party: "Grüne"},
		{Name: "Judith Anna Stofer", Party: "AL"},
	}
	group := []votes.Vote{{Affair: votes.Affair{Type: "Postulat", Authors: five}}}
	measure := func(s string) int { return len([]rune(s)) }
	full := "Postulat von Hannah Pfalzgraf (SP), Yvonne Bürgin (Die Mitte), Andrea Gisler (GLP), Silvia Rigoni (Grüne) und Judith Anna Stofer (AL)"

	t.Run("room for everyone names everyone", func(t *testing.T) {
		prefix, deferred := FitAuthorPrefix(group, 500, measure)
		if prefix != full {
			t.Errorf("prefix = %q, want every signatory named", prefix)
		}
		if len(deferred) != 0 {
			t.Errorf("deferred = %+v, want nobody held back", deferred)
		}
	})

	t.Run("a tight budget sheds from the end", func(t *testing.T) {
		prefix, deferred := FitAuthorPrefix(group, measure(full)-10, measure)
		if measure(prefix) > measure(full)-10 {
			t.Errorf("prefix = %q, still over budget", prefix)
		}
		if !strings.HasSuffix(prefix, "u. a.") {
			t.Errorf("prefix = %q, want it to signal that more names follow", prefix)
		}
		if len(deferred) == 0 {
			t.Fatal("want the shed signatories returned so a reply can name them")
		}
		// Shed from the end: the first signatory is the one that stays.
		if deferred[len(deferred)-1].Name != "Judith Anna Stofer" {
			t.Errorf("deferred = %+v, want the list shed from the end", deferred)
		}
		if !strings.Contains(prefix, "Hannah Pfalzgraf") {
			t.Errorf("prefix = %q, want the first signatory kept", prefix)
		}
	})

	t.Run("no room for any name keeps the type and defers all", func(t *testing.T) {
		prefix, deferred := FitAuthorPrefix(group, 12, measure)
		if prefix != "Postulat" {
			t.Errorf("prefix = %q, want the business type alone", prefix)
		}
		if len(deferred) != len(five) {
			t.Errorf("deferred %d, want all %d signatories moved to the block below",
				len(deferred), len(five))
		}
	})

	t.Run("business with no authors is unaffected", func(t *testing.T) {
		vorlage := []votes.Vote{{Affair: votes.Affair{Type: "Vorlage"}}}
		prefix, deferred := FitAuthorPrefix(vorlage, 5, measure)
		if prefix != "Vorlage" || len(deferred) != 0 {
			t.Errorf("got %q / %+v, want the plain label and nothing deferred", prefix, deferred)
		}
	})
}

func TestSignatoryLine(t *testing.T) {
	authors := []votes.Author{
		{Name: "Ivo Bieri", Party: "SP"},
		{Name: "Silvia Rigoni", Party: "Grüne"},
		{Name: "Judith Anna Stofer", Party: "AL"},
	}
	group := []votes.Vote{{Affair: votes.Affair{Authors: authors}}}

	if got := SignatoryLine(group, nil); got != "" {
		t.Errorf("nothing deferred: %q, want nothing to post", got)
	}
	got := SignatoryLine(group, authors[1:])
	want := "✍️ Weitere Unterzeichnende: Silvia Rigoni (Grüne) und Judith Anna Stofer (AL)"
	if got != want {
		t.Errorf("some deferred: %q, want %q", got, want)
	}
	// Nobody was named beside the title, so nobody is "further".
	got = SignatoryLine(group, authors)
	want = "✍️ Eingereicht von: Ivo Bieri (SP), Silvia Rigoni (Grüne) und Judith Anna Stofer (AL)"
	if got != want {
		t.Errorf("all deferred: %q, want %q", got, want)
	}
}

func TestAuthorListCapped(t *testing.T) {
	three := []votes.Author{
		{Name: "A", Party: "SP"}, {Name: "B", Party: "FDP"}, {Name: "C", Party: "AL"},
	}
	if got, want := AuthorList(three, 0), "A (SP), B (FDP) und C (AL)"; got != want {
		t.Errorf("max 0 = %q, want all named: %q", got, want)
	}
	if got, want := AuthorList(three, 2), "A (SP) und B (FDP) u. a."; got != want {
		t.Errorf("max 2 = %q, want %q", got, want)
	}
	if got, want := AuthorList(three, 1), "A (SP) u. a."; got != want {
		t.Errorf("max 1 = %q, want %q", got, want)
	}
	if got, want := AuthorList(three, 5), "A (SP), B (FDP) und C (AL)"; got != want {
		t.Errorf("a cap above the count = %q, want no 'u. a.': %q", got, want)
	}
}

func TestGroupLinks(t *testing.T) {
	const (
		geschaeft = "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=abc"
		segment   = "https://zh.recapp.ch/shareparl?agendaItemUid=item&segmentUid=seg"
		item      = "https://zh.recapp.ch/shareparl?agendaItemUid=item"
		sitting   = "https://www.kantonsrat.zh.ch/ratsbetrieb/sitzungenundprotokolle/?endDate=2026-08-31&startDate=2026-08-31"
	)
	canton := votes.Vote{
		SourceURL: geschaeft, GroupURL: geschaeft,
		VideoURL: segment, VideoGroupURL: item, SessionURL: sitting,
	}

	t.Run("a lone vote links its own segment", func(t *testing.T) {
		got := GroupLinks([]votes.Vote{canton})
		want := []Link{
			{Icon: "🔗", Name: "Geschäft", URL: geschaeft},
			{Icon: "🎬", Name: "Video", URL: segment},
			{Icon: "📄", Name: "Sitzung", URL: sitting},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GroupLinks() = %+v, want %+v", got, want)
		}
	})

	t.Run("a group links the agenda item, not the first vote", func(t *testing.T) {
		got := GroupLinks([]votes.Vote{canton, canton})
		if len(got) != 3 || got[1].URL != item {
			t.Errorf("video link = %+v, want the widened %q", got, item)
		}
	})

	t.Run("a vote with no business matter is not labelled as one", func(t *testing.T) {
		// applyAffair never ran, so both URL fields still hold the archive link.
		noAffair := votes.Vote{
			SourceURL: segment, GroupURL: segment,
			VideoURL: segment, SessionURL: sitting,
		}
		got := GroupLinks([]votes.Vote{noAffair})
		want := []Link{
			{Icon: "🎬", Name: "Video", URL: segment},
			{Icon: "📄", Name: "Sitzung", URL: sitting},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GroupLinks() = %+v, want the archive named once and no Geschäft: %+v", got, want)
		}
	})

	t.Run("a group whose recording has no agenda item drops the entry", func(t *testing.T) {
		noItem := canton
		noItem.VideoGroupURL = ""
		got := GroupLinks([]votes.Vote{noItem, noItem})
		for _, l := range got {
			if l.URL == segment {
				t.Errorf("GroupLinks() = %+v, want no fallback to one vote's segment", got)
			}
		}
		if len(got) != 2 {
			t.Errorf("GroupLinks() = %+v, want the Geschäft and the sitting", got)
		}
	})

	const curia = "https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=20250059"
	nationalrat := func(ref string) votes.Vote {
		return votes.Vote{
			SourceURL: curia, GroupURL: curia,
			ArchiveURL: "https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_" + ref + ".pdf",
		}
	}

	t.Run("a lone Nationalrat vote links its Protokoll", func(t *testing.T) {
		got := GroupLinks([]votes.Vote{nationalrat("33215")})
		want := []Link{
			{Icon: "🔗", Name: "Geschäft", URL: curia},
			{Icon: "📋", Name: "Protokoll", URL: "https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_33215.pdf"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GroupLinks() = %+v, want %+v", got, want)
		}
	})

	t.Run("a Nationalrat group leaves the Protokolle to the votes", func(t *testing.T) {
		g := []votes.Vote{nationalrat("33215"), nationalrat("33216")}
		got := GroupLinks(g)
		for _, l := range got {
			if l.Name == "Protokoll" {
				t.Errorf("GroupLinks() = %+v, want no Protokoll: there is none for the group", got)
			}
		}
		if len(got) != 1 {
			t.Errorf("GroupLinks() = %+v, want only the Geschäft", got)
		}
		per := VoteLinks(g, 1)
		if len(per) != 1 || per[0].Name != "Protokoll" || !strings.HasSuffix(per[0].URL, "vote_52_33216.pdf") {
			t.Errorf("VoteLinks() = %+v, want the second vote's own Protokoll", per)
		}
	})
}

func TestLinkLine(t *testing.T) {
	const geschaeft = "https://www.gemeinderat-zuerich.ch/abstimmungen/detail.php?aid=1"

	t.Run("a lone link stays unlabelled", func(t *testing.T) {
		city := []votes.Vote{{SourceURL: geschaeft, Attribution: "Source: X"}}
		want := "\n\n🔗 " + geschaeft + "\nSource: X"
		if got := LinkLine(city); got != want {
			t.Errorf("LinkLine() = %q, want %q", got, want)
		}
	})

	t.Run("several links are named so a reader can tell them apart", func(t *testing.T) {
		canton := []votes.Vote{{
			SourceURL:  geschaeft,
			VideoURL:   "https://zh.recapp.ch/shareparl?agendaItemUid=item&segmentUid=seg",
			SessionURL: "https://www.kantonsrat.zh.ch/ratsbetrieb/sitzungenundprotokolle/?d=1",
		}}
		got := LinkLine(canton)
		for _, want := range []string{"🔗 Geschäft: ", "🎬 Video: ", "📄 Sitzung: "} {
			if !strings.Contains(got, want) {
				t.Errorf("LinkLine() = %q, want it to contain %q", got, want)
			}
		}
	})

	t.Run("a source with no links at all posts no block", func(t *testing.T) {
		if got := LinkLine([]votes.Vote{{Attribution: "Source: X"}}); got != "" {
			t.Errorf("LinkLine() = %q, want nothing — a credit with no link is not a link block", got)
		}
	})
}

func TestVoteLinks(t *testing.T) {
	const group, item = "https://example.ch/geschaeft/1", "https://rec.example/s?agendaItemUid=A"
	vote := func(source, video string) votes.Vote {
		return votes.Vote{SourceURL: source, GroupURL: group, VideoURL: video, VideoGroupURL: item}
	}

	t.Run("recording at the vote's segment, business page not repeated", func(t *testing.T) {
		g := []votes.Vote{vote(group, item+"&segmentUid=S1"), vote(group, item+"&segmentUid=S2")}
		got := VoteLinks(g, 1)
		if len(got) != 1 || got[0].URL != item+"&segmentUid=S2" || got[0].Name != "Video" {
			t.Errorf("got %+v, want only the second vote's segment", got)
		}
	})

	t.Run("vote's own page beside the group's", func(t *testing.T) {
		g := []votes.Vote{vote("https://example.ch/vote/1", ""), vote("https://example.ch/vote/2", "")}
		got := VoteLinks(g, 0)
		if len(got) != 1 || got[0].URL != "https://example.ch/vote/1" || got[0].Name != "Abstimmung" {
			t.Errorf("got %+v, want the vote's own page", got)
		}
	})

	t.Run("nothing the post already links", func(t *testing.T) {
		g := []votes.Vote{vote(group, item), vote(group, item)}
		if got := VoteLinks(g, 0); len(got) != 0 {
			t.Errorf("got %+v, want none", got)
		}
	})

	t.Run("out of range", func(t *testing.T) {
		if got := VoteLinks(nil, 0); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})
}
