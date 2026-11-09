package zurichapi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The Geschäft fixtures are PARIS's records of three Postulate, recorded on
// 08.11.2026. They hold steps recorded after the vote each test posts, which
// the adapter has to leave out.
func recordedGeschaeft(t *testing.T, grNr string) processGeschaeft {
	t.Helper()
	name := filepath.Join("testdata", "process", "geschaeft_"+grNr[:4]+"_"+grNr[5:]+".xml")
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	g, err := parseProcessGeschaeft(body, grNr)
	if err != nil || g == nil {
		t.Fatalf("parsing %s: %v, %v", name, g, err)
	}
	return *g
}

func schlussabstimmung(grNr, decision string, day time.Time) votes.Vote {
	return votes.Vote{
		Jurisdiction: JurisdictionKey, Date: day, Decision: decision,
		Subtitle: grNr[:4] + "_0" + grNr[5:] + " Schlussabstimmung",
		Affair:   votes.Affair{Number: grNr},
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestProcessFor_RecordedPostulate(t *testing.T) {
	tests := []struct {
		grNr     string
		decision string
		day      time.Time
		want     votes.Process
	}{
		{"2026/351", "Ja", day(2026, 10, 28), votes.Process{
			Steps: []votes.ProcessStep{
				{Label: "Eingang, Frist 3 Monate", Detail: "01.07.2026", State: votes.StepDone},
				{Label: "Stadtrat, Entgegennahme", Detail: "08.07.2026", State: votes.StepDone},
				{Label: "Ablehnung, beantragt", Detail: "19.08.2026", State: votes.StepDone},
				{Label: "Gemeinderat", Detail: "28.10.2026 · Überweisung", State: votes.StepCurrent},
				{Label: "Stadtrat, Weisung", Detail: "Frist 24 Monate", State: votes.StepNext},
			},
			Next: "Stadtrat, Weisung, Frist 24 Monate",
		}},
		{"2026/303", "Nein", day(2026, 10, 28), votes.Process{
			Steps: []votes.ProcessStep{
				{Label: "Eingang, Frist 3 Monate", Detail: "17.06.2026", State: votes.StepDone},
				{Label: "Stadtrat, Entgegennahme", Detail: "08.07.2026", State: votes.StepDone},
				{Label: "Ablehnung, beantragt", Detail: "19.08.2026", State: votes.StepDone},
				{Label: "Gemeinderat", Detail: "28.10.2026 · Ablehnung", State: votes.StepEnded},
			},
			Next: "Abgeschlossen",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.grNr, func(t *testing.T) {
			got, _ := processFor(schlussabstimmung(tt.grNr, tt.decision, tt.day), recordedGeschaeft(t, tt.grNr))
			if got == nil {
				t.Fatal("no process")
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("invalid: %v", err)
			}
			if !got.Equal(&tt.want) {
				t.Errorf("process =\n%+v\nwant\n%+v", *got, tt.want)
			}
		})
	}
}

func motion(steps ...string) processGeschaeft {
	g := processGeschaeft{GRNr: "2026/1", Art: "Motion"}
	for i, name := range steps {
		s := processStep{Name: name}
		s.Datum.Start = time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.000")
		g.Steps = append(g.Steps, s)
	}
	return g
}

// A Motion's Ja is its Überweisung only where the Stadtrat would take it as a
// Motion; where it would take it as a Postulat, the same Ja was as often its
// Umwandlung, so no process is said. Nein is always the Ablehnung.
func TestProcessFor_Motion(t *testing.T) {
	v := func(decision string) votes.Vote { return schlussabstimmung("2026/1", decision, day(2026, 3, 1)) }

	if p, _ := processFor(v("Ja"), motion("Eingang, Frist 6 Monate", "Stadtrat, Entgegennahme", "Ablehnung, beantragt")); p == nil || p.Next != "Stadtrat, Weisung, Frist 24 Monate" {
		t.Errorf("contested motion the Stadtrat takes: %+v", p)
	}
	if p, _ := processFor(v("Ja"), motion("Eingang, Frist 6 Monate", "Stadtrat, Entgegennahme als Postulat")); p != nil {
		t.Errorf("motion the Stadtrat takes only as a Postulat got %+v", p)
	}
	if p, _ := processFor(v("Ja"), motion("Eingang, Frist 6 Monate", "Stadtrat, Ablehnung")); p != nil {
		t.Errorf("motion the Stadtrat rejects got %+v", p)
	}
	if p, _ := processFor(v("Nein"), motion("Eingang, Frist 6 Monate", "Stadtrat, Entgegennahme als Postulat")); p == nil || p.Next != "Abgeschlossen" {
		t.Errorf("rejected motion: %+v", p)
	}
}

func TestProcessFor_NoRule(t *testing.T) {
	v := schlussabstimmung("2026/1", "Ja", day(2026, 3, 1))
	weisung := motion("Stadtrat, Weisung", "Überweisung, SK FD (Sachkommission Finanzdepartement)")
	weisung.Art = "Weisung"
	if p, why := processFor(v, weisung); p != nil || !strings.Contains(why, "Zustimmung, fakultatives Referendum") {
		t.Errorf("Weisung got %+v, reason %q", p, why)
	}
	changed := motion("Eingang, Frist 3 Monate", "Textänderung, beantragt")
	changed.Art = "Postulat"
	if p, _ := processFor(v, changed); p != nil {
		t.Errorf("Postulat with a Textänderung got %+v", p)
	}
	unknown := schlussabstimmung("2026/1", "", day(2026, 3, 1))
	postulat := motion("Eingang, Frist 3 Monate")
	postulat.Art = "Postulat"
	if p, _ := processFor(unknown, postulat); p != nil {
		t.Errorf("vote without a Schlussresultat got %+v", p)
	}
}

// Steps of the voting day itself lead up to the vote unless they are its
// result: on 2026/392 the Stadtrat answered on the day of the vote.
func TestProcessFor_SameDaySteps(t *testing.T) {
	g := processGeschaeft{GRNr: "2026/392", Art: "Postulat"}
	for _, s := range []struct{ name, start, text string }{
		{"Eingang, Frist 3 Monate", "2026-07-08T00:00:00.000", "08.07.2026"},
		{"Stadtrat, Entgegennahme", "2026-08-26T00:00:00.000", "26.08.2026"},
		{"Ablehnung", "2026-08-26T00:00:00.000", "26.08.2026"},
	} {
		st := processStep{Name: s.name}
		st.Datum.Start, st.Datum.Text = s.start, s.text
		g.Steps = append(g.Steps, st)
	}
	p, _ := processFor(schlussabstimmung("2026/392", "Nein", day(2026, 8, 26)), g)
	if p == nil {
		t.Fatal("no process")
	}
	var labels []string
	for _, s := range p.Steps {
		labels = append(labels, s.Label+" "+s.Detail)
	}
	want := []string{"Eingang, Frist 3 Monate 08.07.2026", "Stadtrat, Entgegennahme 26.08.2026", "Gemeinderat 26.08.2026 · Ablehnung"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("steps = %q, want %q", labels, want)
	}
}

// A vote without a date gets no process: nothing says which steps came
// before it.
func TestProcessFor_NoDate(t *testing.T) {
	g := motion("Eingang, Frist 3 Monate", "Textänderung, beantragt")
	g.Art = "Postulat"
	if p, _ := processFor(schlussabstimmung("2026/1", "Nein", time.Time{}), g); p != nil {
		t.Errorf("undated vote got %+v", p)
	}
}

// Only the vote that decides the business is a Schlussabstimmung here: PARIS
// titles a deadline extension "… Schlussabstimmung Fristverlängerung", and its
// Ja is no Überweisung.
func TestProcess_OnlyTheDecidingVote(t *testing.T) {
	c := NewClient()
	for _, subtitle := range []string{"2021_0183 Schlussabstimmung Fristverlängerung 2", "2026_0098 Dringlicherklärung", "Fristverlängerung Schlussabstimmung"} {
		v := schlussabstimmung("2026/1", "Ja", day(2026, 3, 1))
		v.Subtitle = subtitle
		p, err := c.Process([]votes.Vote{v})
		var none votes.NoProcess
		if p != nil || !errors.As(err, &none) {
			t.Errorf("%q: Process = %+v, %v; want no process", subtitle, p, err)
		}
	}
}
