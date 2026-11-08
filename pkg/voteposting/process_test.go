package voteposting

import (
	"errors"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// processSource is a votes.Source that also says where a business stands, with
// a fixed answer per business number.
type processSource struct {
	votes.Source
	answers map[string]*votes.Process
	errs    map[string]error
}

func (s processSource) Process(group []votes.Vote) (*votes.Process, error) {
	n := group[0].Affair.Number
	return s.answers[n], s.errs[n]
}

func validProcess() *votes.Process {
	return &votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Nationalrat", Detail: "Annahme", State: votes.StepCurrent},
			{Label: "In Kommission des Ständerats", State: votes.StepNext},
		},
		Next: "In Kommission des Ständerats",
	}
}

// Every group the source answers for carries the process on each vote; every
// other outcome — no answer, an error, an invalid process — leaves the group as
// it was, and none of them stops the run.
func TestAttachProcesses(t *testing.T) {
	group := func(n string, size int) []votes.Vote {
		g := make([]votes.Vote, size)
		for i := range g {
			g[i].Affair.Number = n
		}
		return g
	}
	invalid := validProcess()
	invalid.Next = ""

	src := processSource{
		answers: map[string]*votes.Process{"ok": validProcess(), "invalid": invalid},
		errs:    map[string]error{"down": errors.New("service unavailable")},
	}
	groups := [][]votes.Vote{group("ok", 2), group("none", 1), group("down", 1), group("invalid", 1)}
	attachProcesses(src, groups)

	for _, v := range groups[0] {
		if v.Affair.Process == nil {
			t.Error("a vote of the answered group has no process")
		}
	}
	for _, g := range groups[1:] {
		if g[0].Affair.Process != nil {
			t.Errorf("group %s got a process: %+v", g[0].Affair.Number, g[0].Affair.Process)
		}
	}
}
