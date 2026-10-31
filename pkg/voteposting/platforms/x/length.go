package x

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// urlWeightedLen is what X charges for a link of any length. Every URL in a
// post is rewritten to a fixed-width t.co address, so the 106 characters of a
// zh.recapp.ch deep link and the 40 of a kantonsrat.zh.ch permalink cost the
// same.
const urlWeightedLen = 23

// weightedLen returns the length X charges for text, which is neither its byte
// length nor its rune count.
//
// Getting this wrong is not symmetric. Overcounting only wastes room — threads
// split earlier than they need to, which is what made a Kantonsrat post with
// 240 characters of content occupy three replies. Undercounting gets the post
// rejected by the API mid-thread, leaving a published root with no results
// under it. So where this approximates, it approximates upwards.
//
// Two rules, both from X's own counting:
//
//   - Characters outside the Latin/general-punctuation ranges below count 2.
//     "ü" costs 1; every emoji costs 2 per code point, so "🗳️" costs 4 because
//     the variation selector is charged too.
//   - A URL counts urlWeightedLen whatever its length.
func weightedLen(s string) int {
	total := 0
	for i := 0; i < len(s); {
		if n := urlLenAt(s, i); n > 0 {
			total += urlWeightedLen
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		total += runeWeight(r)
		i += size
	}
	return total
}

// WeightedLen is weightedLen for callers outside this package that build their
// own posts and must budget them the way X will count them.
func WeightedLen(s string) int { return weightedLen(s) }

// runeWeight is X's per-code-point cost: 1 inside these ranges, 2 outside.
func runeWeight(r rune) int {
	switch {
	case r <= 4351,
		r >= 8192 && r <= 8205,
		r >= 8208 && r <= 8223,
		r >= 8242 && r <= 8247:
		return 1
	}
	return 2
}

// urlLenAt reports the byte length of the link starting at i, or 0 if there is
// none.
//
// Bare domains count, not just explicit http(s) URLs. X linkifies "example.ch"
// on sight and charges it the full 23 — and the CC BY credit line every
// Kanton Zürich post carries is exactly that: "Source: OpenParlData.ch" renders
// as a link. Missing it undercharged those posts by 8, in the one direction
// that gets a reply rejected mid-thread.
func urlLenAt(s string, i int) int {
	if i > 0 {
		prev, _ := utf8.DecodeLastRuneInString(s[:i])
		if !isLinkBoundary(prev) {
			return 0
		}
	}
	rest := s[i:]
	end := strings.IndexAny(rest, " \n\t")
	if end < 0 {
		end = len(rest)
	}
	token := rest[:end]

	if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") && !isBareDomain(token) {
		return 0
	}

	// Punctuation that ends a sentence is not part of the link X makes, so it
	// has to be charged on its own — "…OpenParlData.ch." costs 23 plus 1, not
	// 23. Trimming a closing paren that genuinely belongs to a URL overcharges
	// by one instead, which is the direction this file errs in on purpose.
	return len(strings.TrimRight(token, urlTrailingPunct))
}

// urlTrailingPunct is punctuation that ends a link rather than belonging to it.
const urlTrailingPunct = ".,;:!?)»\"'"

// isLinkBoundary reports whether a link may start after this character.
// Opening punctuation counts: X linkifies the domain in "(OpenParlData.ch)",
// and requiring whitespace would charge those 17 characters instead of 25.
func isLinkBoundary(r rune) bool {
	switch r {
	case '(', '[', '«', '"', '\'', '‘', '“':
		return true
	}
	return unicode.IsSpace(r)
}

// isBareDomain reports whether a token is one X would turn into a link.
//
// An approximation of X's rules, deliberately narrower than a full TLD list:
// what it must not do is classify "15.06.2026" as a link, because charging a
// date 23 characters would waste room in every post header. Requiring the last
// label to be letters separates "OpenParlData.ch" from a date.
func isBareDomain(token string) bool {
	// Only the host decides: "kantonsrat.zh.ch/geschaefte" is a link, and the
	// path is part of what the 23 characters pay for. Splitting it off also
	// rejects tokens like "xhttps://a.ch", whose host part holds a colon.
	host, _, _ := strings.Cut(token, "/")
	if strings.ContainsAny(host, ":@") {
		return false
	}
	host = strings.TrimRight(host, urlTrailingPunct)

	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" {
			return false
		}
	}

	tld := labels[len(labels)-1]
	if len(tld) < 2 {
		return false
	}
	for _, r := range tld {
		if !isASCIILetter(r) {
			return false
		}
	}

	// The name itself must start with something a host can start with, which
	// rules out tokens that merely happen to contain a dot.
	first := rune(labels[0][0])
	return isASCIILetter(first) || (first >= '0' && first <= '9')
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// truncateToWeighted trims s until it costs at most maxLen, returning the
// trimmed text without any ellipsis.
//
// The result is re-measured rather than trusted, because cutting a string can
// make it more expensive per character than the runes suggest: a title cut
// through a URL leaves "https://aver", which still reads as a link and is
// still charged 23. Estimating by rune weight and then shrinking keeps the
// common case linear while making the guarantee hold in every case.
func truncateToWeighted(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if weightedLen(s) <= maxLen {
		return s
	}

	runes := []rune(s)
	cut := len(runes)
	total := 0
	for i, r := range runes {
		total += runeWeight(r)
		if total > maxLen {
			cut = i
			break
		}
	}
	for cut > 0 && weightedLen(string(runes[:cut])) > maxLen {
		cut--
	}
	return string(runes[:cut])
}
