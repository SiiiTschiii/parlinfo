package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
	"gopkg.in/yaml.v3"
)

var (
	contactsPath   = flag.String("contacts", contacts.PathFor("zurich-city"), "Path to contacts YAML file")
	overridesPath  = flag.String("overrides", "data/email_overrides.yaml", "Path to email overrides YAML file")
	platform       = flag.String("platform", "", "Platform campaign: bluesky | instagram | facebook (mutually exclusive with --audience/--custom)")
	audience       = flag.String("audience", "", "Audience campaign: city-parties | cantonal-national-parties | cantonal-zh | federal-zh (mutually exclusive with --platform/--custom)")
	custom         = flag.String("custom", "", "Custom campaign: path to a messages YAML file (per-message to/subject/body; mutually exclusive with --platform/--audience)")
	recipientsPath = flag.String("recipients", "", "Override the recipient YAML file for --audience (default: the audience's data/campaign_recipients/*.yaml)")
	preview        = flag.Bool("preview", false, "Preview: render all emails")
	outputFile     = flag.String("output", "", "Output file for preview (default: stdout)")
	testAddr       = flag.String("test", "", "Test mode: send all emails to this address")
	send           = flag.Bool("send", false, "Send emails to actual recipients")
	delay          = flag.Int("delay", 2, "Seconds between sends")
)

type platformConfig struct {
	Key         string
	DisplayName string
	Subject     string
	Body        func(platformURL string) string
	// SkipIfOn lists platforms whose holders are left out: they already heard
	// about the project through that channel, so this email would be a repeat.
	SkipIfOn []string
}

var platformConfigs = map[string]platformConfig{
	"bluesky":   blueskyConfig,
	"instagram": instagramConfig,
	"facebook":  facebookConfig,
}

// audienceConfig describes an outreach campaign to a whole audience (parties,
// cantonal/federal politicians). Unlike platformConfig, recipients come from a
// curated YAML file and the body is the general announcement template.
type audienceConfig struct {
	Key            string
	DisplayName    string
	Subject        string
	RecipientsFile string
}

var audienceConfigs = map[string]audienceConfig{
	"city-parties": {
		Key:            "city-parties",
		DisplayName:    "Stadtparteien Zürich",
		Subject:        "zuerichratsinfo – Transparenz für die Zürcher Stadtpolitik",
		RecipientsFile: "data/campaign_recipients/city_parties.yaml",
	},
	"cantonal-national-parties": {
		Key:            "cantonal-national-parties",
		DisplayName:    "Kantonal- und Bundesparteien",
		Subject:        "zuerichratsinfo – Transparenz für die Zürcher Politik",
		RecipientsFile: "data/campaign_recipients/cantonal_national_parties.yaml",
	},
	"cantonal-zh": {
		Key:            "cantonal-zh",
		DisplayName:    "Kantonsrat & Regierungsrat Zürich",
		Subject:        "zuerichratsinfo – Abstimmungen aus dem Gemeinderat Zürich transparent gemacht",
		RecipientsFile: "data/campaign_recipients/cantonal_zh_politicians.yaml",
	},
	"federal-zh": {
		Key:            "federal-zh",
		DisplayName:    "National- & Ständerat (ZH)",
		Subject:        "zuerichratsinfo – Abstimmungen aus dem Gemeinderat Zürich transparent gemacht",
		RecipientsFile: "data/campaign_recipients/federal_zh_politicians.yaml",
	},
}

// generalBody renders the general intro + heads-up announcement used by all
// audience campaigns. Greeting and pronouns adapt to three address forms:
//   - org (any):        "Guten Tag" + ihr (informal plural)
//   - person, formal:   "Sehr geehrte/r Frau/Herr <Name>" + Sie
//   - person, informal: "Liebe/Lieber <Name>" + du
func generalBody(r Recipient) string {
	greeting := greetingFor(r)
	var followShare string
	switch {
	case r.Type == "org":
		followShare = "Ich würde mich freuen, wenn ihr dem Account folgt. Über Feedback und Ideen freue ich mich jederzeit."
	case r.Formal:
		followShare = "Ich würde mich freuen, wenn Sie dem Account folgen. Über Feedback und Ideen freue ich mich jederzeit."
	default:
		followShare = "Ich würde mich freuen, wenn du dem Account folgst. Über Feedback und Ideen freue ich mich jederzeit."
	}
	return fmt.Sprintf(`%s

zuerichratsinfo ist ein zivilgesellschaftliches Projekt, das die Abstimmungsresultate aus dem Gemeinderat der Stadt Zürich auf Social Media veröffentlicht, transparent, automatisiert und für alle nachvollziehbar. Zudem markieren wir jeweils die Politikernnen und Politiker, welche die entsprechenden Vorstösse etc. eingereicht haben.

Zu finden sind wir auf:
🔵 Bluesky: https://bsky.app/profile/zuerichratsinfo.bsky.social
✖️ X: https://x.com/zuerichratsinfo
📸 Instagram: https://www.instagram.com/zueriratsinfo

Wir arbeiten laufend daran, die Posts weiterzuentwickeln, zum Beispiel kürzlich mit Statistiken, wie die einzelnen Fraktionen abgestimmt haben. Aktuell liegt der Fokus auf der Stadt Zürich. Eine Ausweitung auf Kantonsrat und Bundesparlament ist ein weiterer geplanter Meilenstein.

%s

Weitere Informationen zum Projekt:
https://github.com/SiiiTschiii/parlinfo

Herzliche Grüsse
Christof
https://www.linkedin.com/in/christof-gerber/
`, greeting, followShare)
}

var blueskyConfig = platformConfig{
	Key:         "bluesky",
	DisplayName: "Bluesky",
	Subject:     "zuerichratsinfo jetzt auch auf Bluesky",
	Body: func(url string) string {
		return fmt.Sprintf(`zuerichratsinfo ist jetzt auch auf Bluesky verfügbar:
👉 https://bsky.app/profile/zuerichratsinfo.bsky.social

Der Account publiziert die Abstimmungsresultate aus dem Gemeinderat auf X (https://x.com/zuerichratsinfo) und neu auch auf Bluesky, und markiert jeweils die Politikernnen und Politiker, welche die entsprechenden Vorstösse etc. eingereicht haben (wie dich: %s). Ziel ist es, politische Arbeit transparenter und für die Öffentlichkeit besser nachvollziehbar zu machen.

Wir arbeiten laufend daran, die Posts weiterzuentwickeln – zum Beispiel mit Statistiken, wie die einzelnen Fraktionen abgestimmt haben. Falls du Ideen oder Feedback hast, würde ich mich sehr darüber freuen!

Ich würde mich freuen, wenn du dem Account folgst. Und vielleicht hast du ja mal Lust einen Abstimmungspost mit deinen Followern zu teilen.
 
Weitere Informationen zum Projekt:
https://github.com/SiiiTschiii/parlinfo

Vielen Dank und liebe Grüsse
Christof
https://www.linkedin.com/in/christof-gerber/
`, url)
	},
}

var instagramConfig = platformConfig{
	Key:         "instagram",
	DisplayName: "Instagram",
	Subject:     "zuerichratsinfo jetzt auch auf Instagram",
	Body: func(url string) string {
		return fmt.Sprintf(`zuerichratsinfo ist jetzt auch auf Instagram verfügbar:
👉 https://www.instagram.com/zueriratsinfo

Der Account publiziert die Abstimmungsresultate aus dem Gemeinderat auf X (https://x.com/zuerichratsinfo), Bluesky (https://bsky.app/profile/zuerichratsinfo.bsky.social) und neu auch auf Instagram, und markiert jeweils die Politikerinnen und Politiker, welche die entsprechenden Vorstösse etc. eingereicht haben (wie dich: %s). Ziel ist es, politische Arbeit transparenter und für die Öffentlichkeit besser nachvollziehbar zu machen.

Wir arbeiten laufend daran, die Posts weiterzuentwickeln – zum Beispiel mit Statistiken, wie die einzelnen Fraktionen abgestimmt haben. Falls du Ideen oder Feedback hast, würde ich mich sehr darüber freuen!

Ich würde mich freuen, wenn du dem Account folgst. Und vielleicht hast du ja mal Lust einen Abstimmungspost mit deinen Followern zu teilen.

Weitere Informationen zum Projekt:
https://github.com/SiiiTschiii/parlinfo

Vielen Dank und liebe Grüsse
Christof
https://www.linkedin.com/in/christof-gerber/
`, url)
	},
}

// facebookConfig only targets contacts whose sole project-relevant channel is
// Facebook: anyone on Bluesky, Instagram or X is reached through those already.
var facebookConfig = platformConfig{
	Key:         "facebook",
	DisplayName: "Facebook",
	Subject:     "zuerichratsinfo jetzt auch auf Facebook",
	SkipIfOn:    []string{"bluesky", "instagram", "x"},
	Body: func(url string) string {
		return fmt.Sprintf(`zuerichratsinfo ist jetzt auch auf Facebook verfügbar:
👉 https://www.facebook.com/people/ZueriRatsinfo/1063164230212573/

Der Account publiziert die Abstimmungsresultate aus dem Gemeinderat auf X (https://x.com/zuerichratsinfo), Bluesky (https://bsky.app/profile/zuerichratsinfo.bsky.social), Instagram (https://www.instagram.com/zueriratsinfo) und neu auch auf Facebook, und nennt jeweils die Politikerinnen und Politiker, welche die entsprechenden Vorstösse etc. eingereicht haben, beim Namen (wie dich: %s). Ziel ist es, politische Arbeit transparenter und für die Öffentlichkeit besser nachvollziehbar zu machen.

Wir arbeiten laufend daran, die Posts weiterzuentwickeln – zum Beispiel mit Statistiken, wie die einzelnen Fraktionen abgestimmt haben. Falls du Ideen oder Feedback hast, würde ich mich sehr darüber freuen!

Ich würde mich freuen, wenn du dem Account folgst. Und vielleicht hast du ja mal Lust einen Abstimmungspost mit deinen Followern zu teilen.

Weitere Informationen zum Projekt:
https://github.com/SiiiTschiii/zuerichratsinfo

Vielen Dank und liebe Grüsse
Christof
https://www.linkedin.com/in/christof-gerber/
`, url)
	},
}

type Recipient struct {
	Name        string
	Email       string
	Gender      string
	Salutation  string
	PlatformURL string
	Source      string
	Type        string // "person" (default) or "org"
	Role        string // e.g. "Nationalrätin" (audience campaigns)
	Party       string // e.g. "SP" (audience campaigns)
	Formal      bool   // person only: use Sie ("Sehr geehrte/r …") instead of du
	Lastname    string // person only: surname for the formal salutation
	Subject     string // custom campaigns: this message's subject line
	Body        string // custom campaigns: this message's fully-rendered body
}

// Campaign is a fully-resolved email campaign: a subject, the recipient list,
// and a function that renders the full body (including greeting) per recipient.
// Both platform and audience campaigns are expressed as a Campaign so the
// verify/preview/send code paths are shared.
type Campaign struct {
	Recipients    []Recipient
	RenderSubject func(Recipient) string
	RenderBody    func(Recipient) string
}

// audienceRecipient mirrors one entry in a data/campaign_recipients/*.yaml file.
type audienceRecipient struct {
	Name   string `yaml:"name"`
	Email  string `yaml:"email"`
	Type   string `yaml:"type"`
	Gender string `yaml:"gender"`
	Role   string `yaml:"role"`
	Party  string `yaml:"party"`
	// Formal: person only. true -> Sie ("Sehr geehrte/r …"), false/unset -> du.
	// Seeded by the age+office heuristic (see README); hand-tune per entry.
	Formal *bool `yaml:"formal"`
	// Lastname: person only, surname for the formal salutation ("… Frau Moser").
	Lastname string `yaml:"lastname"`
}

type audienceRecipientsFile struct {
	Recipients []audienceRecipient `yaml:"recipients"`
}

type Override struct {
	Name   string `yaml:"name"`
	Email  string `yaml:"email"`
	Gender string `yaml:"gender"`
}

type OverridesFile struct {
	Overrides []Override `yaml:"overrides"`
}

func main() {
	flag.Parse()

	if *send && *testAddr != "" {
		log.Fatal("Cannot use --send and --test at the same time")
	}

	campaign := buildCampaign()

	switch {
	case *preview:
		runPreview(campaign)
	case *testAddr != "":
		runSend(campaign, *testAddr)
	case *send:
		runSend(campaign, "")
	default:
		runVerify(campaign)
	}
}

// buildCampaign resolves the selected campaign from --platform, --audience or
// --custom. Exactly one mode must be chosen.
func buildCampaign() Campaign {
	modes := 0
	for _, on := range []bool{*platform != "", *audience != "", *custom != ""} {
		if on {
			modes++
		}
	}
	if modes == 0 {
		log.Fatal("One of --platform (bluesky|instagram|facebook), --audience (city-parties|…) or --custom <file> is required")
	}
	if modes > 1 {
		log.Fatal("Use only one of --platform, --audience or --custom")
	}

	switch {
	case *custom != "":
		return buildCustomCampaign(*custom)
	case *platform != "":
		cfg, ok := platformConfigs[strings.ToLower(*platform)]
		if !ok {
			log.Fatalf("Unknown --platform %q; supported: bluesky, instagram, facebook", *platform)
		}
		return Campaign{
			Recipients:    buildPlatformRecipientList(cfg),
			RenderSubject: func(Recipient) string { return cfg.Subject },
			RenderBody: func(r Recipient) string {
				return fmt.Sprintf("%s %s\n\n%s", r.Salutation, r.Name, cfg.Body(r.PlatformURL))
			},
		}
	case *audience != "":
		ac, ok := audienceConfigs[strings.ToLower(*audience)]
		if !ok {
			log.Fatalf("Unknown --audience %q; supported: city-parties, cantonal-national-parties, cantonal-zh, federal-zh", *audience)
		}
		file := ac.RecipientsFile
		if *recipientsPath != "" {
			file = *recipientsPath
		}
		return Campaign{
			Recipients:    loadAudienceRecipients(file),
			RenderSubject: func(Recipient) string { return ac.Subject },
			RenderBody:    generalBody,
		}
	default:
		// Unreachable: the mode count is validated above.
		return Campaign{}
	}
}

// buildCustomCampaign resolves a --custom campaign from a messages file: a list
// of self-contained messages, each carrying its own recipient address(es),
// subject and body. Each address becomes an individual send, so recipients
// never see one another.
func buildCustomCampaign(path string) Campaign {
	return Campaign{
		Recipients:    loadCustomMessages(path),
		RenderSubject: func(r Recipient) string { return r.Subject },
		RenderBody:    func(r Recipient) string { return r.Body },
	}
}

// addressList accepts either a single email string or a YAML list of them, so
// a message can be addressed to one or several recipients.
type addressList []string

func (a *addressList) UnmarshalYAML(value *yaml.Node) error {
	var one string
	if err := value.Decode(&one); err == nil {
		*a = addressList{one}
		return nil
	}
	var many []string
	if err := value.Decode(&many); err != nil {
		return err
	}
	*a = many
	return nil
}

// customMessage is one fully self-contained email in a --custom messages file:
// its own recipient address(es), subject and body.
type customMessage struct {
	Name    string      `yaml:"name"` // optional label for logs/preview
	To      addressList `yaml:"to"`
	Subject string      `yaml:"subject"`
	Body    string      `yaml:"body"`
}

type customMessagesFile struct {
	Messages []customMessage `yaml:"messages"`
}

// loadCustomMessages reads a --custom messages file and flattens it into one
// Recipient per address, each carrying that message's subject and body. A
// message missing a recipient, subject or body is a hard error — a half-formed
// email must never reach the send loop.
func loadCustomMessages(path string) []Recipient {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read messages file %s: %v", path, err)
	}
	var f customMessagesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Fatalf("Failed to parse messages file %s: %v", path, err)
	}
	if len(f.Messages) == 0 {
		log.Fatalf("No messages found in %s", path)
	}

	var recipients []Recipient
	for i, m := range f.Messages {
		label := m.Name
		if label == "" && len(m.To) > 0 {
			label = m.To[0]
		}
		if len(m.To) == 0 {
			log.Fatalf("Message %d (%q) has no `to` address", i+1, label)
		}
		if strings.TrimSpace(m.Subject) == "" {
			log.Fatalf("Message %d (%q) has no `subject`", i+1, label)
		}
		if strings.TrimSpace(m.Body) == "" {
			log.Fatalf("Message %d (%q) has no `body`", i+1, label)
		}
		for _, addr := range m.To {
			if strings.TrimSpace(addr) == "" {
				log.Fatalf("Message %d (%q) has an empty `to` address", i+1, label)
			}
			recipients = append(recipients, Recipient{
				Name:  m.Name,
				Email: strings.TrimSpace(addr),
				// Type/greeting fields stay empty: custom bodies are literal, so
				// there is no adaptive salutation to report in the verify table.
				Source:  "file",
				Subject: m.Subject,
				Body:    m.Body,
			})
		}
	}
	fmt.Fprintf(os.Stderr, "Loaded %d messages (%d recipients) from %s\n", len(f.Messages), len(recipients), path)
	return recipients
}

// loadAudienceRecipients reads a curated recipient YAML file for an audience
// campaign. Entries with an empty email are skipped with a warning.
func loadAudienceRecipients(path string) []Recipient {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read recipients file %s: %v", path, err)
	}
	var f audienceRecipientsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Fatalf("Failed to parse recipients file %s: %v", path, err)
	}

	var recipients []Recipient
	for _, e := range f.Recipients {
		typ := strings.ToLower(strings.TrimSpace(e.Type))
		if typ == "" {
			typ = "person"
		}
		if strings.TrimSpace(e.Email) == "" {
			fmt.Fprintf(os.Stderr, "  ⚠️  Skipping (no email): %s\n", e.Name)
			continue
		}
		sal := ""
		if typ != "org" {
			sal = salutation(e.Gender)
		}
		formal := e.Formal != nil && *e.Formal
		recipients = append(recipients, Recipient{
			Name:       e.Name,
			Email:      e.Email,
			Gender:     e.Gender,
			Salutation: sal,
			Source:     "file",
			Type:       typ,
			Role:       e.Role,
			Party:      e.Party,
			Formal:     formal,
			Lastname:   e.Lastname,
		})
	}
	fmt.Fprintf(os.Stderr, "Loaded %d recipients from %s\n", len(recipients), path)
	return recipients
}

func buildPlatformRecipientList(cfg platformConfig) []Recipient {
	mapper, err := contacts.LoadContacts(*contactsPath)
	if err != nil {
		log.Fatalf("Failed to load contacts: %v", err)
	}

	allContacts := mapper.GetAllContacts()
	var matchingContacts []contacts.Contact
	skipped := 0
	for _, c := range allContacts {
		if len(mapper.GetPlatformURLs(c.Name, cfg.Key)) == 0 {
			continue
		}
		if hasAnyPlatform(mapper, c.Name, cfg.SkipIfOn) {
			skipped++
			continue
		}
		matchingContacts = append(matchingContacts, c)
	}
	fmt.Fprintf(os.Stderr, "Found %d contacts with %s accounts (%d skipped for also being on %s)\n",
		len(matchingContacts), cfg.DisplayName, skipped, strings.Join(cfg.SkipIfOn, "/"))

	fmt.Fprintf(os.Stderr, "Fetching contact data from API...\n")
	client := zurichapi.NewClient()
	apiKontakte, err := client.FetchAllKontakte()
	if err != nil {
		log.Fatalf("Failed to fetch API data: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d contacts from API\n", len(apiKontakte))

	overrides := loadOverrides(*overridesPath)
	overrideMap := make(map[string]Override)
	for _, o := range overrides {
		overrideMap[strings.ToLower(o.Name)] = o
	}

	var recipients []Recipient

	for _, contact := range matchingContacts {
		urls := mapper.GetPlatformURLs(contact.Name, cfg.Key)
		if len(urls) == 0 {
			continue
		}
		platformURL := urls[0]
		nameLower := strings.ToLower(contact.Name)

		if o, ok := overrideMap[nameLower]; ok {
			recipients = append(recipients, Recipient{
				Name:        contact.Name,
				Email:       o.Email,
				Gender:      o.Gender,
				Salutation:  salutation(o.Gender),
				PlatformURL: platformURL,
				Source:      "override",
				Type:        "person",
			})
			continue
		}

		email, gender, foundInAPI := findEmailForContact(contact.Name, apiKontakte)
		if !foundInAPI {
			fmt.Fprintf(os.Stderr, "  ⚠️  Not found in API (no override): %s\n", contact.Name)
			continue
		}
		if email == "" {
			fmt.Fprintf(os.Stderr, "  ⚠️  Found in API but no email (no override): %s\n", contact.Name)
			continue
		}

		recipients = append(recipients, Recipient{
			Name:        contact.Name,
			Email:       email,
			Gender:      gender,
			Salutation:  salutation(gender),
			PlatformURL: platformURL,
			Source:      "api",
			Type:        "person",
		})
	}

	fmt.Fprintf(os.Stderr, "Total: %d recipients (%d from API, %d from overrides)\n",
		len(recipients),
		countBySource(recipients, "api"),
		countBySource(recipients, "override"))

	return recipients
}

func hasAnyPlatform(mapper *contacts.Mapper, name string, platforms []string) bool {
	for _, p := range platforms {
		if len(mapper.GetPlatformURLs(name, p)) > 0 {
			return true
		}
	}
	return false
}

func countBySource(recipients []Recipient, source string) int {
	n := 0
	for _, r := range recipients {
		if r.Source == source {
			n++
		}
	}
	return n
}

func salutation(gender string) string {
	if gender == "weiblich" {
		return "Liebe"
	}
	return "Lieber"
}

func formalSalutation(gender string) string {
	if gender == "weiblich" {
		return "Sehr geehrte Frau"
	}
	return "Sehr geehrter Herr"
}

// greetingFor renders the salutation line for a recipient, matching the three
// address forms (org -> "Guten Tag"; formal person -> "Sehr geehrte/r Frau/Herr
// <lastname>"; informal person -> "Liebe/Lieber <Name>"). Shared by generalBody
// and custom-campaign templates (exposed there as {{.Greeting}}).
func greetingFor(r Recipient) string {
	switch {
	case r.Type == "org":
		return "Guten Tag"
	case r.Formal:
		// Formal address uses the surname only ("Sehr geehrte Frau Moser").
		// Fall back to the full name if no lastname is curated.
		name := r.Lastname
		if name == "" {
			name = r.Name
		}
		return fmt.Sprintf("%s %s", formalSalutation(r.Gender), name)
	default:
		return fmt.Sprintf("%s %s", r.Salutation, r.Name)
	}
}

// addressForm is the pronoun a recipient is addressed with, for the verify table.
func addressForm(r Recipient) string {
	switch {
	case r.Type == "":
		return "-" // custom campaigns: literal body, no adaptive salutation
	case r.Type == "org":
		return "ihr"
	case r.Formal:
		return "Sie"
	default:
		return "du"
	}
}

func loadOverrides(path string) []Override {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "No overrides file found at %s, continuing without overrides\n", path)
			return nil
		}
		log.Fatalf("Failed to read overrides file: %v", err)
	}
	var f OverridesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Fatalf("Failed to parse overrides file: %v", err)
	}
	return f.Overrides
}

func findEmailForContact(name string, apiKontakte []zurichapi.Kontakt) (string, string, bool) {
	nameParts := strings.Fields(name)
	if len(nameParts) < 2 {
		return "", "", false
	}

	for _, kontakt := range apiKontakte {
		apiName := normalize(kontakt.Name)
		apiVorname := normalize(kontakt.Vorname)
		apiCombined := apiName + " " + apiVorname

		matchCount := 0
		for _, part := range nameParts {
			if strings.Contains(apiCombined, strings.ToLower(part)) {
				matchCount++
			}
		}

		if matchCount == len(nameParts) {
			email := ""
			if kontakt.EmailPrivat != "" {
				email = kontakt.EmailPrivat
			} else if kontakt.EmailGeschaeft != "" {
				email = kontakt.EmailGeschaeft
			}
			return email, kontakt.Geschlecht, true
		}
	}

	return "", "", false
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.ToLower(strings.TrimSpace(s))
}

// --- Verify mode ---

func runVerify(c Campaign) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "#\tName\tEmail\tType\tAnrede\tGender\tRole\tParty\tPlatform URL\tSubject\tSource\n")
	_, _ = fmt.Fprintf(w, "-\t----\t-----\t----\t------\t------\t----\t-----\t------------\t-------\t------\n")
	for i, r := range c.Recipients {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			i+1, r.Name, r.Email, r.Type, addressForm(r), r.Gender, r.Role, r.Party, r.PlatformURL, c.RenderSubject(r), r.Source)
	}
	if err := w.Flush(); err != nil {
		log.Fatalf("Failed to flush table: %v", err)
	}

	fmt.Printf("\nTotal: %d recipients\n", len(c.Recipients))
	if len(c.Recipients) > 0 {
		fmt.Printf("\n--- Sample rendered email (recipient #1) ---\n\n")
		fmt.Printf("Subject: %s\n\n", c.RenderSubject(c.Recipients[0]))
		fmt.Println(c.RenderBody(c.Recipients[0]))
	}
}

// --- Preview mode ---

func runPreview(c Campaign) {
	output := os.Stdout
	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close output file: %v\n", err)
			}
		}()
		output = f
	}

	_, _ = fmt.Fprintf(output, "# Campaign preview\n\n---\n\n")

	for i, r := range c.Recipients {
		_, _ = fmt.Fprintf(output, "## %d. %s\n\n", i+1, r.Name)
		_, _ = fmt.Fprintf(output, "**An:** %s\n\n", r.Email)
		_, _ = fmt.Fprintf(output, "**Betreff:** %s\n\n", c.RenderSubject(r))
		_, _ = fmt.Fprintf(output, "%s", c.RenderBody(r))
		_, _ = fmt.Fprintf(output, "\n---\n\n")
	}

	_, _ = fmt.Fprintf(output, "Total: %d emails\n", len(c.Recipients))

	if *outputFile != "" {
		fmt.Fprintf(os.Stderr, "Wrote %d emails to %s\n", len(c.Recipients), *outputFile)
	}
}

// --- Send mode ---

func runSend(c Campaign, testOverride string) {
	gmailAddr := os.Getenv("GMAIL_ADDRESS")
	gmailPass := os.Getenv("GMAIL_APP_PASSWORD")
	if gmailAddr == "" || gmailPass == "" {
		log.Fatal("GMAIL_ADDRESS and GMAIL_APP_PASSWORD environment variables must be set")
	}

	isTest := testOverride != ""
	if isTest {
		fmt.Fprintf(os.Stderr, "🧪 TEST MODE: All emails will be sent to %s\n\n", testOverride)
	} else {
		fmt.Fprintf(os.Stderr, "📧 SENDING %d emails for real\n\n", len(c.Recipients))
	}

	var sent, failed int

	for i, r := range c.Recipients {
		toAddr := r.Email
		if isTest {
			toAddr = testOverride
		}

		body := c.RenderBody(r)
		msg := buildMIMEMessage(gmailAddr, toAddr, c.RenderSubject(r), body)

		err := sendEmail(gmailAddr, gmailPass, toAddr, msg)

		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] ❌ %s <%s>: %v\n", i+1, len(c.Recipients), r.Name, toAddr, err)
			failed++
		} else {
			fmt.Fprintf(os.Stderr, "[%d/%d] ✅ %s <%s>\n", i+1, len(c.Recipients), r.Name, toAddr)
			sent++
		}

		if i < len(c.Recipients)-1 {
			time.Sleep(time.Duration(*delay) * time.Second)
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone: %d sent, %d failed\n", sent, failed)
}

func buildMIMEMessage(from, to, subject, body string) []byte {
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		from, to, subject)
	return []byte(headers + body)
}

func sendEmail(gmailAddr, gmailPass, to string, msg []byte) error {
	host := "smtp.gmail.com"
	addr := host + ":587"

	auth := smtp.PlainAuth("", gmailAddr, gmailPass, host)

	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	tlsConfig := &tls.Config{ServerName: host}
	if err = conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}

	if err = conn.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err = conn.Mail(gmailAddr); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err = conn.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return conn.Quit()
}
