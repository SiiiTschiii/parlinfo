package parlamentch

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The process fixtures are the service's responses for six businesses,
// recorded on 09.11.2026 with only the fields the adapter reads: the German
// feed's in testdata/process, the French and Italian feeds' in its fr and it
// folders. They hold the businesses' whole histories, including what came
// after the vote each test posts, which the adapter has to leave out.
var (
	processBusiness = regexp.MustCompile(`(?:ID|BusinessNumber) eq (\d+)`)
	processBill     = regexp.MustCompile(`IdBill eq guid'([0-9a-f-]+)'`)
	processLanguage = regexp.MustCompile(`Language eq '([A-Z]{2})'`)
)

func newProcessClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("$filter")
		entity := r.URL.Path[1:]
		if entity == "Vote/$count" {
			// None of the recorded businesses has a second bill.
			_, _ = w.Write([]byte("0"))
			return
		}
		dir := filepath.Join("testdata", "process")
		if m := processLanguage.FindStringSubmatch(filter); m != nil && m[1] != "DE" {
			dir = filepath.Join(dir, strings.ToLower(m[1]))
		}
		var name string
		if entity == "Resolution" {
			m := processBill.FindStringSubmatch(filter)
			if m == nil {
				http.NotFound(w, r)
				return
			}
			matches, _ := filepath.Glob(filepath.Join(dir, "*_Resolution_"+m[1]+".json"))
			if len(matches) != 1 {
				http.NotFound(w, r)
				return
			}
			name = matches[0]
		} else {
			m := processBusiness.FindStringSubmatch(filter)
			if m == nil {
				http.NotFound(w, r)
				return
			}
			name = filepath.Join(dir, m[1]+"_"+entity+".json")
		}
		data, err := os.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	c := New(testJurisdiction)
	c.SetBaseURL(srv.URL)
	c.retryDelay = 0
	return c
}

// processVote is a vote as the client hands it on after labelling it, with
// the meanings the service served for it.
func processVote(c *Client, sourceID, businessID, businessType, subtitle, decision, outcome, meaningYes, meaningNo string, at time.Time) votes.Vote {
	c.labels[sourceID] = voteLabel{meaningYes: meaningYes, meaningNo: meaningNo}
	return votes.Vote{
		SourceID: sourceID, Jurisdiction: "nationalrat", Body: "Nationalrat",
		Date: at, Subtitle: subtitle, Decision: decision, Outcome: outcome,
		Affair: votes.Affair{ID: businessID, Type: businessType},
	}
}

func swiss(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, zurich)
}

func TestProcess_RealBusinesses(t *testing.T) {
	c := newProcessClient(t)

	tests := []struct {
		name  string
		vote  votes.Vote
		steps []votes.ProcessStep
		next  string
	}{
		{
			// Adopted as first chamber against the Bundesrat; the Ständerat's
			// committee has since been named, and the business was still with
			// it when recorded.
			name: "motion adopted as first chamber",
			vote: processVote(c, "32084", "20243004", typeMotion, "", "angenommen", outcomeAdopted,
				"Antrag der Mehrheit (Annahme der Motion)", "Antrag der Minderheit Weichelt und des Bundesrates (Ablehnung der Motion)",
				swiss(2024, 3, 7, 12, 51)),
			steps: []votes.ProcessStep{
				{Label: "Eingereicht", Detail: "18.01.2024", State: votes.StepDone},
				{Label: "Antrag des Bundesrates", Detail: "21.02.2024 · Ablehnung", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "07.03.2024 · Annahme", State: votes.StepCurrent},
				{Label: "In Kommission des Ständerats", Detail: "Kommission für soziale Sicherheit und Gesundheit Ständerat (SGK-S)", State: votes.StepNext},
			},
			next: "In Kommission des Ständerats (SGK-S)",
		},
		{
			name: "motion adopted as second chamber",
			vote: processVote(c, "36159", "20254179", typeMotion, "", "angenommen", outcomeAdopted,
				"Proposition de la majorité (adopter la motion)", "Proposition de la minorité Ryser et du Conseil fédéral (rejeter la motion)",
				swiss(2026, 6, 1, 18, 38)),
			next: "Überwiesen an den Bundesrat",
		},
		{
			name: "initiative given no leave",
			vote: processVote(c, "38790", "20250408", typeParlIv, "", "angenommen", outcomeNoLeave,
				"Proposition de la majorité (ne pas donner suite)", "Proposition de la minorité (donner suite)",
				swiss(2026, 9, 24, 17, 15)),
			next: "Erledigt",
		},
		{
			// 24.065 has two bills; the vote names its own, and the history
			// is that bill's chronology, as the business page lists it under
			// Entwurf 1, after the business's filing: the Ständerat's
			// Schlussabstimmung of the same day included.
			name: "Schlussabstimmung adopted",
			vote: onBill(c, processVote(c, "36539", "20240065", typeFederalCouncil, "Vote final", "angenommen", "",
				"Adopter le projet", "Rejeter le projet", swiss(2026, 6, 19, 10, 8)), 1),
			steps: []votes.ProcessStep{
				{Label: "Eingereicht", Detail: "14.08.2024", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "16.09.2025 · Beschluss abweichend vom Entwurf", State: votes.StepDone},
				{Label: "Ständerat", Detail: "08.06.2026 · Abweichung", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "11.06.2026 · Zustimmung", State: votes.StepDone},
				{Label: "Ständerat", Detail: "19.06.2026 · Annahme in der Schlussabstimmung", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "19.06.2026 · Annahme in der Schlussabstimmung", State: votes.StepCompleted},
			},
			next: "Erledigt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := c.Process([]votes.Vote{tt.vote})
			if err != nil {
				t.Fatal(err)
			}
			if p == nil {
				t.Fatal("no process")
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("invalid process %+v: %v", p, err)
			}
			if p.Next != tt.next {
				t.Errorf("Next = %q, want %q", p.Next, tt.next)
			}
			if tt.steps != nil && !(&votes.Process{Steps: p.Steps, Next: p.Next}).Equal(&votes.Process{Steps: tt.steps, Next: tt.next}) {
				t.Errorf("steps =\n%+v\nwant\n%+v", p.Steps, tt.steps)
			}
			for _, s := range p.Steps {
				if s.State == votes.StepDone && s.Label == "Nationalrat" && strings.HasPrefix(s.Detail, tt.vote.Date.Format("02.01.2006")) {
					t.Errorf("a decision taken on the vote's own day or later is listed as history: %+v", s)
				}
			}
		})
	}
}

// The second-chamber motion lists the Ständerat's decision before it, which
// is how the adapter knows the Nationalrat was second.
func TestProcess_SecondChamberNamesTheFirst(t *testing.T) {
	c := newProcessClient(t)
	v := processVote(c, "36159", "20254179", typeMotion, "", "angenommen", outcomeAdopted,
		"Proposition de la majorité (adopter la motion)", "Proposition de la minorité Ryser et du Conseil fédéral (rejeter la motion)",
		swiss(2026, 6, 1, 18, 38))
	p, err := c.Process([]votes.Vote{v})
	if err != nil || p == nil {
		t.Fatalf("process = %v, %v", p, err)
	}
	var sawStaenderat bool
	for _, s := range p.Steps {
		if s.Label == "Ständerat" && s.State == votes.StepDone {
			sawStaenderat = true
		}
	}
	if !sawStaenderat {
		t.Errorf("no Ständerat decision in %+v", p.Steps)
	}
}

// What has no rule gets no process, and asks the service nothing.
func TestProcess_NoRuleNoProcess(t *testing.T) {
	c := newProcessClient(t)

	cases := map[string][]votes.Vote{
		"amended motion": {processVote(c, "a", "20253007", typeMotion, "", "angenommen", outcomeAdopted,
			"Proposition de la majorité (adopter la motion modifiée)", "Proposition de la minorité (rejeter la motion)", swiss(2026, 9, 14, 18, 1))},
		"initiative given leave": {processVote(c, "b", "20264000", typeParlIv, "", "angenommen", outcomeLeave,
			"donner suite", "ne pas donner suite", swiss(2026, 9, 14, 18, 1))},
		"Gesamtabstimmung": {processVote(c, "c", "20260033", typeFederalCouncil, "Vote sur l'ensemble", "angenommen", "",
			"Adopter le projet", "Rejeter le projet", swiss(2026, 9, 14, 18, 1))},
		"final vote whose Ja is not adopting the bill": {processVote(c, "d", "20250059", typeFederalCouncil, "Vote final", "angenommen", "",
			"Recommander de rejeter l'initiative", "Rejeter le projet", swiss(2026, 9, 14, 18, 1))},
		"no readable outcome": {processVote(c, "e", "20264001", typeMotion, "", "", "",
			"Adopter le point 1", "Rejeter le point 1", swiss(2026, 9, 14, 18, 1))},
		"unknown type": {processVote(c, "f", "20264002", "Petition", "", "angenommen", "",
			"", "", swiss(2026, 9, 14, 18, 1))},
		"no vote date": {processVote(c, "g", "20243004", typeMotion, "", "angenommen", outcomeAdopted,
			"Antrag der Mehrheit (Annahme der Motion)", "Antrag des Bundesrates (Ablehnung der Motion)", time.Time{})},
	}
	for name, group := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := c.Process(group)
			var none votes.NoProcess
			if p != nil || !errors.As(err, &none) || none.Reason == "" {
				t.Errorf("Process = %+v, %v; want no process, with a reason", p, err)
			}
		})
	}
}

// germanWords are words of the German process that a French or Italian one
// must not carry.
var germanWords = regexp.MustCompile(`Nationalrat|Ständerat|Kommission|Eingereicht|Erledigt|Antrag|Annahme|Ablehnung|Folge|Überwiesen|Bundesrat|Entwurf|Beratungen`)

// A French or Italian post gets the process the German one does, in the words
// of that language's feed and of parlament.ch's pages in it. The rules read
// the service's codes, so the businesses are the German tests' own, served
// by the French and Italian feeds, with the type codes enrich caches.
func TestProcess_FrenchAndItalian(t *testing.T) {
	type want struct {
		steps  []string // "state label | detail"
		next   string
		status string
		about  string
	}
	vote := func(c *Client, id string) votes.Vote {
		switch id {
		case "20243004":
			return processVote(c, "32084", id, "", "", "angenommen", outcomeAdopted,
				"Antrag der Mehrheit (Annahme der Motion)", "Antrag der Minderheit Weichelt und des Bundesrates (Ablehnung der Motion)", swiss(2024, 3, 7, 12, 51))
		case "20254179":
			return processVote(c, "36159", id, "", "", "angenommen", outcomeAdopted,
				"Proposition de la majorité (adopter la motion)", "Proposition de la minorité Ryser et du Conseil fédéral (rejeter la motion)", swiss(2026, 6, 1, 18, 38))
		case "20200454":
			return processVote(c, "x", id, "", "", "angenommen", outcomeNoLeave,
				"Antrag der Minderheit (keine Folge geben)", "Antrag der Mehrheit (Folge geben)", swiss(2023, 2, 27, 18, 44))
		case "20250301":
			return processVote(c, "35999", id, "", "", "", "", "", "", swiss(2026, 4, 28, 9, 21))
		}
		t.Fatalf("no vote for %s", id)
		return votes.Vote{}
	}
	codes := map[int64]int{20243004: 5, 20254179: 5, 20200454: 4, 20250301: 3, 20240065: 1}

	tests := []struct {
		lang     votes.Language
		business string
		subtitle string // a final vote's, as the post labels it
		want     want
	}{
		{votes.French, "20243004", "", want{
			steps: []string{
				"0 Déposé | 18.01.2024",
				"0 Proposition du Conseil fédéral | 21.02.2024 · Rejet",
				"1 Conseil national | 07.03.2024 · Adoption",
				"3 En commission du Conseil des Etats | Commission de la sécurité sociale et de la santé publique Conseil des États (CSSS-E)",
			},
			next:  "En commission du Conseil des Etats (CSSS-E)",
			about: "La motion charge le Conseil fédéral de déposer un projet d’acte de l’Assemblée fédérale ou de prendre une mesure.",
		}},
		{votes.Italian, "20243004", "", want{
			steps: []string{
				"0 Depositato | 18.01.2024",
				"0 Proposta del Consiglio federale | 21.02.2024 · Respingere",
				"1 Consiglio nazionale | 07.03.2024 · Adozione",
				"3 Nella Commissione del Consiglio degli Stati | Commissione della sicurezza sociale e della sanità Consiglio degli Stati (CSSS-S)",
			},
			next:  "Nella Commissione del Consiglio degli Stati (CSSS-S)",
			about: "La mozione incarica il Consiglio federale di presentare un disegno di atto legislativo dell’Assemblea federale o di prendere un provvedimento.", //nolint:misspell // Italian, as parlament.ch words it
		}},
		{votes.French, "20254179", "", want{next: "Transmis au Conseil fédéral"}},
		{votes.Italian, "20254179", "", want{next: "Trasmesso al Consiglio federale"}},
		{votes.French, "20200454", "", want{
			steps: []string{
				"0 Déposé | 19.06.2020",
				"0 Commission CSEC-N | 01.07.2021 · Donner suite (conseil prioritaire)",
				"0 Commission CSEC-E | 16.11.2021 · Ne pas donner suite",
				"2 Conseil national | 27.02.2023 · Refusé de donner suite",
			},
			next: "Liquidé", status: "Etat des délibérations:",
		}},
		{votes.Italian, "20250301", "", want{
			steps: []string{
				"0 Depositato | 28.01.2025",
				"0 Consiglio degli Stati | 16.06.2025 · Non è dato seguito",
				"2 Consiglio nazionale | 28.04.2026 · Non è dato seguito",
			},
			next: "Liquidato", status: "Stato delle deliberazioni:",
			about: "Ogni Cantone può proporre mediante un’iniziativa che una commissione elabori un progetto di atto legislativo dell'Assemblea federale.", //nolint:misspell // Italian, as parlament.ch words it
		}},
		{votes.French, "20240065", "Vote final", want{next: "Liquidé", status: "Etat des délibérations:"}},
		{votes.Italian, "20240065", "Votazione finale", want{next: "Liquidato", status: "Stato delle deliberazioni:"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang)+" "+tt.business, func(t *testing.T) {
			c := newProcessClient(t)
			c.jurisdiction.Language = tt.lang
			for id, code := range codes {
				c.businessCodes[id] = code
			}
			var v votes.Vote
			if tt.subtitle != "" {
				v = onBill(c, processVote(c, "36539", tt.business, "", tt.subtitle, "angenommen", "",
					"Adopter le projet", "Rejeter le projet", swiss(2026, 6, 19, 10, 8)), 1)
			} else {
				v = vote(c, tt.business)
			}
			p, err := c.Process([]votes.Vote{v})
			if err != nil || p == nil {
				t.Fatalf("Process = %+v, %v", p, err)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("invalid process %+v: %v", p, err)
			}
			var got []string
			for _, s := range p.Steps {
				got = append(got, strconv.Itoa(int(s.State))+" "+s.Label+" | "+s.Detail)
				if germanWords.MatchString(s.Label + " " + s.Detail) {
					t.Errorf("German in step %+v", s)
				}
			}
			if tt.want.steps != nil && strings.Join(got, "\n") != strings.Join(tt.want.steps, "\n") {
				t.Errorf("steps =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want.steps, "\n"))
			}
			if p.Next != tt.want.next || p.StatusLabel != tt.want.status {
				t.Errorf("status %q next %q, want %q %q", p.StatusLabel, p.Next, tt.want.status, tt.want.next)
			}
			if tt.want.about != "" && p.About != tt.want.about {
				t.Errorf("About = %q, want %q", p.About, tt.want.about)
			}
			if germanWords.MatchString(p.Next + " " + p.StatusLabel + " " + p.Bill + " " + p.About) {
				t.Errorf("German in %+v", p)
			}
		})
	}
}

// onBill records which bill a vote was on, as the listing serves it.
func onBill(c *Client, v votes.Vote, bill int64) votes.Vote {
	l := c.labels[v.SourceID]
	l.billNumber = bill
	c.labels[v.SourceID] = l
	return v
}

// A business with several bills gets no process where the vote does not say
// which it was on: its decisions are filed per bill, and a card merging them
// read as one bill rejected and then adopted. 24.065 has two bills.
func TestProcess_SeveralBillsNoProcess(t *testing.T) {
	c := newProcessClient(t)
	v := processVote(c, "36539", "20240065", typeFederalCouncil, "Vote final", "angenommen", "",
		"Adopter le projet", "Rejeter le projet", swiss(2026, 6, 19, 10, 8))
	var none votes.NoProcess
	if p, err := c.Process([]votes.Vote{v}); p != nil || !errors.As(err, &none) {
		t.Errorf("Process = %+v, %v; want no process, with a reason", p, err)
	}
}

// A vote whose meanings the service does not serve takes the decision it
// records for the Nationalrat that day, once it has: 25.301, recorded here
// after the decision was.
func TestProcess_RecordedOutcome(t *testing.T) {
	c := newProcessClient(t)
	v := processVote(c, "35999", "20250301", typeStandesIv, "", "", "", "", "", swiss(2026, 4, 28, 9, 21))
	p, err := c.Process([]votes.Vote{v})
	if err != nil || p == nil {
		t.Fatalf("Process = %+v, %v", p, err)
	}
	last := p.Steps[len(p.Steps)-1]
	want := votes.ProcessStep{Label: "Nationalrat", Detail: "28.04.2026 · Keine Folge gegeben", State: votes.StepEnded}
	if last != want {
		t.Errorf("this vote's step = %+v, want %+v", last, want)
	}
	if p.Next != "Erledigt" || p.StatusLabel != "Stand der Beratungen:" {
		t.Errorf("status = %q %q", p.StatusLabel, p.Next)
	}
	if p.About != typeAbout[votes.German][typeStandesIv] {
		t.Errorf("About = %q", p.About)
	}
}

// The committees' decisions on an initiative are steps of its history, as the
// business page lists them, labelled by the committee: 20.454.
func TestProcess_CommitteeDecisions(t *testing.T) {
	c := newProcessClient(t)
	v := processVote(c, "x", "20200454", typeParlIv, "", "angenommen", outcomeNoLeave,
		"Antrag der Minderheit (keine Folge geben)", "Antrag der Mehrheit (Folge geben)", swiss(2023, 2, 27, 18, 44))
	p, err := c.Process([]votes.Vote{v})
	if err != nil || p == nil {
		t.Fatalf("Process = %+v, %v", p, err)
	}
	var got []string
	for _, s := range p.Steps {
		got = append(got, s.Label+" | "+s.Detail)
	}
	want := []string{
		"Eingereicht | 19.06.2020",
		"Kommission WBK-N | 01.07.2021 · Folge geben (Erstrat)",
		"Kommission WBK-S | 16.11.2021 · Keine Zustimmung",
		"Nationalrat | 27.02.2023 · Keine Folge gegeben",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("steps =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
