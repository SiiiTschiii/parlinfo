package main

import (
	"strings"
	"testing"
)

func TestHandleFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.instagram.com/annarosenwasser/":   "@annarosenwasser",
		"https://x.com/andrisilber":                    "@andrisilber",
		"https://bsky.app/profile/someone.bsky.social": "@someone.bsky.social",
		"https://www.instagram.com/handle/?hl=de":      "@handle",
		"  https://www.instagram.com/spaced  ":         "@spaced",
		"https://www.tiktok.com/@already.prefixed":     "@already.prefixed",
	} {
		if got := handleFromURL(in); got != want {
			t.Errorf("handleFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJoinList(t *testing.T) {
	for _, tc := range []struct {
		items []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a und b"},
		{[]string{"a", "b", "c"}, "a, b und c"},
	} {
		if got := joinList(tc.items, "und"); got != tc.want {
			t.Errorf("joinList(%q) = %q, want %q", tc.items, got, tc.want)
		}
	}
}

func TestChparlAccount_FallsBackToGermanWhereThereIsNoOther(t *testing.T) {
	a, german := chparlAccount("Instagram", "it")
	if german || a.URL != "https://www.instagram.com/chparlinfo_it" {
		t.Errorf("Instagram/it = %q (german %v), want the Italian account", a.URL, german)
	}
	a, german = chparlAccount("X", "fr")
	if !german || a.URL != "https://x.com/chparlinfo" {
		t.Errorf("X/fr = %q (german %v), want the German account, flagged", a.URL, german)
	}
}

func TestNationalratBody(t *testing.T) {
	tests := []struct {
		name    string
		r       Recipient
		want    []string
		notWant []string
	}{
		{
			name: "French member tagged on Instagram and X",
			r: Recipient{
				Name: "Léa Dupont", Lastname: "Dupont", Gender: "weiblich", Type: "person",
				Language: "fr", Formal: true,
				Tagged: []taggedAccount{{"Instagram", "@lea"}, {"X", "@lea_x"}},
			},
			want: []string{
				"Bonjour Madame Dupont,",
				"dans la publication : sur Instagram (@lea) et sur X (@lea_x).",
				"📸 Instagram : https://www.instagram.com/chparlinfo_fr\n",
				"✖️ X : https://x.com/chparlinfo (en allemand)\n",
				"https://parlinfo.ch/fr/ch/",
				"Dès le 30 novembre",
			},
			notWant: []string{"m'envoyer le lien"},
		},
		{
			name: "German member on du, tagged nowhere",
			r: Recipient{
				Name: "Max Muster", Lastname: "Muster", Gender: "männlich", Salutation: "Lieber",
				Type: "person", Language: "de",
			},
			want: []string{
				"Lieber Max Muster\n",
				"nennen wir dich im Post",
				"Wenn du mir den Link schickst",
				// Untagged members are pointed at Instagram, and only there.
				"📸 Instagram: https://www.instagram.com/chparlinfo\n",
				"https://parlinfo.ch/de/ch/",
				"Liebe Grüsse",
			},
			notWant: []string{"✖️", "Sie", "(en allemand)"},
		},
		{
			name: "German member on Sie, tagged on Bluesky",
			r: Recipient{
				Name: "Erika Muster", Lastname: "Muster", Gender: "weiblich", Type: "person",
				Language: "de", Formal: true,
				Tagged: []taggedAccount{{"Bluesky", "@erika.bsky.social"}},
			},
			want: []string{
				"Sehr geehrte Frau Muster\n",
				"markieren wir Sie im Post: auf Bluesky als @erika.bsky.social.",
				"🔵 Bluesky: https://bsky.app/profile/chparlinfo.bsky.social\n",
				"Freundliche Grüsse",
			},
			notWant: []string{"📸", " du "},
		},
		{
			name: "Italian member tagged on X",
			r: Recipient{
				Name: "Mario Esempio", Lastname: "Esempio", Gender: "männlich", Type: "person",
				Language: "it", Formal: true,
				Tagged: []taggedAccount{{"X", "@mario"}},
			},
			want: []string{
				"Gentile signor Esempio,\n\nin vista della sessione invernale",
				"La menzioniamo nel post: su X (@mario).",
				"✖️ X: https://x.com/chparlinfo (in tedesco)\n",
				"https://parlinfo.ch/it/ch/",
				"Cordiali saluti",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := nationalratBody(tc.r)
			for _, w := range tc.want {
				if !strings.Contains(body, w) {
					t.Errorf("body lacks %q:\n%s", w, body)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(body, w) {
					t.Errorf("body has %q:\n%s", w, body)
				}
			}
		})
	}
}

func TestNationalratSubjectIsInTheMembersLanguage(t *testing.T) {
	for lang, want := range map[string]string{
		"de": "Nationalrats",
		"fr": "Conseil national",
		"it": "Consiglio nazionale",
	} {
		if s := nationalratWordsFor(lang).Subject; !strings.Contains(s, want) {
			t.Errorf("%s subject %q lacks %q", lang, s, want)
		}
	}
}

func TestBuildMIMEMessage_EncodesANonASCIISubject(t *testing.T) {
	msg := string(buildMIMEMessage("a@example.org", "b@example.org", "les réseaux sociaux", "body"))
	if strings.Contains(msg, "réseaux") || !strings.Contains(msg, "Subject: =?utf-8?q?") {
		t.Errorf("subject not encoded:\n%s", msg)
	}
	msg = string(buildMIMEMessage("a@example.org", "b@example.org", "plain", "body"))
	if !strings.Contains(msg, "Subject: plain\r\n") {
		t.Errorf("ASCII subject changed:\n%s", msg)
	}
}
