// Command fetch_nationalrat_recipients writes the recipient list for the
// Nationalrat outreach campaign (send_campaign_emails --audience nationalrat):
// every sitting member with their published email, the language to write to
// them in, and whether to address them with Sie.
//
// The file carries personal emails, so it lands in the gitignored
// data/campaign_recipients/. It is meant to be read and corrected by hand
// before anything is sent; every guess the command makes is commented in it.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/parlamentch"
)

var (
	outPath  = flag.String("output", "data/campaign_recipients/nationalrat.yaml", "Where to write the recipients file")
	force    = flag.Bool("force", false, "Overwrite an existing file (it may hold hand corrections)")
	speeches = flag.Int("speeches", 20, "How many of a member's latest speeches decide their language")
)

func main() {
	flag.Parse()

	if _, err := os.Stat(*outPath); err == nil && !*force {
		log.Fatalf("%s exists and may hold hand corrections; pass --force to overwrite it", *outPath)
	}

	j, err := config.LookupJurisdiction(config.NationalratKey)
	if err != nil {
		log.Fatalf("Looking up the Nationalrat: %v", err)
	}
	client := parlamentch.New(j.Jurisdiction)

	members, err := client.FetchMemberDetails()
	if err != nil {
		log.Fatalf("Fetching members: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Fetched %d sitting members\n", len(members))

	var sb strings.Builder
	sb.WriteString(header(time.Now()))
	noEmail := 0
	for i, m := range members {
		counts, err := client.SpeechLanguages(m.PersonNumber, *speeches)
		if err != nil {
			// A failed lookup falls back to the canton, which the comment says.
			fmt.Fprintf(os.Stderr, "  ⚠️  %s: speeches not fetched (%v), using the canton\n", m.Name(), err)
			counts = nil
		}
		if len(m.Emails) == 0 {
			noEmail++
			fmt.Fprintf(os.Stderr, "  ⚠️  %s publishes no email: left empty, fill it in by hand or the send skips them\n", m.Name())
		}
		sb.WriteString(entry(m, counts))
		if (i+1)%25 == 0 {
			fmt.Fprintf(os.Stderr, "  … %d/%d\n", i+1, len(members))
		}
	}

	if err := os.WriteFile(*outPath, []byte(sb.String()), 0o600); err != nil {
		log.Fatalf("Writing %s: %v", *outPath, err)
	}
	fmt.Fprintf(os.Stderr, "Wrote %d members to %s (%d without an email)\n", len(members), *outPath, noEmail)
	fmt.Fprintf(os.Stderr, "Next: review the language and formal lines, then\n  go run ./cmd/send_campaign_emails --audience nationalrat\n")
}

func header(now time.Time) string {
	return fmt.Sprintf(`# Nationalrat outreach recipients, written by cmd/fetch_nationalrat_recipients
# on %s from the parlament.ch OData service. GITIGNORED: it holds personal
# emails. Read it before sending and correct what is wrong; the send reads
# exactly what is here.
#
#   language  de | fr | it. The majority language of the member's latest
#             speeches; the canton where they gave too few. Members from the
#             bilingual cantons (BE, FR, VS, GR) decided by canton are marked
#             "prüfen".
#   formal    Always true: vous, Lei, Sie. These are cold emails to national
#             parliamentarians, so nobody gets a du. Set false by hand for
#             someone you know.
#   email     The first address the member publishes; any others are listed in
#             a comment. Empty when they publish none, and then skipped.

recipients:
`, now.Format("2006-01-02"))
}

func entry(m parlamentch.MemberDetails, counts map[string]int) string {
	lang, langWhy := chooseLanguage(counts, m.Canton)

	gender, role := "männlich", "Nationalrat"
	if m.Gender == "f" {
		gender, role = "weiblich", "Nationalrätin"
	}

	email := ""
	if len(m.Emails) > 0 {
		email = m.Emails[0]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  - name: %s\n", strconv.Quote(m.Name()))
	fmt.Fprintf(&sb, "    lastname: %s\n", strconv.Quote(m.LastName))
	fmt.Fprintf(&sb, "    email: %s", strconv.Quote(email))
	if len(m.Emails) > 1 {
		fmt.Fprintf(&sb, "  # auch: %s", strings.Join(m.Emails[1:], ", "))
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "    type: person\n")
	fmt.Fprintf(&sb, "    gender: %s\n", gender)
	fmt.Fprintf(&sb, "    role: %s\n", role)
	fmt.Fprintf(&sb, "    party: %s\n", strconv.Quote(m.Party))
	fmt.Fprintf(&sb, "    canton: %s\n", m.Canton)
	fmt.Fprintf(&sb, "    language: %s  # %s\n", lang, langWhy)
	fmt.Fprintf(&sb, "    formal: true  # %s\n", formalWord(lang))
	return sb.String()
}

// minSpeeches is how many speeches a member needs before their speeches, and
// not their canton, decide the language.
const minSpeeches = 3

// cantonLanguage is the language a canton's members are written to in when
// their speeches do not say. The bilingual cantons go by their majority and are
// flagged for a look.
var cantonLanguage = map[string]string{
	"GE": "fr", "VD": "fr", "NE": "fr", "JU": "fr",
	"FR": "fr", "VS": "fr",
	"TI": "it",
}

var bilingualCanton = map[string]bool{"BE": true, "FR": true, "VS": true, "GR": true}

// mixedShare is the share of speeches in a second language at which a member's
// language stops being obvious. It is also the share of Italian that makes a
// member an Italian speaker: Ticinesi and Italian-speaking Bündner give most
// of their speeches in German to be understood, while German and French
// speakers almost never speak Italian in the chamber. French has no such rule,
// because German speakers do give the odd speech in French.
const mixedShare = 0.25

// chooseLanguage picks de, fr or it for a member and says why, for the comment
// beside it in the file.
func chooseLanguage(counts map[string]int, canton string) (string, string) {
	de, fr, it := counts["DE"], counts["FR"], counts["IT"]
	total := de + fr + it

	if total >= minSpeeches {
		why := fmt.Sprintf("Reden: %s", speechSummary(counts))
		share := func(n int) float64 { return float64(n) / float64(total) }
		switch {
		case share(it) >= mixedShare:
			if share(it) < 0.5 {
				why += ", mehrheitlich nicht IT: prüfen"
			}
			return "it", why
		case de != fr:
			lang, runnerUp := "de", fr
			if fr > de {
				lang, runnerUp = "fr", de
			}
			if share(runnerUp) >= mixedShare {
				why += ", gemischt: prüfen"
			}
			return lang, why
		}
		// An even split between German and French: the canton decides.
	}

	lang := cantonLanguage[canton]
	if lang == "" {
		lang = "de"
	}
	why := fmt.Sprintf("Kanton %s", canton)
	if total == 0 {
		why += ", keine Reden"
	} else {
		why += fmt.Sprintf(", Reden unklar (%s)", speechSummary(counts))
	}
	if bilingualCanton[canton] {
		why += ", zweisprachig: prüfen"
	}
	return lang, why
}

func speechSummary(counts map[string]int) string {
	var parts []string
	for _, l := range []string{"DE", "FR", "IT", "RM"} {
		if counts[l] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[l], l))
		}
	}
	if len(parts) == 0 {
		return "keine"
	}
	return strings.Join(parts, ", ")
}

// formalWord names the formal address in the member's language, for the comment
// beside the formal line.
func formalWord(lang string) string {
	switch lang {
	case "fr":
		return "vous"
	case "it":
		return "Lei"
	}
	return "Sie"
}
