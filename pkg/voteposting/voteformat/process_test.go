package voteformat

import (
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestNextStepLine(t *testing.T) {
	group := func(p *votes.Process) []votes.Vote {
		v := votes.Vote{Type: "Normal"}
		v.Affair.Process = p
		return []votes.Vote{v}
	}
	open := &votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Nationalrat", State: votes.StepCurrent},
			{Label: "Ständerat", State: votes.StepNext},
		},
		Next: " In Kommission des Ständerats ",
	}
	ended := &votes.Process{
		Steps:       []votes.ProcessStep{{Label: "Nationalrat", State: votes.StepEnded}},
		Next:        "Erledigt",
		StatusLabel: "Stand der Beratungen:",
	}

	tests := []struct {
		name  string
		group []votes.Vote
		want  string
	}{
		{"no process", group(nil), ""},
		{"open business", group(open), "➡️ Nächster Schritt: In Kommission des Ständerats"},
		{"ended business", group(ended), "🏁 Stand der Beratungen: Erledigt"},
		{"next step not known", group(&votes.Process{Steps: []votes.ProcessStep{{Label: "Nationalrat", State: votes.StepCurrent}}}), ""},
		{"a line in place of the next step", group(&votes.Process{
			Steps:       []votes.ProcessStep{{Label: "Nationalrat", State: votes.StepCurrent}},
			StatusLabel: "Noch nicht endgültig:",
			Next:        "Schlussabstimmung beider Räte steht aus",
		}), "⏳ Noch nicht endgültig: Schlussabstimmung beider Räte steht aus"},
		{"invalid process", group(&votes.Process{Next: "In Kommission des Ständerats"}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextStepLine(tt.group); got != tt.want {
				t.Errorf("NextStepLine = %q, want %q", got, tt.want)
			}
		})
	}
}
