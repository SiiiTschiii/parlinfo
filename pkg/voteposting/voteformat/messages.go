package voteformat

import "github.com/siiitschiii/parlinfo/pkg/votes"

// Words is the fixed wording of a post in one language: every label,
// connective and hint a post carries that does not come from the source.
//
// Wording only the Zürich bodies reach — quorum and Ausgabenbremse labels, the
// Wahlgeschäft notice — stays German literals where it is used, because
// nothing posts it in another language and a translation nobody reads is a
// translation nobody checks.
type Words struct {
	Accepted       string
	Rejected       string
	Yes            string
	No             string
	AbstainShort   string // stats line and card, with a dot
	AbsentShort    string // stats line and card, with a dot
	AbstainHeader  string // Fraktion header, without one
	AbsentHeader   string // Fraktion header, without one
	AbstainLong    string // long counts line
	AbsentLong     string // long counts line
	Fraktionen     string // "🏛️ Fraktionen (…):"
	Vote           string // headline without a date
	VoteOn         string // headline before the date
	VoteNumbered   string // fmt verb: the ordinal
	AuthorsBy      string // between the business type and its authors
	ByFraktion     string // between the business type and the Fraktion that filed it
	ByCanton       string // fmt verb: the canton that filed a Standesinitiative, after the type
	LeaveGiven     string // an initiative's verdict when it carried
	NoLeaveGiven   string // an initiative's verdict when it did not
	And            string // before the last of several authors
	AndOthers      string // after a capped author list
	MoreSignatures string // signatories shed for length, after one was named
	By             string // signatories shed for length, none named, no business type
	TookOverBy     string // members who took a business over
	CoSignedBy     string // co-signers
	LinkBusiness   string
	LinkProtocol   string
	LinkVideo      string
	LinkSitting    string
	LinkBulletin   string
	LinkVote       string
	ThreadHint     string
	CaptionCut     string // Instagram, when the caption had to drop votes
	CardOverview   string // fmt verb: how many votes the card lists
	OverviewMore   string // fmt verb: how many rows the card dropped
}

// german is the reference: it is what every post said before there were other
// languages, and what any other language falls back to.
//
// French and Italian follow parlament.ch's own vocabulary where it has one
// ("Objet", "Oggetto", "Procès-verbal", "Bollettino"), so a reader who clicks
// through finds the same words on the other side. TestWordsAreComplete keeps
// a field from being left empty, which would print nothing in a post.
var (
	german = Words{
		Accepted:       "Angenommen",
		Rejected:       "Abgelehnt",
		Yes:            "Ja",
		No:             "Nein",
		AbstainShort:   "Enth.",
		AbsentShort:    "Abw.",
		AbstainHeader:  "Enth",
		AbsentHeader:   "Abw",
		AbstainLong:    "Enthaltung",
		AbsentLong:     "Abwesend",
		Fraktionen:     "Fraktionen",
		Vote:           "Abstimmung",
		VoteOn:         "Abstimmung vom",
		VoteNumbered:   "Abstimmung %d",
		AuthorsBy:      "von",
		ByFraktion:     "der",
		ByCanton:       " des Kantons %s",
		LeaveGiven:     "Folge gegeben",
		NoLeaveGiven:   "Keine Folge gegeben",
		And:            "und",
		AndOthers:      "u. a.",
		MoreSignatures: "Weitere Unterzeichnende",
		By:             "Von",
		TookOverBy:     "Übernommen von",
		CoSignedBy:     "Mitunterzeichnet von",
		LinkBusiness:   "Geschäft",
		LinkProtocol:   "Protokoll",
		LinkVideo:      "Video",
		LinkSitting:    "Sitzung",
		LinkBulletin:   "Bulletin",
		LinkVote:       "Abstimmung",
		ThreadHint:     "Details im Thread",
		CaptionCut:     "Gekürzt – weitere Teilabstimmungen im Link.",
		CardOverview:   "%d Abstimmungen",
		OverviewMore:   "… und %d weitere Abstimmungen",
	}
	french = Words{
		Accepted:       "Accepté",
		Rejected:       "Rejeté",
		Yes:            "Oui",
		No:             "Non",
		AbstainShort:   "Abst.",
		AbsentShort:    "Abs.",
		AbstainHeader:  "Abst",
		AbsentHeader:   "Abs",
		AbstainLong:    "Abstentions",
		AbsentLong:     "Absents",
		Fraktionen:     "Groupes",
		Vote:           "Vote",
		VoteOn:         "Vote du",
		VoteNumbered:   "Vote %d",
		AuthorsBy:      "de",
		ByFraktion:     "du",
		ByCanton:       " (%s)",
		LeaveGiven:     "Suite donnée",
		NoLeaveGiven:   "Pas de suite donnée",
		And:            "et",
		AndOthers:      "et al.",
		MoreSignatures: "Autres signataires",
		By:             "De",
		TookOverBy:     "Repris par",
		CoSignedBy:     "Cosigné par",
		LinkBusiness:   "Objet",
		LinkProtocol:   "Procès-verbal",
		LinkVideo:      "Vidéo",
		LinkSitting:    "Séance",
		LinkBulletin:   "Bulletin",
		LinkVote:       "Vote",
		ThreadHint:     "Détails dans le fil",
		CaptionCut:     "Abrégé – autres votes dans le lien.",
		CardOverview:   "%d votes",
		OverviewMore:   "… et %d autres votes",
	}
	italian = Words{
		Accepted:       "Accettato",
		Rejected:       "Respinto",
		Yes:            "Sì",
		No:             "No",
		AbstainShort:   "Ast.",
		AbsentShort:    "Ass.",
		AbstainHeader:  "Ast",
		AbsentHeader:   "Ass",
		AbstainLong:    "Astensioni",
		AbsentLong:     "Assenti",
		Fraktionen:     "Gruppi",
		Vote:           "Votazione",
		VoteOn:         "Votazione del",
		VoteNumbered:   "Votazione %d",
		AuthorsBy:      "di",
		ByFraktion:     "del",
		ByCanton:       " (%s)",
		LeaveGiven:     "Dato seguito",
		NoLeaveGiven:   "Non dato seguito",
		And:            "e",
		AndOthers:      "e altri",
		MoreSignatures: "Altri firmatari",
		By:             "Di",
		TookOverBy:     "Ripreso da",
		CoSignedBy:     "Cofirmato da",
		LinkBusiness:   "Oggetto",
		LinkProtocol:   "Protocollo",
		LinkVideo:      "Video",
		LinkSitting:    "Seduta",
		LinkBulletin:   "Bollettino",
		LinkVote:       "Votazione",
		ThreadHint:     "Dettagli nel thread",
		CaptionCut:     "Abbreviato – altre votazioni nel link.", //nolint:misspell // Italian
		CardOverview:   "%d votazioni",
		OverviewMore:   "… e altre %d votazioni",
	}
)

// W returns the wording for a language, German for any other.
func W(l votes.Language) Words {
	switch l {
	case votes.French:
		return french
	case votes.Italian:
		return italian
	default:
		return german
	}
}

// groupLanguage is the language a group's post is written in.
func groupLanguage(group []votes.Vote) votes.Language {
	if len(group) == 0 {
		return votes.German
	}
	return group[0].Language.OrGerman()
}
