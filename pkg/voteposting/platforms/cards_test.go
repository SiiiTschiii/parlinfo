package platforms

import (
	"reflect"
	"testing"
)

func TestSpreadCards(t *testing.T) {
	cards := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}

	tests := []struct {
		name  string
		cards int
		posts int
		want  [][]int
	}{
		{"single card on a two-post thread", 1, 2, [][]int{{1}, nil}},
		{"four cards all on the root", 4, 2, [][]int{{1, 2, 3, 4}, nil}},
		{"overflow goes to the first reply", 6, 3, [][]int{{1, 2, 3, 4}, {5, 6}, nil}},
		{"more cards than the thread holds adds posts", 10, 2, [][]int{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10}}},
		{"no cards", 0, 2, [][]int{nil, nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpreadCards(cards(tt.cards), tt.posts, 4)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d posts, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if len(got[i]) == 0 && len(tt.want[i]) == 0 {
					continue
				}
				if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("post %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
