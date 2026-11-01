package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

var testJurisdiction = votes.Jurisdiction{
	Key:       "nationalrat",
	Name:      "Nationalrat",
	ShortName: "Nationalrat",
}

var finalAndOverall = []string{"Schlussabstimmung", "Vote final", "Gesamtabstimmung", "Vote sur l'ensemble"}

// The fixtures are the service's own responses for the Schlussabstimmungen of
// 02.10.2026, recorded with __metadata stripped. Three are on 25.059, the
// Pelzinitiative, whose tallies were checked against the Nationalrat's
// Abstimmungsprotokoll (Ref. 33215 and 33233); the fourth, 26.033, is a lone
// vote on its business.
var (
	idVoting   = regexp.MustCompile(`IdVote eq (\d+)`)
	idBusiness = regexp.MustCompile(`^/Business\(ID=(\d+),`)
)

// newTestClient serves recorded responses so the suite never touches the live
// service, and records the raw query strings it was sent.
func newTestClient(t *testing.T) (*Client, *[]string) {
	t.Helper()

	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RawQuery)

		var name string
		switch r.URL.Path {
		case "/Vote":
			name = "votes.json"
		case "/Vote/$count":
			// Only the Pelzinitiative's business has a second bill.
			n := "0"
			if strings.Contains(r.URL.Query().Get("$filter"), "BusinessNumber eq 20250059 ") {
				n = "4"
			}
			_, _ = w.Write([]byte(n))
			return
		case "/MemberCouncil":
			name = "members.json"
		case "/Voting":
			m := idVoting.FindStringSubmatch(r.URL.Query().Get("$filter"))
			if m == nil {
				http.Error(w, "no IdVote in filter", http.StatusBadRequest)
				return
			}
			name = "voting_" + m[1] + ".json"
		default:
			m := idBusiness.FindStringSubmatch(r.URL.Path)
			if m == nil {
				http.NotFound(w, r)
				return
			}
			name = "business_" + m[1] + ".json"
		}
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	c := New(testJurisdiction).WithSubjects(finalAndOverall...)
	c.SetBaseURL(srv.URL)
	c.retryDelay = 0
	return c, &requests
}

func TestFetchRecent_FiltersOnTheServer(t *testing.T) {
	c, requests := newTestClient(t)

	vs, err := c.FetchRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 4 {
		t.Fatalf("got %d votes, want 4", len(vs))
	}

	raw := (*requests)[0]
	// A "+" would reach the service as a literal plus and match nothing.
	if strings.Contains(raw, "+") {
		t.Errorf("query encodes spaces as '+': %s", raw)
	}
	filter := firstQueryValue(t, raw, "$filter")
	for _, want := range []string{
		"Language eq 'DE'",
		"startswith(tolower(Subject),'vote final') eq true",
		// The apostrophe is doubled, which is how OData escapes it.
		"startswith(tolower(Subject),'vote sur l''ensemble') eq true",
	} {
		if !strings.Contains(filter, want) {
			t.Errorf("filter %q lacks %q", filter, want)
		}
	}
}

func TestFetchRecent_MapsTheListing(t *testing.T) {
	c, _ := newTestClient(t)

	vs, err := c.FetchRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	v := vs[0]
	if v.SourceID != "38963" {
		t.Fatalf("first vote = %s, want the newest, 38963", v.SourceID)
	}
	if v.Body != "Nationalrat" || v.Jurisdiction != "nationalrat" {
		t.Errorf("body/jurisdiction = %q/%q", v.Body, v.Jurisdiction)
	}
	if v.Affair.Number != "26.033" || v.Affair.ID != "20260033" {
		t.Errorf("affair = %q/%q", v.Affair.Number, v.Affair.ID)
	}
	if strings.HasPrefix(v.Title, " ") {
		t.Errorf("title keeps the service's leading space: %q", v.Title)
	}
	// Abstimmungsprotokoll Ref. 33225 is stamped 09:40.
	want := time.Date(2026, 10, 2, 9, 40, 36, 907e6, zurich)
	if !v.Date.Equal(want) {
		t.Errorf("date = %v, want %v", v.Date, want)
	}
	if v.Date.Location() != zurich {
		t.Errorf("date is in %v, want Europe/Zurich", v.Date.Location())
	}
	if v.SourceURL != "https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=20260033" {
		t.Errorf("source URL = %q", v.SourceURL)
	}
	if v.Attribution != Attribution {
		t.Errorf("attribution = %q", v.Attribution)
	}
}

func TestGroupByAffair_TalliesMatchTheProtocol(t *testing.T) {
	c, _ := newTestClient(t)
	groups := fetchAndGroup(t, c)

	byID := make(map[string]votes.Vote)
	for _, g := range groups {
		for _, v := range g {
			byID[v.SourceID] = v
		}
	}

	// Abstimmungsprotokoll Ref. 33215, 33233 and 33225 for the other two.
	tests := []struct {
		id                    string
		ja, nein, enth, abw   int
		decision, affairType  string
		fraktion              string
		fJa, fNein, fEn, fAbw int
	}{
		{"38953", 156, 33, 7, 3, "angenommen", "Geschäft des Bundesrates", "SP", 36, 4, 1, 0},
		{"38954", 139, 52, 5, 3, "angenommen", "Geschäft des Bundesrates", "SVP", 10, 52, 4, 0},
		{"38971", 120, 70, 6, 3, "angenommen", "Geschäft des Bundesrates", "SP", 0, 41, 0, 0},
		{"38963", 116, 71, 9, 3, "angenommen", "Geschäft des Bundesrates", "Mitte-EVP", 27, 0, 3, 1},
	}
	for _, tt := range tests {
		v, ok := byID[tt.id]
		if !ok {
			t.Errorf("vote %s missing", tt.id)
			continue
		}
		if *v.Yes != tt.ja || *v.No != tt.nein || *v.Abstention != tt.enth || *v.Absent != tt.abw {
			t.Errorf("%s: %d/%d/%d/%d, want %d/%d/%d/%d", tt.id,
				*v.Yes, *v.No, *v.Abstention, *v.Absent, tt.ja, tt.nein, tt.enth, tt.abw)
		}
		// 200 seats, less the President, who does not vote.
		if len(v.MemberVotes) != 199 {
			t.Errorf("%s: %d member votes, want 199", tt.id, len(v.MemberVotes))
		}
		if !votes.IsBreakdownComplete(v) {
			t.Errorf("%s: breakdown incomplete", tt.id)
		}
		if v.Decision != tt.decision {
			t.Errorf("%s: decision %q, want %q", tt.id, v.Decision, tt.decision)
		}
		if v.Affair.Type != tt.affairType {
			t.Errorf("%s: affair type %q, want %q", tt.id, v.Affair.Type, tt.affairType)
		}
		got := map[string]int{}
		for _, m := range v.MemberVotes {
			if m.Fraktion == tt.fraktion {
				got[m.Choice]++
			}
		}
		if got["Ja"] != tt.fJa || got["Nein"] != tt.fNein || got["Enthaltung"] != tt.fEn || got["Abwesend"] != tt.fAbw {
			t.Errorf("%s %s: %v, want %d/%d/%d/%d", tt.id, tt.fraktion, got, tt.fJa, tt.fNein, tt.fEn, tt.fAbw)
		}
	}
}

func TestFetchRecent_LinksTheProtocol(t *testing.T) {
	c, _ := newTestClient(t)

	vs, err := c.FetchRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	// The PDF headed "Ref. 33225", fetched from this URL on 25.10.2026.
	want := "https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_33225.pdf"
	if vs[0].ArchiveURL != want {
		t.Errorf("archive URL = %q, want %q", vs[0].ArchiveURL, want)
	}
}

func TestToVote_NoLegislatureNoProtocol(t *testing.T) {
	c := New(testJurisdiction)
	v := c.toVote(voteDTO{ID: 1, RegistrationNumber: 33225, BusinessNumber: 20260033})
	// vote_0_33225.pdf does not exist, and a link to it would be a 404 in a post.
	if v.ArchiveURL != "" {
		t.Errorf("archive URL = %q, want none", v.ArchiveURL)
	}
}

func TestGroupByAffair_LabelsBillsAndUnusualMeanings(t *testing.T) {
	c, _ := newTestClient(t)
	groups := fetchAndGroup(t, c)

	subtitles := make(map[string]string)
	for _, g := range groups {
		for _, v := range g {
			subtitles[v.SourceID] = v.Subtitle
		}
	}

	want := map[string]string{
		// Each vote names its own bill. The first adopts the recommendation
		// to reject the initiative, so its Ja is spelled out.
		"38953": "Vote final: Bundesbeschluss über die Volksinitiative «Ja zum Importverbot für tierquälerisch erzeugte Pelzprodukte (Pelz-Initiative)» (Ja: Adopter le projet (recommandation de rejeter l'initiative))", //nolint:misspell // French, as the service serves it
		"38954": "Vote final: Tierschutzgesetz (TSchG)",
		// The service gives this one no bill.
		"38971": "Vote final",
		// A single-bill business does not repeat its title as a bill.
		"38963": "Vote final",
	}
	for id, w := range want {
		if subtitles[id] != w {
			t.Errorf("%s subtitle = %q, want %q", id, subtitles[id], w)
		}
	}
}

func TestGroupByAffair_LabelDoesNotDependOnSiblings(t *testing.T) {
	// 38954 alone, as it arrives when 38953 was posted by an earlier run or
	// fell outside the fetch window. It is still the counter-proposal.
	c, _ := newTestClient(t)
	vs, err := c.FetchRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	var lone []votes.Vote
	for _, v := range vs {
		if v.SourceID == "38954" {
			lone = append(lone, v)
		}
	}
	groups, err := c.GroupByAffair(lone)
	if err != nil {
		t.Fatal(err)
	}
	if got := groups[0][0].Subtitle; got != "Vote final: Tierschutzgesetz (TSchG)" {
		t.Errorf("lone subtitle = %q, want the bill named", got)
	}
}

func TestLabel_SingleBillBusinessDoesNotNameItsBill(t *testing.T) {
	// Vote 38971's bill is nameless; 38963's is a longer form of its title.
	// Neither belongs on the card, which is what posted on 25.10.2026 with
	// four lines of restated title above the subject.
	c, _ := newTestClient(t)
	groups := fetchAndGroup(t, c)
	for _, g := range groups {
		for _, v := range g {
			if v.SourceID == "38963" && strings.Contains(v.Subtitle, "Bundesbeschluss") {
				t.Errorf("single-bill vote carries its bill: %q", v.Subtitle)
			}
		}
	}
}

func TestLabel_UncountableBillsAreNamed(t *testing.T) {
	// A service that cannot say errs towards naming, so that two cards on one
	// business are never left looking the same.
	c := New(testJurisdiction)
	c.baseURL = "http://127.0.0.1:1"
	c.maxAttempts = 1
	c.labels["1"] = voteLabel{bill: "Tierschutzgesetz (TSchG)"}
	group := []votes.Vote{{SourceID: "1", Subtitle: "Vote final", Affair: votes.Affair{ID: "20250059", Title: "Pelzinitiative"}}}
	c.label(group)
	if got := group[0].Subtitle; got != "Vote final: Tierschutzgesetz (TSchG)" {
		t.Errorf("subtitle = %q", got)
	}
}

func TestLabel_BillRepeatingTheTitleIsLeftOut(t *testing.T) {
	c := New(testJurisdiction)
	c.labels["1"] = voteLabel{bill: "Raumfahrtgesetz", meaningYes: "Adopter le projet"}
	group := []votes.Vote{{SourceID: "1", Subtitle: "Vote sur l'ensemble", Affair: votes.Affair{Title: "Raumfahrtgesetz"}}}
	c.label(group)
	if got := group[0].Subtitle; got != "Vote sur l'ensemble" {
		t.Errorf("subtitle = %q", got)
	}
}

func TestGroupByAffair_EveryVoteIsPostable(t *testing.T) {
	c, _ := newTestClient(t)
	groups := fetchAndGroup(t, c)

	postable := voteposting.PostableGroups(groups)
	n := 0
	for _, g := range postable {
		n += len(g)
	}
	if n != 4 {
		t.Errorf("%d of 4 votes postable", n)
	}
}

// fullChamber returns one Ja row per seat, so a test can change the few it is
// about.
func fullChamber() []votingDTO {
	rows := make([]votingDTO, Seats)
	for i := range rows {
		rows[i] = votingDTO{FirstName: "A", LastName: strconv.Itoa(i), ParlGroupCode: "S", Decision: 1}
	}
	return rows
}

func TestApplyMemberVotes_RefusesAnUnknownDecision(t *testing.T) {
	v := votes.Vote{SourceID: "1", Type: "Normal"}
	rows := fullChamber()
	rows[1] = votingDTO{FirstName: "C", LastName: "D", ParlGroupCode: "V", Decision: 4, DecisionText: "?"}
	applyMemberVotes(&v, rows)
	if v.Type != "" {
		t.Errorf("type = %q, want empty so the pipeline refuses the vote", v.Type)
	}
	if v.Yes != nil {
		t.Error("totals were set for a vote that could not be counted")
	}
}

func TestApplyMemberVotes_RefusesAPartialSnapshot(t *testing.T) {
	// One row short of the chamber. Counted, it would still read as a clear
	// result with a verdict; nothing derived from the rows alone can tell.
	v := votes.Vote{SourceID: "1", Type: "Normal"}
	applyMemberVotes(&v, fullChamber()[:Seats-1])
	if v.Type != "" {
		t.Errorf("type = %q, want empty so the pipeline refuses the vote", v.Type)
	}
	if v.Yes != nil || v.Decision != "" {
		t.Errorf("a partial snapshot set totals or a verdict: yes=%v decision=%q", v.Yes, v.Decision)
	}
}

func TestApplyMemberVotes_FullChamberIsCounted(t *testing.T) {
	v := votes.Vote{SourceID: "1", Type: "Normal"}
	applyMemberVotes(&v, fullChamber())
	if v.Type != "Normal" || v.Yes == nil || *v.Yes != Seats {
		t.Errorf("type=%q yes=%v, want a counted vote", v.Type, v.Yes)
	}
}

func TestApplyMemberVotes_NoRowsIsRefused(t *testing.T) {
	v := votes.Vote{SourceID: "1", Type: "Normal"}
	applyMemberVotes(&v, nil)
	if v.Type != "" {
		t.Errorf("type = %q, want empty", v.Type)
	}
}

func TestDecisionFor_TieStatesNothing(t *testing.T) {
	if got := decisionFor(90, 90); got != "" {
		t.Errorf("tie = %q, want empty", got)
	}
	if got := decisionFor(91, 90); got != "angenommen" {
		t.Errorf("91:90 = %q", got)
	}
	if got := decisionFor(90, 91); got != "abgelehnt" {
		t.Errorf("90:91 = %q", got)
	}
}

func TestFraktionLabel(t *testing.T) {
	tests := []struct{ code, abbr, want string }{
		{"V", "Fraktion V", "SVP"},
		{"M-E", "Fraktion M-E", "Mitte-EVP"},
		// A group nobody has mapped keeps its official short name.
		{"X", "Fraktion X", "X"},
		{"X", "", "X"},
	}
	for _, tt := range tests {
		if got := fraktionLabel(tt.code, tt.abbr); got != tt.want {
			t.Errorf("fraktionLabel(%q, %q) = %q, want %q", tt.code, tt.abbr, got, tt.want)
		}
	}
}

func TestParseODataDate(t *testing.T) {
	// Abstimmungsprotokoll Ref. 33215: 02.10.2026, 09:29:18 in the chamber.
	want := time.Date(2026, 10, 2, 9, 29, 18, 735e6, zurich)
	for _, in := range []string{"/Date(1790933358735)/", "/Date(1790933358735+0120)/"} {
		if got := parseODataDate(in); !got.Equal(want) {
			t.Errorf("parseODataDate(%q) = %v, want %v", in, got, want)
		}
	}
	// Winter time: the same wall clock, whatever the offset.
	if got := parseODataDate("/Date(1764581400000)/"); got.Hour() != 9 || got.Minute() != 30 {
		t.Errorf("winter date = %v, want 09:30 local", got)
	}
	for _, in := range []string{"", "2026-10-02", "/Date(x)/", "/Date()/"} {
		if got := parseODataDate(in); !got.IsZero() {
			t.Errorf("parseODataDate(%q) = %v, want zero", in, got)
		}
	}
}

func fetchAndGroup(t *testing.T, c *Client) [][]votes.Vote {
	t.Helper()
	vs, err := c.FetchRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := c.GroupByAffair(vs)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

// firstQueryValue decodes one parameter from a raw query string.
func firstQueryValue(t *testing.T, raw, key string) string {
	t.Helper()
	for _, part := range strings.Split(raw, "&") {
		k, v, _ := strings.Cut(part, "=")
		if k == key || k == strings.ReplaceAll(key, "$", "%24") {
			dec, err := url.PathUnescape(v)
			if err != nil {
				t.Fatal(err)
			}
			return dec
		}
	}
	t.Fatalf("no %s in %s", key, raw)
	return ""
}

func TestFetchMembers_NamesMatchTheVoteRows(t *testing.T) {
	c, _ := newTestClient(t)
	names, err := c.SittingNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != Seats {
		t.Fatalf("%d sitting members, want %d", len(names), Seats)
	}

	// The tagger matches a contact's name against post text, and the post
	// text comes from the vote rows. A roster that named people differently
	// would fill the contacts file with names that never match.
	sitting := make(map[string]bool, len(names))
	for _, n := range names {
		sitting[n] = true
	}
	groups := fetchAndGroup(t, c)
	for _, m := range groups[len(groups)-1][0].MemberVotes {
		if !sitting[m.Name] {
			t.Errorf("vote row %q is not in the roster", m.Name)
		}
	}
}

func TestFetchMembers_MapsTheRow(t *testing.T) {
	c, _ := newTestClient(t)
	members, err := c.FetchMembers()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.Name != "Gerhard Pfister" {
			continue
		}
		if m.Party != "Die Mitte" || m.Fraktion != "Mitte-EVP" {
			t.Errorf("party/fraktion = %q/%q", m.Party, m.Fraktion)
		}
		if m.ProfileURL != "https://www.parlament.ch/de/biografie?CouncillorId=1109" {
			t.Errorf("profile = %q", m.ProfileURL)
		}
		return
	}
	t.Error("Gerhard Pfister not in the roster")
}

// The roster is the chamber's and no other: a Ständerat contacts file seeded
// from the Nationalrat would hold 200 people who never sit there.
func TestFetchMembers_AsksForItsCouncil(t *testing.T) {
	for _, tc := range []struct {
		council int
		want    string
	}{
		{CouncilNationalrat, "Council eq 1"},
		{CouncilStaenderat, "Council eq 2"},
	} {
		c, requests := newTestClient(t)
		c.WithCouncil(tc.council)
		if _, err := c.FetchMembers(); err != nil {
			t.Fatal(err)
		}
		filter := firstQueryValue(t, (*requests)[len(*requests)-1], "$filter")
		if !strings.HasSuffix(filter, tc.want) {
			t.Errorf("council %d: filter = %q, want it to end in %q", tc.council, filter, tc.want)
		}
	}
}
