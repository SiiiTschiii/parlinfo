package platforms

// SpreadCards assigns a group's carousel cards to the posts of a thread, in
// carousel order: the root takes the first perPost, each reply the next
// perPost. The result has one entry per post, and more entries than posts when
// the thread runs out before the cards do — those are image-only replies for
// the caller to append.
//
// Front-loading the root keeps the opening post as close to the Instagram
// carousel as the platform allows: for the nineteen groups in twenty that
// render four cards or fewer, the root carries all of them.
func SpreadCards[T any](cards []T, posts, perPost int) [][]T {
	if perPost <= 0 {
		return make([][]T, posts)
	}
	n := posts
	if needed := (len(cards) + perPost - 1) / perPost; needed > n {
		n = needed
	}
	out := make([][]T, n)
	for i := range out {
		start := i * perPost
		if start >= len(cards) {
			break
		}
		end := start + perPost
		if end > len(cards) {
			end = len(cards)
		}
		out[i] = cards[start:end]
	}
	return out
}
