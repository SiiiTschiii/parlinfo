package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The process fixtures are the service's responses for four businesses,
// recorded on 08.11.2026 with __metadata stripped. They hold the businesses'
// whole histories, including what came after the vote each test posts, which
// the adapter has to leave out.
var (
	processBusiness = regexp.MustCompile(`(?:ID|BusinessNumber) eq (\d+)`)
	processBill     = regexp.MustCompile(`IdBill eq guid'([0-9a-f-]+)'`)
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
		var name string
		if entity == "Resolution" {
			m := processBill.FindStringSubmatch(filter)
			if m == nil {
				http.NotFound(w, r)
				return
			}
			matches, _ := filepath.Glob(filepath.Join("testdata", "process", "*_Resolution_"+m[1]+".json"))
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
			name = filepath.Join("testdata", "process", m[1]+"_"+entity+".json")
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
				{Label: "Stellungnahme zum Vorstoss liegt vor", Detail: "Antrag des Bundesrates: Ablehnung", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "Annahme", State: votes.StepCurrent},
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
			next: processDone,
		},
		{
			// 24.065 has two bills; the vote names its own, and the history
			// is that bill's alone.
			name: "Schlussabstimmung adopted",
			vote: onBill(c, processVote(c, "36539", "20240065", typeFederalCouncil, "Vote final", "angenommen", "",
				"Adopter le projet", "Rejeter le projet", swiss(2026, 6, 19, 10, 8)), 1),
			steps: []votes.ProcessStep{
				{Label: "Eingereicht", Detail: "14.08.2024", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "Beschluss abweichend vom Entwurf", State: votes.StepDone},
				{Label: "Ständerat", Detail: "Abweichung", State: votes.StepDone},
				{Label: "Nationalrat", Detail: "Annahme in der Schlussabstimmung", State: votes.StepCompleted},
			},
			next: processDone,
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
				if s.State == votes.StepDone && s.Label == "Nationalrat" && s.Detail == "Annahme" {
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
	}
	for name, group := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := c.Process(group)
			if p != nil || err != nil {
				t.Errorf("Process = %+v, %v; want nothing", p, err)
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
	if p, err := c.Process([]votes.Vote{v}); p != nil || err != nil {
		t.Errorf("Process = %+v, %v; want nothing", p, err)
	}
}
