package contacts

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// HandleTag represents a name and its social handle for tagging.
type HandleTag struct {
	Name   string
	Handle string
}

// BlueskyMention represents a detected mention of a politician who has a Bluesky account.
// ByteStart/ByteEnd are byte offsets into the original text (for Bluesky facets).
type BlueskyMention struct {
	Handle    string // Bluesky handle (e.g. "perparimzh.bsky.social")
	ByteStart int    // byte offset of name start in text
	ByteEnd   int    // byte offset of name end in text
}

// ExtractXHandleFromURL extracts the @handle from an X/Twitter URL
// Examples:
//   - "https://x.com/MoritzBoegli" -> "@MoritzBoegli"
//   - "https://twitter.com/thuritch" -> "@thuritch"
//   - "www.x.com/pascallamprecht" -> "@pascallamprecht"
func ExtractXHandleFromURL(url string) string {
	// Remove common prefixes
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")

	// Handle both x.com and twitter.com
	if strings.HasPrefix(url, "x.com/") {
		handle := strings.TrimPrefix(url, "x.com/")
		return "@" + strings.TrimSpace(handle)
	}
	if strings.HasPrefix(url, "twitter.com/") {
		handle := strings.TrimPrefix(url, "twitter.com/")
		return "@" + strings.TrimSpace(handle)
	}

	return ""
}

// ExtractInstagramHandleFromURL extracts the @username from an Instagram profile URL.
// Examples:
//   - "https://www.instagram.com/annagraff_/" -> "@annagraff_"
//   - "instagram.com/alana.gerdes?hl=de" -> "@alana.gerdes"
func ExtractInstagramHandleFromURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")
	url = strings.TrimPrefix(url, "m.")

	if !strings.HasPrefix(url, "instagram.com/") {
		return ""
	}

	path := strings.TrimPrefix(url, "instagram.com/")
	path = strings.SplitN(path, "?", 2)[0]
	path = strings.SplitN(path, "#", 2)[0]
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	username := strings.Split(path, "/")[0]
	username = strings.TrimPrefix(username, "@")
	if username == "" {
		return ""
	}

	validHandle := regexp.MustCompile(`^[A-Za-z0-9._]+$`)
	if !validHandle.MatchString(username) {
		return ""
	}

	return "@" + username
}

// generateNameVariants creates name permutations for flexible matching
// For "Bögli Moritz" -> ["Bögli Moritz", "Moritz Bögli"]
// For "Garcia Nuñez David" -> ["Garcia Nuñez David", "David Garcia Nuñez"]
func generateNameVariants(name string) []string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) < 2 {
		return []string{strings.TrimSpace(name)}
	}

	// Join parts to create normalized original
	normalized := strings.Join(parts, " ")
	variants := []string{normalized}

	// Simple reversal: last word becomes first, everything else follows
	// "A B C" -> "C A B"
	reversed := parts[len(parts)-1] + " " + strings.Join(parts[:len(parts)-1], " ")
	variants = append(variants, reversed)

	return variants
}

// TagXHandlesInText finds all contacts with X accounts in the text and adds their @handle
// Returns the modified text with inline tags like: "Moritz Bögli @MoritzBoegli"
// Uses greedy matching to prefer longer names (e.g., "David Garcia Nuñez" over "David Garcia")
func (m *Mapper) TagXHandlesInText(text string) string {
	// Collect all contacts with X accounts and their handles
	var taggableContacts []HandleTag

	for _, contact := range m.getAllContacts() {
		verified := contact.Verified("x")
		if len(verified) == 0 {
			continue
		}

		handle := ExtractXHandleFromURL(verified[0])
		if handle == "" {
			continue
		}

		// Generate name variants for flexible matching
		variants := generateNameVariants(contact.Name)
		for _, variant := range variants {
			taggableContacts = append(taggableContacts, HandleTag{
				Name:   variant,
				Handle: handle,
			})
		}
	}

	return tagHandlesInText(text, taggableContacts)
}

// TagInstagramHandlesInText finds all contacts with Instagram accounts in the text and adds their @handle.
func (m *Mapper) TagInstagramHandlesInText(text string) string {
	var taggableContacts []HandleTag

	for _, contact := range m.getAllContacts() {
		verified := contact.Verified("instagram")
		if len(verified) == 0 {
			continue
		}

		handle := ExtractInstagramHandleFromURL(verified[0])
		if handle == "" {
			continue
		}

		variants := generateNameVariants(contact.Name)
		for _, variant := range variants {
			taggableContacts = append(taggableContacts, HandleTag{
				Name:   variant,
				Handle: handle,
			})
		}
	}

	return tagHandlesInText(text, taggableContacts)
}

// tagHandlesInText inserts social handles after matching contact names in text.
func tagHandlesInText(text string, taggableContacts []HandleTag) string {
	// Sort by name length (descending) to match longer names first
	for i := 0; i < len(taggableContacts); i++ {
		for j := i + 1; j < len(taggableContacts); j++ {
			if len(taggableContacts[i].Name) < len(taggableContacts[j].Name) {
				taggableContacts[i], taggableContacts[j] = taggableContacts[j], taggableContacts[i]
			}
		}
	}

	type match struct {
		start  int
		end    int
		handle string
	}
	var matches []match

	for _, tag := range taggableContacts {
		pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(tag.Name))
		re := regexp.MustCompile(pattern)

		indices := re.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			matches = append(matches, match{
				start:  idx[0],
				end:    idx[1],
				handle: tag.Handle,
			})
		}
	}

	var filtered []match
	for _, m1 := range matches {
		overlaps := false
		for _, m2 := range filtered {
			if (m1.start >= m2.start && m1.start < m2.end) ||
				(m1.end > m2.start && m1.end <= m2.end) ||
				(m1.start <= m2.start && m1.end >= m2.end) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			filtered = append(filtered, m1)
		}
	}

	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].start < filtered[j].start {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	result := text
	for _, m := range filtered {
		result = result[:m.end] + " " + m.handle + result[m.end:]
	}

	return result
}

// getAllContacts returns all contacts (helper method for internal use)
func (m *Mapper) getAllContacts() []Contact {
	seen := make(map[string]bool)
	var contacts []Contact

	for _, contact := range m.contacts {
		// Use name as unique key (since we store both original and normalized)
		if !seen[contact.Name] {
			seen[contact.Name] = true
			contacts = append(contacts, contact)
		}
	}

	return contacts
}

// ExtractBlueskyHandleFromURL extracts the handle from a Bluesky profile URL.
// Examples:
//
//   - "https://bsky.app/profile/perparimzh.bsky.social" -> "perparimzh.bsky.social"
//   - "https://bsky.app/profile/spzuerich.ch" -> "spzuerich.ch"
//   - "https://bsky.app/profile/did:plc:xxx" -> "did:plc:xxx"
//   - "https://web-cdn.bsky.app/profile/handle" -> "handle"
func ExtractBlueskyHandleFromURL(url string) string {
	// Remove scheme
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Match bsky.app/profile/ or web-cdn.bsky.app/profile/ etc.
	const profilePath = "/profile/"
	idx := strings.Index(url, profilePath)
	if idx < 0 {
		return ""
	}

	// Verify it's a bsky.app domain
	domain := url[:idx]
	if !strings.HasSuffix(domain, "bsky.app") {
		return ""
	}

	handle := url[idx+len(profilePath):]
	handle = strings.TrimSpace(handle)
	handle = strings.TrimRight(handle, "/")
	if handle == "" {
		return ""
	}
	return handle
}

// FindBlueskyMentions scans text for politician names that have Bluesky accounts
// and returns their byte positions and handles for creating mention facets.
// Unlike TagXHandlesInText, this does NOT modify the text — it only returns
// the positions of names and their corresponding Bluesky handles.
func (m *Mapper) FindBlueskyMentions(text string) []BlueskyMention {
	// Collect all contacts with Bluesky accounts and their handles
	type blueskyTag struct {
		name   string
		handle string
	}
	var taggable []blueskyTag

	for _, contact := range m.getAllContacts() {
		verified := contact.Verified("bluesky")
		if len(verified) == 0 {
			continue
		}

		handle := ExtractBlueskyHandleFromURL(verified[0])
		if handle == "" {
			continue
		}

		// Generate name variants for flexible matching
		variants := generateNameVariants(contact.Name)
		for _, variant := range variants {
			taggable = append(taggable, blueskyTag{
				name:   variant,
				handle: handle,
			})
		}
	}

	// Sort by name length (descending) to match longer names first
	for i := 0; i < len(taggable); i++ {
		for j := i + 1; j < len(taggable); j++ {
			if len(taggable[i].name) < len(taggable[j].name) {
				taggable[i], taggable[j] = taggable[j], taggable[i]
			}
		}
	}

	// Find all matches with positions
	type nameMatch struct {
		start  int
		end    int
		handle string
	}
	var allMatches []nameMatch

	for _, tag := range taggable {
		pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(tag.name))
		re := regexp.MustCompile(pattern)

		indices := re.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			allMatches = append(allMatches, nameMatch{
				start:  idx[0],
				end:    idx[1],
				handle: tag.handle,
			})
		}
	}

	// Remove overlapping matches (keep longest/first)
	var filtered []nameMatch
	for _, m1 := range allMatches {
		overlaps := false
		for _, m2 := range filtered {
			if (m1.start >= m2.start && m1.start < m2.end) ||
				(m1.end > m2.start && m1.end <= m2.end) ||
				(m1.start <= m2.start && m1.end >= m2.end) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			filtered = append(filtered, m1)
		}
	}

	// Convert to BlueskyMention (byte offsets are already correct since
	// FindAllStringIndex returns byte positions for UTF-8 strings in Go)
	var mentions []BlueskyMention
	for _, match := range filtered {
		mentions = append(mentions, BlueskyMention{
			Handle:    match.handle,
			ByteStart: match.start,
			ByteEnd:   match.end,
		})
	}

	return mentions
}

// Mention is a contact named in a text, with their verified accounts on one
// platform.
type Mention struct {
	Name string
	URLs []string
}

// Mentions lists the contacts a text names who have a verified account on a
// platform, in the order the text names them, each once.
//
// It is for a platform where a tag cannot be written into the text: LinkedIn
// mentions are picked from a dropdown by whoever posts, so what a draft needs
// is the list of people to pick and the profile to check each against. Names
// match as they do for the tagging platforms, in either order and longest
// first, so "David Garcia Nuñez" is not also found as "David Garcia".
func (m *Mapper) Mentions(text, platform string) []Mention {
	type match struct {
		start, end int
		mention    Mention
	}
	var matches []match
	for _, contact := range m.allContacts {
		urls := contact.Verified(platform)
		if len(urls) == 0 {
			continue
		}
		for _, variant := range generateNameVariants(contact.Name) {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(variant) + `\b`)
			for _, idx := range re.FindAllStringIndex(text, -1) {
				matches = append(matches, match{idx[0], idx[1], Mention{Name: contact.Name, URLs: urls}})
			}
		}
	}

	// Longest first, so a longer name claims its span before a shorter one
	// contained in it; then back into reading order.
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].end-matches[i].start > matches[j].end-matches[j].start
	})
	var kept []match
	for _, c := range matches {
		overlaps := false
		for _, k := range kept {
			if c.start < k.end && k.start < c.end {
				overlaps = true
				break
			}
		}
		if !overlaps {
			kept = append(kept, c)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].start < kept[j].start })

	seen := map[string]bool{}
	var out []Mention
	for _, k := range kept {
		if !seen[k.mention.Name] {
			seen[k.mention.Name] = true
			out = append(out, k.mention)
		}
	}
	return out
}
