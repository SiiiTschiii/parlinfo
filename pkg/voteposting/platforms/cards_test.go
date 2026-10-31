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
		want       []int
	}{
		{"lone vote: card on the root only", 1, 2, []int{0}, []int{0, NoCard}},
		{"each vote's card on the reply opening it", 3, 4, []int{0, 1, 2}, []int{0, 1, 2, 3}},
		{"a vote spilling over leaves its continuation bare", 2, 5, []int{0, 2}, []int{0, 1, NoCard, 2, NoCard}},
		{"election notice: one post, one card", 1, 1, nil, []int{0}},
		{"no posts", 2, 0, nil, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ThreadCards(tt.groupSize, tt.posts, tt.voteStarts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ThreadCards = %v, want %v", got, tt.want)
			}
		})
	}
}
