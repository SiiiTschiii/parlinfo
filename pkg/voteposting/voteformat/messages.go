package voteformat

import "github.com/siiitschiii/parlinfo/pkg/votes"

// Msg names one piece of fixed post wording: a label, a connective, a hint.
// Everything a post says that does not come from the source goes through one.
type Msg string

// The wording a Nationalrat post can carry. Messages only the Zürich bodies
// reach — quorum and Ausgabenbremse labels, the Wahlgeschäft notice — stay
// German literals where they are used, because nothing posts them in another
// language and a translation nobody reads is a translation nobody checks.
const (
	MsgAccepted       Msg = "accepted"
	MsgRejected       Msg = "rejected"
	MsgYes            Msg = "yes"
	MsgNo             Msg = "no"
	MsgAbstainShort   Msg = "abstain-short"   // stats line and card, with a dot
	MsgAbsentShort    Msg = "absent-short"    // stats line and card, with a dot
	MsgAbstainHeader  Msg = "abstain-header"  // Fraktion header, without one
	MsgAbsentHeader   Msg = "absent-header"   // Fraktion header, without one
	MsgAbstainLong    Msg = "abstain-long"    // long counts line
	MsgAbsentLong     Msg = "absent-long"     // long counts line
	MsgFraktionen     Msg = "fraktionen"      // "🏛️ Fraktionen (…):"
	MsgVote           Msg = "vote"            // headline without a date
	MsgVoteOn         Msg = "vote-on"         // headline before the date
	MsgVoteNumbered   Msg = "vote-numbered"   // fmt verb: the ordinal
	MsgAuthorsBy      Msg = "authors-by"      // between the business type and its authors
	MsgAnd            Msg = "and"             // before the last of several authors
	MsgAndOthers      Msg = "and-others"      // after a capped author list
	MsgMoreSignatures Msg = "more-signatures" // signatories shed for length, after one was named
	MsgBy             Msg = "by"              // signatories shed for length, none named, no business type
	MsgTookOverBy     Msg = "took-over-by"    // members who took a business over
	MsgCoSignedBy     Msg = "co-signed-by"    // co-signers
	MsgLinkBusiness   Msg = "link-business"
	MsgLinkProtocol   Msg = "link-protocol"
	MsgLinkVideo      Msg = "link-video"
	MsgLinkSitting    Msg = "link-sitting"
	MsgLinkBulletin   Msg = "link-bulletin"
	MsgLinkVote       Msg = "link-vote"
	MsgThreadHint     Msg = "thread-hint"
	MsgCaptionCut     Msg = "caption-cut"   // Instagram, when the caption had to drop votes
	MsgCardOverview   Msg = "card-overview" // fmt verb: how many votes the card lists
	MsgOverviewMore   Msg = "overview-more" // fmt verb: how many rows the card dropped
)

// messages is the wording in each post language. German is the reference: it
// is what every post said before there were other languages, and what a
// message missing elsewhere falls back to — TestMessagesAreComplete keeps that
// from happening by accident.
//
// French and Italian follow parlament.ch's own vocabulary where it has one
// ("Objet", "Oggetto", "Procès-verbal", "Bollettino"), so a reader who clicks
// through finds the same words on the other side.
var messages = map[votes.Language]map[Msg]string{
	votes.German: {
		MsgAccepted:       "Angenommen",
		MsgRejected:       "Abgelehnt",
		MsgYes:            "Ja",
		MsgNo:             "Nein",
		MsgAbstainShort:   "Enth.",
		MsgAbsentShort:    "Abw.",
		MsgAbstainHeader:  "Enth",
		MsgAbsentHeader:   "Abw",
		MsgAbstainLong:    "Enthaltung",
		MsgAbsentLong:     "Abwesend",
		MsgFraktionen:     "Fraktionen",
		MsgVote:           "Abstimmung",
		MsgVoteOn:         "Abstimmung vom",
		MsgVoteNumbered:   "Abstimmung %d",
		MsgAuthorsBy:      "von",
		MsgAnd:            "und",
		MsgAndOthers:      "u. a.",
		MsgMoreSignatures: "Weitere Unterzeichnende",
		MsgBy:             "Von",
		MsgTookOverBy:     "Übernommen von",
		MsgCoSignedBy:     "Mitunterzeichnet von",
		MsgLinkBusiness:   "Geschäft",
		MsgLinkProtocol:   "Protokoll",
		MsgLinkVideo:      "Video",
		MsgLinkSitting:    "Sitzung",
		MsgLinkBulletin:   "Bulletin",
		MsgLinkVote:       "Abstimmung",
		MsgThreadHint:     "Details im Thread",
		MsgCaptionCut:     "Gekürzt – weitere Teilabstimmungen im Link.",
		MsgCardOverview:   "%d Abstimmungen",
		MsgOverviewMore:   "… und %d weitere Abstimmungen",
	},
	votes.French: {
		MsgAccepted:       "Accepté",
		MsgRejected:       "Rejeté",
		MsgYes:            "Oui",
		MsgNo:             "Non",
		MsgAbstainShort:   "Abst.",
		MsgAbsentShort:    "Abs.",
		MsgAbstainHeader:  "Abst",
		MsgAbsentHeader:   "Abs",
		MsgAbstainLong:    "Abstentions",
		MsgAbsentLong:     "Absents",
		MsgFraktionen:     "Groupes",
		MsgVote:           "Vote",
		MsgVoteOn:         "Vote du",
		MsgVoteNumbered:   "Vote %d",
		MsgAuthorsBy:      "de",
		MsgAnd:            "et",
		MsgAndOthers:      "et al.",
		MsgMoreSignatures: "Autres signataires",
		MsgBy:             "De",
		MsgTookOverBy:     "Repris par",
		MsgCoSignedBy:     "Cosigné par",
		MsgLinkBusiness:   "Objet",
		MsgLinkProtocol:   "Procès-verbal",
		MsgLinkVideo:      "Vidéo",
		MsgLinkSitting:    "Séance",
		MsgLinkBulletin:   "Bulletin",
		MsgLinkVote:       "Vote",
		MsgThreadHint:     "Détails dans le fil",
		MsgCaptionCut:     "Abrégé – autres votes dans le lien.",
		MsgCardOverview:   "%d votes",
		MsgOverviewMore:   "… et %d autres votes",
	},
	votes.Italian: {
		MsgAccepted:       "Accettato",
		MsgRejected:       "Respinto",
		MsgYes:            "Sì",
		MsgNo:             "No",
		MsgAbstainShort:   "Ast.",
		MsgAbsentShort:    "Ass.",
		MsgAbstainHeader:  "Ast",
		MsgAbsentHeader:   "Ass",
		MsgAbstainLong:    "Astensioni",
		MsgAbsentLong:     "Assenti",
		MsgFraktionen:     "Gruppi",
		MsgVote:           "Votazione",
		MsgVoteOn:         "Votazione del",
		MsgVoteNumbered:   "Votazione %d",
		MsgAuthorsBy:      "di",
		MsgAnd:            "e",
		MsgAndOthers:      "e altri",
		MsgMoreSignatures: "Altri firmatari",
		MsgBy:             "Di",
		MsgTookOverBy:     "Ripreso da",
		MsgCoSignedBy:     "Cofirmato da",
		MsgLinkBusiness:   "Oggetto",
		MsgLinkProtocol:   "Protocollo",
		MsgLinkVideo:      "Video",
		MsgLinkSitting:    "Seduta",
		MsgLinkBulletin:   "Bollettino",
		MsgLinkVote:       "Votazione",
		MsgThreadHint:     "Dettagli nel thread",
		MsgCaptionCut:     "Abbreviato – altre votazioni nel link.", //nolint:misspell // Italian
		MsgCardOverview:   "%d votazioni",
		MsgOverviewMore:   "… e altre %d votazioni",
	},
}

// T returns a message in a language, falling back to German for a language
// or a message the table does not have.
func T(l votes.Language, m Msg) string {
	if s, ok := messages[l.OrGerman()][m]; ok {
		return s
	}
	return messages[votes.German][m]
}

// groupLanguage is the language a group's post is written in.
func groupLanguage(group []votes.Vote) votes.Language {
	if len(group) == 0 {
		return votes.German
	}
	return group[0].Language.OrGerman()
}
