package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/site"
)

// The Nationalrat campaign writes to every member in their own language and
// names the accounts on which CH Parl Info tags them. Its recipients file is
// written by cmd/fetch_nationalrat_recipients; the handles come from the
// curated contacts, so a member is only ever told about a handle the bots
// would actually tag.

const nationalratAudienceKey = "nationalrat"

var nationalratContactsPath = flag.String("nationalrat-contacts", contacts.PathFor("nationalrat"),
	"Nationalrat contacts YAML, for the handles the --audience nationalrat emails name")

// sessionStart is the first day of the session the emails announce, written
// the way each language writes a date in running text.
var sessionStart = map[string]string{
	"de": "30. November",
	"fr": "30 novembre",
	"it": "30 novembre",
}

// taggingPlatforms are the platforms whose Nationalrat posts tag members, in
// the order the emails name them. Facebook only names people: a Page cannot
// mention a profile through the API, see pkg/voteposting/platforms/facebook.
var taggingPlatforms = []struct{ Key, Name string }{
	{"instagram", "Instagram"},
	{"x", "X"},
	{"bluesky", "Bluesky"},
}

var platformEmoji = map[string]string{"Instagram": "📸", "X": "✖️", "Bluesky": "🔵"}

// taggedAccount is one of the member's own accounts that the bots tag.
type taggedAccount struct {
	Platform string // display name, e.g. "Instagram"
	Handle   string // "@annarosenwasser"
}

// buildNationalratCampaign joins the recipients file with the contacts file:
// the one says where and how to write, the other which handles to name.
func buildNationalratCampaign(recipientsFile string) Campaign {
	mapper, err := contacts.LoadContacts(*nationalratContactsPath)
	if err != nil {
		log.Fatalf("Failed to load Nationalrat contacts: %v", err)
	}

	recipients := loadAudienceRecipients(recipientsFile, true)
	tagged := 0
	for i := range recipients {
		r := &recipients[i]
		switch r.Language {
		case "de", "fr", "it":
		default:
			log.Fatalf("%s: language %q in %s; use de, fr or it", r.Name, r.Language, recipientsFile)
		}
		if _, ok := mapper.GetContact(r.Name); !ok {
			fmt.Fprintf(os.Stderr, "  ⚠️  Not in %s, so no handle named: %s\n", *nationalratContactsPath, r.Name)
		}
		r.Tagged = taggedAccounts(mapper, r.Name)
		if len(r.Tagged) > 0 {
			tagged++
			r.PlatformURL = mapper.GetPlatformURLs(r.Name, strings.ToLower(r.Tagged[0].Platform))[0]
		}
		r.Source = "file+contacts"
	}
	fmt.Fprintf(os.Stderr, "%d recipients are tagged on at least one platform, %d only named\n", tagged, len(recipients)-tagged)

	return Campaign{
		Recipients:    recipients,
		RenderSubject: func(r Recipient) string { return nationalratWordsFor(r.Language).Subject },
		RenderBody:    nationalratBody,
	}
}

// taggedAccounts lists the member's verified handles on the platforms that tag.
func taggedAccounts(mapper *contacts.Mapper, name string) []taggedAccount {
	var out []taggedAccount
	for _, p := range taggingPlatforms {
		urls := mapper.GetPlatformURLs(name, p.Key)
		if len(urls) == 0 {
			continue
		}
		out = append(out, taggedAccount{Platform: p.Name, Handle: handleFromURL(urls[0])})
	}
	return out
}

// handleFromURL turns a profile URL into the handle people know it by:
// https://www.instagram.com/annarosenwasser/ -> @annarosenwasser,
// https://bsky.app/profile/x.bsky.social -> @x.bsky.social.
func handleFromURL(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	seg := u[strings.LastIndex(u, "/")+1:]
	return "@" + strings.TrimPrefix(seg, "@")
}

// chparlAccount is CH Parl Info's own account on a platform in a language,
// falling back to the German one where there is no other (X). The second
// result says whether it fell back.
func chparlAccount(platform, lang string) (site.Account, bool) {
	var fallback *site.Account
	for _, b := range site.Brands {
		if b.Key != "ch" {
			continue
		}
		for i, a := range b.Accounts {
			if a.Platform != platform {
				continue
			}
			if a.Lang == lang {
				return a, false
			}
			if a.Lang == "de" {
				fallback = &b.Accounts[i]
			}
		}
	}
	if fallback == nil {
		log.Fatalf("site.Brands has no CH Parl Info account on %s", platform)
	}
	return *fallback, true
}

// nationalratWords is one language's text. Each language is written out in
// full rather than assembled from shared phrases, so each reads as written in
// it rather than translated word by word.
type nationalratWords struct {
	Subject string
	// Intro presents the project.
	Intro string
	// TaggedItem renders one platform and handle, e.g. "auf Instagram als %s".
	TaggedItem string
	And        string
	// Tagged says where the member is tagged; %s is the list of TaggedItems.
	Tagged string
	// Untagged replaces Tagged for a member with no handle the bots tag.
	Untagged string
	// Where introduces our accounts; %s is the session's first day.
	Where string
	// InGerman marks an account that posts in German, for a reader of another
	// language.
	InGerman string
	// Colon follows a platform's name in the account list: French puts a
	// space before it.
	Colon string
	// AllAccounts points at the parlinfo.ch brand page; %s is its URL.
	AllAccounts string
	Close       string
	Thanks      string
}

func nationalratWordsFor(lang string) nationalratWords {
	switch lang {
	case "fr":
		return frWords
	case "it":
		return itWords
	default:
		return deSieWords
	}
}

var deSieWords = nationalratWords{
	Subject:    "CH Parl Info – die Abstimmungen des Nationalrats auf Social Media",
	Intro:      "Ich möchte Ihnen vor der Wintersession kurz CH Parl Info vorstellen, ein ehrenamtliches Civic-Tech-Projekt. Es bringt die Abstimmungen des Nationalrats automatisch auf Social Media, auf Deutsch, Französisch und Italienisch. Zu jeder Abstimmung zeigen wir das Resultat, wie die einzelnen Fraktionen gestimmt haben und wo das Geschäft im Prozess steht.",
	TaggedItem: "auf %s als %s",
	And:        "und",
	Tagged:     "Stimmt der Nationalrat über einen Vorstoss ab, den Sie eingereicht oder mitunterzeichnet haben, markieren wir Sie im Post: %s. So erreichen Ihre Vorstösse und ihr Ausgang auch Leute, die nie auf der Parlamentswebsite nachschauen würden.",
	Untagged:   "Stimmt der Nationalrat über einen Vorstoss ab, den Sie eingereicht oder mitunterzeichnet haben, nennen wir Sie im Post. Auf Instagram, X und Bluesky markieren wir Sie auch, sobald wir Ihren Account dort kennen: Wenn Sie mir den Link schicken, ergänze ich ihn gern.",
	Where:      "Ab dem %s finden Sie die Abstimmungen der Wintersession hier:",
	InGerman:   "",
	// The German brand page; the language switcher there leads to the others.
	Colon:       ":",
	AllAccounts: "Alle Accounts, auch auf Französisch und Italienisch: %s",
	Close:       "Ich würde mich freuen, wenn Sie dem Account folgen und einen Post teilen, wenn er eines Ihrer Geschäfte betrifft. Und ich bin gespannt auf Ihr Feedback: Was würde die Posts für Sie oder Ihre Wähler nützlicher machen?",
	Thanks:      "Freundliche Grüsse",
}

var deDuWords = nationalratWords{
	Subject:     "CH Parl Info – die Abstimmungen des Nationalrats auf Social Media",
	Intro:       "Ich möchte dir vor der Wintersession kurz CH Parl Info vorstellen, ein ehrenamtliches Civic-Tech-Projekt. Es bringt die Abstimmungen des Nationalrats automatisch auf Social Media, auf Deutsch, Französisch und Italienisch. Zu jeder Abstimmung zeigen wir das Resultat, wie die einzelnen Fraktionen gestimmt haben und wo das Geschäft im Prozess steht.",
	TaggedItem:  "auf %s als %s",
	And:         "und",
	Tagged:      "Stimmt der Nationalrat über einen Vorstoss ab, den du eingereicht oder mitunterzeichnet hast, markieren wir dich im Post: %s. So erreichen deine Vorstösse und ihr Ausgang auch Leute, die nie auf der Parlamentswebsite nachschauen würden.",
	Untagged:    "Stimmt der Nationalrat über einen Vorstoss ab, den du eingereicht oder mitunterzeichnet hast, nennen wir dich im Post. Auf Instagram, X und Bluesky markieren wir dich auch, sobald wir deinen Account dort kennen: Wenn du mir den Link schickst, ergänze ich ihn gern.",
	Where:       "Ab dem %s findest du die Abstimmungen der Wintersession hier:",
	InGerman:    "",
	Colon:       ":",
	AllAccounts: "Alle Accounts, auch auf Französisch und Italienisch: %s",
	Close:       "Ich würde mich freuen, wenn du dem Account folgst und einen Post teilst, wenn er eines deiner Geschäfte betrifft. Und ich bin gespannt auf dein Feedback: Was würde die Posts für dich oder deine Wähler nützlicher machen?",
	Thanks:      "Liebe Grüsse",
}

var frWords = nationalratWords{
	Subject:     "CH Parl Info – les votes du Conseil national sur les réseaux sociaux",
	Intro:       "Avant la session d'hiver, je me permets de vous présenter CH Parl Info, un projet civic tech bénévole. Il publie automatiquement les votes du Conseil national sur les réseaux sociaux, en français, en allemand et en italien. Pour chaque vote, nous montrons le résultat, la position de chaque groupe parlementaire et l'étape où en est l'objet.",
	TaggedItem:  "sur %s (%s)",
	And:         "et",
	Tagged:      "Lorsque le Conseil national vote sur une intervention que vous avez déposée ou cosignée, nous vous identifions dans la publication : %s. Vos interventions et leur issue atteignent ainsi des personnes qui ne consulteraient jamais le site du Parlement.",
	Untagged:    "Lorsque le Conseil national vote sur une intervention que vous avez déposée ou cosignée, nous vous nommons dans la publication. Sur Instagram, X et Bluesky, nous vous identifions aussi dès que nous connaissons votre compte : n'hésitez pas à m'envoyer le lien, je l'ajouterai volontiers.",
	Where:       "Dès le %s, vous trouverez les votes de la session d'hiver ici :",
	InGerman:    " (en allemand)",
	Colon:       " :",
	AllAccounts: "Tous les comptes, aussi en allemand et en italien : %s",
	Close:       "Je serais ravi que vous suiviez le compte et que vous partagiez une publication lorsqu'elle concerne l'un de vos objets. Et vos remarques m'intéressent : qu'est-ce qui rendrait ces publications plus utiles pour vous ou pour vos électeurs ?",
	Thanks:      "Meilleures salutations",
}

//nolint:misspell // Italian: "intervento"
var itWords = nationalratWords{
	Subject:     "CH Parl Info – le votazioni del Consiglio nazionale sui social media",
	Intro:       "in vista della sessione invernale mi permetto di presentarLe CH Parl Info, un progetto civic tech volontario. Pubblica automaticamente le votazioni del Consiglio nazionale sui social media, in italiano, tedesco e francese. Per ogni votazione mostriamo il risultato, come hanno votato i singoli gruppi parlamentari e a che punto si trova l'oggetto.",
	TaggedItem:  "su %s (%s)",
	And:         "e",
	Tagged:      "Quando il Consiglio nazionale vota su un intervento che ha depositato o cofirmato, La menzioniamo nel post: %s. Così i Suoi interventi e il loro esito raggiungono anche persone che non consulterebbero mai il sito del Parlamento.",
	Untagged:    "Quando il Consiglio nazionale vota su un intervento che ha depositato o cofirmato, La citiamo nel post. Su Instagram, X e Bluesky La menzioniamo anche non appena conosciamo il Suo account: se mi invia il link, lo aggiungo volentieri.",
	Where:       "Dal %s trova le votazioni della sessione invernale qui:",
	InGerman:    " (in tedesco)",
	Colon:       ":",
	AllAccounts: "Tutti gli account, anche in tedesco e francese: %s",
	Close:       "Mi farebbe piacere se seguisse l'account e condividesse un post quando riguarda uno dei Suoi oggetti. E sono curioso di sapere cosa ne pensa: cosa renderebbe i post più utili per Lei o per i Suoi elettori?",
	Thanks:      "Cordiali saluti",
}

// nationalratGreeting addresses a member in their language. German follows the
// other campaigns (du or Sie); French and Italian are always formal.
func nationalratGreeting(r Recipient) string {
	name := r.Lastname
	if name == "" {
		name = r.Name
	}
	female := r.Gender == "weiblich"
	switch r.Language {
	case "fr":
		if female {
			return "Bonjour Madame " + name + ","
		}
		return "Bonjour Monsieur " + name + ","
	case "it":
		if female {
			return "Gentile signora " + name + ","
		}
		return "Gentile signor " + name + ","
	default:
		return greetingFor(r)
	}
}

func nationalratBody(r Recipient) string {
	w := nationalratWordsFor(r.Language)
	if r.Language == "de" && !r.Formal {
		w = deDuWords
	}

	tagLine := w.Untagged
	if len(r.Tagged) > 0 {
		items := make([]string, len(r.Tagged))
		for i, t := range r.Tagged {
			items[i] = fmt.Sprintf(w.TaggedItem, t.Platform, t.Handle)
		}
		tagLine = fmt.Sprintf(w.Tagged, joinList(items, w.And))
	}

	// Our account on each platform the member is tagged on; Instagram, the
	// largest, for a member tagged nowhere.
	platforms := []string{"Instagram"}
	if len(r.Tagged) > 0 {
		platforms = platforms[:0]
		for _, t := range r.Tagged {
			platforms = append(platforms, t.Platform)
		}
	}
	var accounts strings.Builder
	for _, p := range platforms {
		a, german := chparlAccount(p, r.Language)
		note := ""
		if german {
			note = w.InGerman
		}
		fmt.Fprintf(&accounts, "%s %s%s %s%s\n", platformEmoji[p], p, w.Colon, a.URL, note)
	}
	brandPage := fmt.Sprintf("https://%s/%s/ch/", site.Domain, r.Language)

	return fmt.Sprintf(`%s

%s

%s

%s
%s%s

%s

%s
Christof
https://www.linkedin.com/in/christof-gerber/
`,
		nationalratGreeting(r),
		w.Intro,
		tagLine,
		fmt.Sprintf(w.Where, sessionStart[r.Language]),
		accounts.String(), fmt.Sprintf(w.AllAccounts, brandPage),
		w.Close,
		w.Thanks)
}

// joinList joins items as running text: "a", "a und b", "a, b und c".
func joinList(items []string, and string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " " + and + " " + items[len(items)-1]
	}
}
