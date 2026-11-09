package parlamentch

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// A vote the Nationalrat takes twice. When a member moves that a vote be
// repeated — "Ordnungsantrag Matter (Wiederholen der Gesamtabstimmung)" on
// 25.060, 08.09.2025 — and the chamber agrees, it votes again, and the second
// vote replaces the first: the minutes go on "Wir wiederholen also die
// Gesamtabstimmung". Both votes pass the subject filter and land in one group,
// labelled alike, so the card showed two Gesamtabstimmungen with different
// counts and nothing to say which one stood.
//
// The Ordnungsantrag itself is in the service, on the same business, with the
// repetition in its subject. It is the evidence used: two votes alike and an
// Ordnungsantrag to repeat between them. Two votes alike without one are left
// as they are. On 24.314 (28.04.2026) the chamber did vote twice with the same
// meanings, and the vote between was recorded with neither a business nor a
// subject; nothing in the service says the second replaced the first.

// repeatOrderDTO is a vote on an Ordnungsantrag, as far as markRepeats reads it.
type repeatOrderDTO struct {
	Subject *string `json:"Subject"`
	VoteEnd string  `json:"VoteEnd"`
}

// isRepeatOrder reports whether a vote's subject is a motion to repeat a vote,
// in the forms the service holds: "Wiederholen der Gesamtabstimmung",
// "Wiederholung der Abstimmung", "Abstimmung … wiederholen".
func isRepeatOrder(subject string) bool {
	return strings.Contains(strings.ToLower(subject), "wiederhol")
}

// markRepeats marks, in a group, each vote that a later vote alike replaced
// after an Ordnungsantrag to repeat it, and the vote that replaced it. Votes
// are alike when they share subject, bill and both meanings.
//
// A group without two votes alike costs no request.
func (c *Client) markRepeats(group []votes.Vote) error {
	type pair struct{ first, second int }
	var pairs []pair
	for i := range group {
		for j := range group {
			if j == i || !group[j].Date.After(group[i].Date) {
				continue
			}
			a, b := c.labels[group[i].SourceID], c.labels[group[j].SourceID]
			if a.subject != "" && a.subject == b.subject && a.billNumber == b.billNumber &&
				a.meaningYes == b.meaningYes && a.meaningNo == b.meaningNo {
				pairs = append(pairs, pair{i, j})
			}
		}
	}
	if len(pairs) == 0 {
		return nil
	}

	var orders []repeatOrderDTO
	day := group[0].Date
	if err := c.get("/Vote", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %s and substringof('wiederhol',tolower(Subject)) eq true", group[0].Affair.ID),
		"$select": "Subject,VoteEnd",
	}, &orders); err != nil {
		return fmt.Errorf("parlamentch: business %s: reading its Ordnungsanträge: %w", group[0].Affair.Number, err)
	}
	for _, p := range pairs {
		first, second := group[p.first].Date, group[p.second].Date
		for _, o := range orders {
			at := parseODataDate(o.VoteEnd)
			if !isRepeatOrder(deref(o.Subject)) || at.Format("2006-01-02") != day.Format("2006-01-02") ||
				!at.After(first) || !at.Before(second) {
				continue
			}
			a, b := c.labels[group[p.first].SourceID], c.labels[group[p.second].SourceID]
			a.repeated, b.repeat = true, true
			c.labels[group[p.first].SourceID], c.labels[group[p.second].SourceID] = a, b
			break
		}
	}
	return nil
}

// standing is the group without the votes a repetition replaced.
func (c *Client) standing(group []votes.Vote) []votes.Vote {
	out := group[:0:0]
	for _, v := range group {
		if !c.labels[v.SourceID].repeated {
			out = append(out, v)
		}
	}
	return out
}
