package votes

import (
	"strings"
	"testing"
)

func open() Process {
	return Process{
		Steps: []ProcessStep{
			{Label: "Eingereicht", State: StepDone},
			{Label: "Nationalrat", Detail: "Angenommen", State: StepCurrent},
			{Label: "Ständerat", State: StepNext},
		},
		Next: "Der Ständerat entscheidet.",
	}
}

func ended() Process {
	return Process{Steps: []ProcessStep{
		{Label: "Eingereicht", State: StepDone},
		{Label: "Nationalrat", Detail: "Abgelehnt", State: StepEnded},
	}}
}

func TestProcessValidate(t *testing.T) {
	tests := []struct {
		name  string
		p     func() Process
		valid bool
	}{
		{"open business", open, true},
		{"ended business", ended, true},
		{"no steps", func() Process { return Process{Next: "x"} }, false},
		{"too many steps", func() Process {
			p := open()
			for len(p.Steps) <= MaxProcessSteps {
				p.Steps = append(p.Steps, ProcessStep{Label: "Kommission", State: StepNext})
			}
			return p
		}, false},
		{"no step for this vote", func() Process {
			p := open()
			p.Steps[1].State = StepDone
			return p
		}, false},
		{"two steps for this vote", func() Process {
			p := open()
			p.Steps[2].State = StepCurrent
			return p
		}, false},
		{"a step to come before this vote's", func() Process {
			p := open()
			p.Steps[0].State = StepNext
			return p
		}, false},
		{"a step taken after this vote's", func() Process {
			p := open()
			p.Steps[2].State = StepDone
			return p
		}, false},
		{"a step without a label", func() Process {
			p := open()
			p.Steps[0].Label = " "
			return p
		}, false},
		{"next step at the limit", func() Process {
			p := open()
			p.Next = strings.Repeat("x", MaxNextRunes)
			return p
		}, true},
		{"next step over the limit", func() Process {
			p := open()
			p.Next = strings.Repeat("x", MaxNextRunes+1)
			return p
		}, false},
		{"open business without a next step", func() Process {
			p := open()
			p.Next = ""
			return p
		}, false},
		{"ended business with a next step", func() Process {
			p := ended()
			p.Next = "Der Ständerat entscheidet."
			return p
		}, false},
		{"steps after the one that ended the business", func() Process {
			p := ended()
			p.Steps = append(p.Steps, ProcessStep{Label: "Ständerat", State: StepNext})
			return p
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.p()
			if err := p.Validate(); (err == nil) != tt.valid {
				t.Errorf("Validate() = %v, want valid=%v", err, tt.valid)
			}
		})
	}
}

func TestProcessEnded(t *testing.T) {
	if open().Ended() {
		t.Error("an open business reports itself ended")
	}
	if !ended().Ended() {
		t.Error("an ended business reports itself open")
	}
}

// GroupProcess reports a process only where every vote of the group agrees on
// it: anything less is a guess at which vote is right.
func TestGroupProcess(t *testing.T) {
	withP := func(p *Process) Vote {
		v := Vote{Type: "Normal"}
		v.Affair.Process = p
		return v
	}
	o, o2, e := open(), open(), ended()
	invalid := open()
	invalid.Next = ""

	tests := []struct {
		name  string
		group []Vote
		want  *Process
	}{
		{"empty group", nil, nil},
		{"no process", []Vote{withP(nil)}, nil},
		{"lone vote", []Vote{withP(&o)}, &o},
		{"every vote agrees", []Vote{withP(&o), withP(&o2)}, &o},
		{"one vote without", []Vote{withP(&o), withP(nil)}, nil},
		{"votes disagree", []Vote{withP(&o), withP(&e)}, nil},
		{"invalid process", []Vote{withP(&invalid)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GroupProcess(tt.group); !got.Equal(tt.want) {
				t.Errorf("GroupProcess = %+v, want %+v", got, tt.want)
			}
		})
	}

	t.Run("election business", func(t *testing.T) {
		v := withP(&o)
		v.Type = AttendanceType
		v.Affair.Type = WahlAffairType
		if got := GroupProcess([]Vote{v}); got != nil {
			t.Errorf("an election roll call reported a process: %+v", got)
		}
	})
}
