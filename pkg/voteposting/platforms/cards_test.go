package platforms

import (
	"reflect"
	"testing"
)

func TestThreadCards(t *testing.T) {
	tests := []struct {
		name       string
		groupSize  int
		posts      int
		voteStarts []int
		status     bool
		want       []int
	}{
		{"lone vote: card on the root only", 1, 2, []int{0}, false, []int{0, NoCard}},
		{"each vote's card on the reply opening it", 3, 4, []int{0, 1, 2}, false, []int{0, 1, 2, 3}},
		{"a vote spilling over leaves its continuation bare", 2, 5, []int{0, 2}, false, []int{0, 1, NoCard, 2, NoCard}},
		{"election notice: one post, one card", 1, 1, nil, false, []int{0}},
		{"no posts", 2, 0, nil, false, []int{}},

		{"lone vote: status card on the first reply", 1, 3, []int{0}, true, []int{0, 1, NoCard}},
		{"lone vote with no reply: status card left out", 1, 1, []int{0}, true, []int{0}},
		// The links take the first reply, so the votes start one further down
		// and their cards one index later, after the status card.
		{"multi-vote: status card on the links reply, results after it", 2, 4, []int{1, 2}, true, []int{0, 1, 2, 3}},
		// No links: the first reply opens vote 1, whose card it keeps.
		{"multi-vote: a result card is never displaced", 2, 3, []int{0, 1}, true, []int{0, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ThreadCards(tt.groupSize, tt.posts, tt.voteStarts, tt.status); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ThreadCards = %v, want %v", got, tt.want)
			}
		})
	}
}
