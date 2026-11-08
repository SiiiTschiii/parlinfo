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
		Next: " Der Ständerat entscheidet. ",
	}
	ended := &votes.Process{Steps: []votes.ProcessStep{
		{Label: "Nationalrat", State: votes.StepEnded},
	}}

	tests := []struct {
		name  string
		group []votes.Vote
		want  string
	}{
		{"no process", group(nil), ""},
		{"open business", group(open), "➡️ Als Nächstes: Der Ständerat entscheidet."},
		{"ended business", group(ended), "🏁 Das Geschäft ist erledigt."},
		{"invalid process", group(&votes.Process{Next: "Der Ständerat entscheidet."}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextStepLine(tt.group); got != tt.want {
				t.Errorf("NextStepLine = %q, want %q", got, tt.want)
			}
		})
	}
}
