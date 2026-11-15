package contacts

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractXHandleFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard X URL",
			url:      "https://x.com/MoritzBoegli",
			expected: "@MoritzBoegli",
		},
		{
			name:     "Twitter URL",
			url:      "https://twitter.com/thuritch",
			expected: "@thuritch",
		},
		{
			name:     "URL without https",
			url:      "www.x.com/pascallamprecht",
			expected: "@pascallamprecht",
		},
		{
			name:     "URL in array format from YAML",
			url:      "https://x.com/HenzYves",
			expected: "@HenzYves",
		},
		{
			name:     "Invalid URL",
			url:      "https://facebook.com/something",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractXHandleFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractXHandleFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractInstagramHandleFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard Instagram URL with slash",
			url:      "https://www.instagram.com/annagraff_/",
			expected: "@annagraff_",
		},
		{
			name:     "Instagram URL without scheme",
			url:      "instagram.com/alana.gerdes",
			expected: "@alana.gerdes",
		},
		{
			name:     "Instagram URL with query",
			url:      "https://instagram.com/aliwankoh?hl=de",
			expected: "@aliwankoh",
		},
		{
			name:     "Invalid URL",
			url:      "https://x.com/someone",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractInstagramHandleFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractInstagramHandleFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestGenerateNameVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Two part name",
			input:    "Bögli Moritz",
			expected: []string{"Bögli Moritz", "Moritz Bögli"},
		},
		{
			name:     "Three part name",
			input:    "Garcia Nuñez David",
			expected: []string{"Garcia Nuñez David", "David Garcia Nuñez"},
		},
		{
			name:     "Single name",
			input:    "Madonna",
			expected: []string{"Madonna"},
		},
		{
			name:     "Name with extra spaces",
			input:    "  Moritz  Bögli  ",
			expected: []string{"Moritz Bögli", "Bögli Moritz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNameVariants(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("generateNameVariants(%q) returned %d variants, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, variant := range result {
				if variant != tt.expected[i] {
					t.Errorf("generateNameVariants(%q)[%d] = %q, want %q", tt.input, i, variant, tt.expected[i])
				}
			}
		})
	}
}

func TestTagXHandlesInText(t *testing.T) {
	// Create a test mapper with sample contacts
	mapper := &Mapper{
		contacts: map[string]Contact{
			"Bögli Moritz": {
				Name: "Bögli Moritz",
				X:    VerifiedAccounts("https://x.com/MoritzBoegli"),
			},
			"moritz bögli": {
				Name: "Bögli Moritz",
				X:    VerifiedAccounts("https://x.com/MoritzBoegli"),
			},
			"Garcia Nuñez David": {
				Name: "Garcia Nuñez David",
				X:    VerifiedAccounts("https://x.com/thuritch"),
			},
			"garcia nuñez david": {
				Name: "Garcia Nuñez David",
				X:    VerifiedAccounts("https://x.com/thuritch"),
			},
			"Christian Häberli": {
				Name: "Christian Häberli",
				X:    VerifiedAccounts(),
			},
			"christian häberli": {
				Name: "Christian Häberli",
				X:    VerifiedAccounts(),
			},
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single name match",
			input:    "Dringliches Postulat von Moritz Bögli (AL)",
			expected: "Dringliches Postulat von Moritz Bögli @MoritzBoegli (AL)",
		},
		{
			name:     "Multiple names, some with handles",
			input:    "Postulat von Moritz Bögli (AL), Christian Häberli (AL) und Dr. David Garcia Nuñez (AL)",
			expected: "Postulat von Moritz Bögli @MoritzBoegli (AL), Christian Häberli (AL) und Dr. David Garcia Nuñez @thuritch (AL)",
		},
		{
			name:     "No matches",
			input:    "Postulat von Unknown Person (AL)",
			expected: "Postulat von Unknown Person (AL)",
		},
		{
			name:     "Name in different format",
			input:    "Motion von Bögli Moritz",
			expected: "Motion von Bögli Moritz @MoritzBoegli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.TagXHandlesInText(tt.input)
			if result != tt.expected {
				t.Errorf("TagXHandlesInText() failed\nInput:    %q\nExpected: %q\nGot:      %q", tt.input, tt.expected, result)
			}
		})
	}
}

func TestTagInstagramHandlesInText(t *testing.T) {
	mapper := &Mapper{
		contacts: map[string]Contact{
			"Graff Anna": {
				Name:      "Graff Anna",
				Instagram: VerifiedAccounts("https://www.instagram.com/annagraff_/"),
			},
			"graff anna": {
				Name:      "Graff Anna",
				Instagram: VerifiedAccounts("https://www.instagram.com/annagraff_/"),
			},
			"Garcia Nuñez David": {
				Name:      "Garcia Nuñez David",
				Instagram: VerifiedAccounts("https://www.instagram.com/david.gn/"),
			},
			"garcia nuñez david": {
				Name:      "Garcia Nuñez David",
				Instagram: VerifiedAccounts("https://www.instagram.com/david.gn/"),
			},
			"Christian Häberli": {
				Name: "Christian Häberli",
			},
			"christian häberli": {
				Name: "Christian Häberli",
			},
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single name match",
			input:    "Postulat von Anna Graff (SP)",
			expected: "Postulat von Anna Graff @annagraff_ (SP)",
		},
		{
			name:     "Multiple names, some with handles",
			input:    "Postulat von Anna Graff (SP), Christian Häberli (AL) und David Garcia Nuñez (AL)",
			expected: "Postulat von Anna Graff @annagraff_ (SP), Christian Häberli (AL) und David Garcia Nuñez @david.gn (AL)",
		},
		{
			name:     "No matches",
			input:    "Postulat von Unknown Person (AL)",
			expected: "Postulat von Unknown Person (AL)",
		},
		{
			name:     "Reversed name format",
			input:    "Motion von Graff Anna",
			expected: "Motion von Graff Anna @annagraff_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.TagInstagramHandlesInText(tt.input)
			if result != tt.expected {
				t.Errorf("TagInstagramHandlesInText() failed\nInput:    %q\nExpected: %q\nGot:      %q", tt.input, tt.expected, result)
			}
		})
	}
}

// --- Bluesky Tests ---

func TestExtractBlueskyHandleFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard bsky.app URL",
			url:      "https://bsky.app/profile/perparimzh.bsky.social",
			expected: "perparimzh.bsky.social",
		},
		{
			name:     "Custom domain handle",
			url:      "https://bsky.app/profile/spzuerich.ch",
			expected: "spzuerich.ch",
		},
		{
			name:     "DID URL",
			url:      "https://bsky.app/profile/did:plc:fy7i5nqguaqwvqozbguqwjz3",
			expected: "did:plc:fy7i5nqguaqwvqozbguqwjz3",
		},
		{
			name:     "CDN URL variant",
			url:      "https://web-cdn.bsky.app/profile/michaamstad.bsky.social",
			expected: "michaamstad.bsky.social",
		},
		{
			name:     "Custom domain in bsky.app",
			url:      "https://bsky.app/profile/patrickstaehlin.ch",
			expected: "patrickstaehlin.ch",
		},
		{
			name:     "Not a Bluesky URL",
			url:      "https://x.com/someone",
			expected: "",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBlueskyHandleFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractBlueskyHandleFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestFindBlueskyMentions(t *testing.T) {
	mapper := &Mapper{
		contacts: map[string]Contact{
			"Graff Anna": {
				Name:    "Graff Anna",
				Bluesky: VerifiedAccounts("https://bsky.app/profile/annagraff.bsky.social"),
			},
			"graff anna": {
				Name:    "Graff Anna",
				Bluesky: VerifiedAccounts("https://bsky.app/profile/annagraff.bsky.social"),
			},
			"Garcia Nuñez David": {
				Name:    "Garcia Nuñez David",
				Bluesky: VerifiedAccounts("https://bsky.app/profile/thuritch.bsky.social"),
			},
			"garcia nuñez david": {
				Name:    "Garcia Nuñez David",
				Bluesky: VerifiedAccounts("https://bsky.app/profile/thuritch.bsky.social"),
			},
			"Christian Häberli": {
				Name: "Christian Häberli",
				// No Bluesky account
			},
			"christian häberli": {
				Name: "Christian Häberli",
			},
		},
	}

	t.Run("Single match", func(t *testing.T) {
		text := "Postulat von Anna Graff (SP)"
		mentions := mapper.FindBlueskyMentions(text)

		if len(mentions) != 1 {
			t.Fatalf("expected 1 mention, got %d", len(mentions))
		}

		m := mentions[0]
		if m.Handle != "annagraff.bsky.social" {
			t.Errorf("handle = %q, want %q", m.Handle, "annagraff.bsky.social")
		}
		// Verify byte offsets point to "Anna Graff" in the text
		extracted := text[m.ByteStart:m.ByteEnd]
		if extracted != "Anna Graff" {
			t.Errorf("byte range extracts %q, want %q", extracted, "Anna Graff")
		}
	})

	t.Run("Multiple matches", func(t *testing.T) {
		text := "Postulat von Anna Graff (SP) und Dr. David Garcia Nuñez (AL)"
		mentions := mapper.FindBlueskyMentions(text)

		if len(mentions) != 2 {
			t.Fatalf("expected 2 mentions, got %d", len(mentions))
		}

		// Verify both mentions extract correctly
		handles := make(map[string]string)
		for _, m := range mentions {
			extracted := text[m.ByteStart:m.ByteEnd]
			handles[m.Handle] = extracted
		}

		if name, ok := handles["annagraff.bsky.social"]; !ok || name != "Anna Graff" {
			t.Errorf("missing or wrong mention for Anna Graff: %v", handles)
		}
		if name, ok := handles["thuritch.bsky.social"]; !ok || name != "David Garcia Nuñez" {
			t.Errorf("missing or wrong mention for David Garcia Nuñez: %v", handles)
		}
	})

	t.Run("No Bluesky account", func(t *testing.T) {
		text := "Postulat von Christian Häberli (AL)"
		mentions := mapper.FindBlueskyMentions(text)

		if len(mentions) != 0 {
			t.Errorf("expected 0 mentions for contact without Bluesky, got %d", len(mentions))
		}
	})

	t.Run("No matches at all", func(t *testing.T) {
		text := "Postulat von Unknown Person (AL)"
		mentions := mapper.FindBlueskyMentions(text)

		if len(mentions) != 0 {
			t.Errorf("expected 0 mentions, got %d", len(mentions))
		}
	})

	t.Run("Reversed name format", func(t *testing.T) {
		text := "Motion von Graff Anna (Grüne)"
		mentions := mapper.FindBlueskyMentions(text)

		if len(mentions) != 1 {
			t.Fatalf("expected 1 mention, got %d", len(mentions))
		}

		extracted := text[mentions[0].ByteStart:mentions[0].ByteEnd]
		if extracted != "Graff Anna" {
			t.Errorf("byte range extracts %q, want %q", extracted, "Graff Anna")
		}
	})

	t.Run("Text not modified", func(t *testing.T) {
		text := "Postulat von Anna Graff (SP)"
		originalText := text
		_ = mapper.FindBlueskyMentions(text)

		if text != originalText {
			t.Errorf("FindBlueskyMentions should not modify text")
		}
	})
}

// A dual mandate puts the same person in two jurisdictions' files: curated in
// one, a bare name in the other's uncurated roster. Losing their handles
// because a second file happened to list them would be a silent regression in
// the body that already worked.
func TestLoadContactFiles_MergesADualMandate(t *testing.T) {
	dir := t.TempDir()

	curated := filepath.Join(dir, "city.yaml")
	if err := os.WriteFile(curated, []byte(`version: "1.0"
contacts:
  - name: David Garcia Nuñez
    bluesky:
      - url: https://bsky.app/profile/thuritch.bsky.social
        verified: true
    x:
      - url: https://x.com/thuritch
        verified: true
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	roster := filepath.Join(dir, "canton.yaml")
	if err := os.WriteFile(roster, []byte(`version: "1.0"
contacts:
  - name: David Garcia Nuñez
  - name: Someone Else
    x:
      - url: https://x.com/someoneelse
        verified: true
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The uncurated roster loads last, as it does in the real channel.
	mapper, err := LoadContactFiles(curated, roster)
	if err != nil {
		t.Fatalf("LoadContactFiles: %v", err)
	}

	if got := mapper.GetXHandle("David Garcia Nuñez"); got != "https://x.com/thuritch" {
		t.Errorf("X handle = %q, want the curated one kept", got)
	}
	if !mapper.HasPlatform("David Garcia Nuñez", "bluesky") {
		t.Error("the Bluesky account was lost to the bare roster entry")
	}
	if got := len(mapper.GetAllContacts()); got != 2 {
		t.Errorf("got %d contacts, want the duplicate folded into one", got)
	}
	if got := mapper.GetXHandle("Someone Else"); got != "https://x.com/someoneelse" {
		t.Errorf("Someone Else X handle = %q, want the second file still read", got)
	}
}

// Accounts verified separately in two files are both that person's.
func TestLoadContactFiles_UnionsAccountsAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	a := filepath.Join(dir, "a.yaml")
	if err := os.WriteFile(a, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    x:
      - url: https://x.com/annagraff
        verified: true
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	b := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(b, []byte(`version: "1.0"
contacts:
  - name: anna graff
    x:
      - url: https://x.com/annagraff
        verified: true
      - url: https://x.com/annagraff_kr
        verified: true
    instagram:
      - url: https://www.instagram.com/annagraff_/
        verified: true
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	mapper, err := LoadContactFiles(a, b)
	if err != nil {
		t.Fatalf("LoadContactFiles: %v", err)
	}

	handles := mapper.GetXHandles("Anna Graff")
	if len(handles) != 2 {
		t.Errorf("X handles = %v, want both, with the shared one recorded once", handles)
	}
	if !mapper.HasPlatform("Anna Graff", "instagram") {
		t.Error("the Instagram account from the second file was dropped")
	}
}

// An unverified candidate must never reach a reader, on any platform. This is
// the test the `verified` flag exists for: candidates now sit in the same file
// as confirmed handles, so the only thing keeping a search result out of a post
// is that every tagging path filters on it.
func TestUnverifiedAccountsAreNeverTagged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.yaml")
	if err := os.WriteFile(path, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    bluesky:
      - url: https://bsky.app/profile/guess.bsky.social
        confidence: high
    instagram:
      - url: https://www.instagram.com/guess/
        verified: false
        confidence: high
    x:
      - url: https://x.com/guess
        confidence: high
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	mapper, err := LoadContacts(path)
	if err != nil {
		t.Fatalf("LoadContacts: %v", err)
	}

	const text = "Postulat von Anna Graff (SP)"
	if got := mapper.TagXHandlesInText(text); got != text {
		t.Errorf("X: an unverified handle was tagged: %q", got)
	}
	if got := mapper.TagInstagramHandlesInText(text); got != text {
		t.Errorf("Instagram: an unverified handle was tagged: %q", got)
	}
	if got := mapper.FindBlueskyMentions(text); len(got) != 0 {
		t.Errorf("Bluesky: an unverified handle was mentioned: %+v", got)
	}

	for _, p := range []string{"x", "instagram", "bluesky"} {
		if urls := mapper.GetPlatformURLs("Anna Graff", p); len(urls) != 0 {
			t.Errorf("GetPlatformURLs(%q) = %v, want nothing unverified", p, urls)
		}
		if mapper.HasPlatform("Anna Graff", p) {
			t.Errorf("HasPlatform(%q) is true on an unverified account", p)
		}
	}
	if h := mapper.GetXHandle("Anna Graff"); h != "" {
		t.Errorf("GetXHandle = %q, want empty", h)
	}

	// The candidate is still visible to the curation tools — that is the point
	// of keeping it in the file rather than out of sight.
	c, ok := mapper.GetContact("Anna Graff")
	if !ok || len(c.Accounts("x")) != 1 {
		t.Fatalf("the candidate should still be readable for curation: %+v", c)
	}
	if c.Accounts("x")[0].Confidence != "high" {
		t.Errorf("confidence was lost: %+v", c.Accounts("x")[0])
	}
}

// The shape that leaves `verified` unsaid must not publish anything. It still
// parses, so a stale file degrades to posting without tags rather than failing
// a run — cmd/validate_contacts is what rejects it.
func TestBareStringAccountIsNotVerified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.yaml")
	if err := os.WriteFile(path, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    x:
      - https://x.com/annagraff
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	mapper, err := LoadContacts(path)
	if err != nil {
		t.Fatalf("LoadContacts: %v", err)
	}
	if got := mapper.GetXHandle("Anna Graff"); got != "" {
		t.Errorf("GetXHandle = %q, want nothing — the file never said it was verified", got)
	}
}

// Round-tripping must not quietly change what a file says about publication.
func TestAccountRoundTrip(t *testing.T) {
	in := `version: "1.0"
contacts:
    - name: Anna Graff
      x:
        - url: https://x.com/annagraff
          verified: true
        - url: https://x.com/maybe
          verified: false
          confidence: medium
`
	var m ContactMapping
	if err := yaml.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != in {
		t.Errorf("round trip changed the file:\n--- got ---\n%s\n--- want ---\n%s", out, in)
	}
}

// A confirmation is a fact about the account; a second file listing the same
// URL as a candidate must not take it back.
func TestMergeKeepsTheVerifiedClaim(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(a, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    x:
      - url: https://x.com/annagraff
        confidence: high
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(b, []byte(`version: "1.0"
contacts:
  - name: Anna Graff
    x:
      - url: https://x.com/annagraff
        verified: true
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	mapper, err := LoadContactFiles(a, b)
	if err != nil {
		t.Fatalf("LoadContactFiles: %v", err)
	}
	if got := mapper.GetXHandle("Anna Graff"); got != "https://x.com/annagraff" {
		t.Errorf("GetXHandle = %q — the verified copy should win the merge", got)
	}
}

func TestMentions(t *testing.T) {
	m := newMapper([]Contact{
		{Name: "David Garcia", LinkedIn: VerifiedAccounts("https://www.linkedin.com/in/david-garcia")},
		{Name: "David Garcia Nuñez", LinkedIn: VerifiedAccounts("https://www.linkedin.com/in/dgn")},
		{Name: "Moritz Bögli", LinkedIn: VerifiedAccounts("https://www.linkedin.com/in/moritz-boegli")},
		{Name: "Anna Graff", LinkedIn: []Account{{URL: "https://www.linkedin.com/in/guess"}}},
		{Name: "Ivo Bieri", X: VerifiedAccounts("https://x.com/ivobieri")},
	})

	got := m.Mentions("Postulat von Bögli Moritz (AL), David Garcia Nuñez (AL), Anna Graff (SP), Ivo Bieri (SP) und Moritz Bögli (AL)", "linkedin")

	want := []Mention{
		{Name: "Moritz Bögli", URLs: []string{"https://www.linkedin.com/in/moritz-boegli"}},
		{Name: "David Garcia Nuñez", URLs: []string{"https://www.linkedin.com/in/dgn"}},
	}
	if len(got) != len(want) {
		t.Fatalf("Mentions() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || len(got[i].URLs) != 1 || got[i].URLs[0] != want[i].URLs[0] {
			t.Errorf("Mentions()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
