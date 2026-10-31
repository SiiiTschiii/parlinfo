package zurichapi

import "encoding/xml"

// Geschaeft represents a council business/matter from the Zurich city council
type Geschaeft struct {
	OBJGUID          string   `xml:"OBJ_GUID,attr"`
	GRNr             string   `xml:"GRNr"`
	Titel            string   `xml:"Titel"`
	Geschaeftsart    string   `xml:"Geschaeftsart"`
	Geschaeftsstatus string   `xml:"Geschaeftsstatus"`
	Dringlich        bool     `xml:"Dringlich"`
	Beginn           Zeitraum `xml:"Beginn"`
	PendentBei       string   `xml:"PendentBei"`
	// Erstunterzeichner holds one entry in practice, and none on a handful of
	// older Postulate. A slice keeps "absent" distinguishable from "empty name".
	Erstunterzeichner []KontaktGremium `xml:"Erstunterzeichner>KontaktGremium"`
	Mitunterzeichner  []KontaktGremium `xml:"Mitunterzeichner>KontaktGremium"`
	// AnzahlMitunterzeichnende counts co-signers *beyond* those listed in
	// Mitunterzeichner — the council's own page shows the named signers and
	// this number side by side. It is nil-marked on most Vorstösse, which
	// decodes to 0.
	AnzahlMitunterzeichnende *int      `xml:"AnzahlMitunterzeichnende"`
	Ablaufschritte           []Aufgabe `xml:"Ablaufschritte>Aufgabe"`
}

// Zeitraum is a PARIS date range. Start is ISO-like ("2026-09-30T00:00:00.000"),
// Text the Swiss rendering of the same day ("30.09.2026").
type Zeitraum struct {
	Start string `xml:"Start"`
	Text  string `xml:"Text"`
}

// KontaktGremium is a signer of a Vorstoss: either a member, or — when
// IstGremium is set — a whole Fraktion, in which case Name is the party
// ("SP") and Partei is empty.
type KontaktGremium struct {
	OBJGUID    string `xml:"OBJ_GUID,attr"`
	Name       string `xml:"Name"`
	Partei     string `xml:"Partei"`
	IstGremium bool   `xml:"IstGremium"`
}

// Aufgabe is one step in a Geschäft's procedure, e.g. "Eingang, Frist 3
// Monate" or "Stadtrat, Entgegennahme".
type Aufgabe struct {
	AblaufschrittName string   `xml:"AblaufschrittName"`
	Datum             Zeitraum `xml:"Datum"`
	FristBis          Zeitraum `xml:"FristBis"`
}

// GeschaeftSearchResponse represents the XML response from the geschaeft API
type GeschaeftSearchResponse struct {
	XMLName xml.Name       `xml:"SearchDetailResponse"`
	NumHits int            `xml:"numHits,attr"`
	Hits    []GeschaeftHit `xml:"Hit"`
}

// GeschaeftHit represents a single result in the geschaeft search response
type GeschaeftHit struct {
	Geschaeft Geschaeft `xml:"Geschaeft"`
}

// Kontakt represents a contact from the Zurich city council
type Kontakt struct {
	OBJGUID                string            `xml:"OBJ_GUID,attr"`
	SEQ                    string            `xml:"SEQ,attr"`
	AltsystemID            string            `xml:"AltsystemID"`
	NameVorname            string            `xml:"NameVorname"`
	Name                   string            `xml:"Name"`
	Vorname                string            `xml:"Vorname"`
	Partei                 string            `xml:"Partei"`
	ParteiGuid             string            `xml:"ParteiGuid"`
	Fraktion               string            `xml:"Fraktion"`
	Wahlkreis              string            `xml:"Wahlkreis"`
	Wohnkreis              string            `xml:"Wohnkreis"`
	Beruf                  string            `xml:"Beruf"`
	Jahrgang               *int              `xml:"Jahrgang"`
	Todesjahr              *int              `xml:"Todesjahr"`
	Geschlecht             string            `xml:"Geschlecht"`
	EmailGeschaeft         string            `xml:"EmailGeschaeft"`
	EmailPrivat            string            `xml:"EmailPrivat"`
	TelefonGeschaeft       string            `xml:"TelefonGeschaeft"`
	TelefonPrivat          string            `xml:"TelefonPrivat"`
	TelefonMobileGeschaeft string            `xml:"TelefonMobileGeschaeft"`
	TelefonMobilePrivat    string            `xml:"TelefonMobilePrivat"`
	HomepageGeschaeft      string            `xml:"HomepageGeschaeft"`
	HomepagePrivat         string            `xml:"HomepagePrivat"`
	SozialeMedien          SozialeMedienList `xml:"SozialeMedien"`
}

// SozialeMedienList represents a list of social media communications
type SozialeMedienList struct {
	Kommunikation []SozialesMedium `xml:"Kommunikation"`
}

// SozialesMedium represents a single social media entry
type SozialesMedium struct {
	OBJGUID string `xml:"OBJ_GUID,attr"`
	Adresse string `xml:"Adresse"`
	Typ     string `xml:"Typ"`
}

// KontaktSearchResponse represents the XML response from the kontakt API
type KontaktSearchResponse struct {
	XMLName xml.Name     `xml:"SearchDetailResponse"`
	NumHits int          `xml:"numHits,attr"`
	Hits    []KontaktHit `xml:"Hit"`
}

// KontaktHit represents a single result in the kontakt search response
type KontaktHit struct {
	Kontakt Kontakt `xml:"Kontakt"`
}

// Abstimmung represents a vote from the Zurich city council
type Abstimmung struct {
	OBJGUID                    string `xml:"OBJ_GUID,attr"`
	SEQ                        string `xml:"SEQ,attr"`
	SitzungGuid                string `xml:"SitzungGuid"`
	SitzungTitel               string `xml:"SitzungTitel"`
	SitzungDatum               string `xml:"SitzungDatum"`
	TraktandumGuid             string `xml:"TraktandumGuid"`
	TraktandumNr               string `xml:"TraktandumNr"`
	TraktandumTitel            string `xml:"TraktandumTitel"`
	GeschaeftGuid              string `xml:"GeschaeftGuid"`
	GeschaeftTitel             string `xml:"GeschaeftTitel"`
	GeschaeftGrNr              string `xml:"GeschaeftGrNr"`
	GeschaeftRatsgeschaeftsart string `xml:"GeschaeftRatsgeschaeftsart"`
	Abstimmungstitel           string `xml:"Abstimmungstitel"`
	Nummer                     string `xml:"Nummer"`
	Abstimmungstyp             string `xml:"Abstimmungstyp"`
	AnzahlJa                   *int   `xml:"Anzahl_Ja"`
	AnzahlNein                 *int   `xml:"Anzahl_Nein"`
	AnzahlEnthaltung           *int   `xml:"Anzahl_Enthaltung"`
	AnzahlAbwesend             *int   `xml:"Anzahl_Abwesend"`
	AnzahlA                    *int   `xml:"Anzahl_A"`
	AnzahlB                    *int   `xml:"Anzahl_B"`
	AnzahlC                    *int   `xml:"Anzahl_C"`
	AnzahlD                    *int   `xml:"Anzahl_D"`
	AnzahlE                    *int   `xml:"Anzahl_E"`
	Schlussresultat            string `xml:"Schlussresultat"`
	Stimmabgaben               struct {
		Stimmabgabe []Stimmabgabe `xml:"Stimmabgabe"`
	} `xml:"Stimmabgaben"`
}

// Stimmabgabe represents an individual vote by a council member
type Stimmabgabe struct {
	OBJGUID              string `xml:"OBJ_GUID,attr"`
	KontaktGuid          string `xml:"KontaktGuid"`
	Name                 string `xml:"Name"`
	Vorname              string `xml:"Vorname"`
	Partei               string `xml:"Partei"`
	Fraktion             string `xml:"Fraktion"`
	Spezialfunktion      string `xml:"Spezialfunktion"`
	Geschlecht           string `xml:"Geschlecht"`
	Alter                *int   `xml:"Alter"`
	Abstimmungsverhalten string `xml:"Abstimmungsverhalten"`
}

// AbstimmungSearchResponse represents the XML response from the abstimmung API
type AbstimmungSearchResponse struct {
	XMLName xml.Name        `xml:"SearchDetailResponse"`
	NumHits int             `xml:"numHits,attr"`
	Hits    []AbstimmungHit `xml:"Hit"`
}

// AbstimmungHit represents a single result in the abstimmung search response
type AbstimmungHit struct {
	Abstimmung Abstimmung `xml:"Abstimmung"`
}

// Behoerdenmandat represents a mandate from the Zurich city council
type Behoerdenmandat struct {
	OBJGUID  string `xml:"OBJ_GUID,attr"`
	SEQ      string `xml:"SEQ,attr"`
	Name     string `xml:"Name"`
	Vorname  string `xml:"Vorname"`
	Funktion string `xml:"Funktion"`
	Gremium  string `xml:"Gremium"`
	Dauer    struct {
		Start string `xml:"Start"`
		End   string `xml:"End"`
	} `xml:"Dauer"`
	Partei   string `xml:"Partei"`
	Fraktion string `xml:"Fraktion"`
}

// BehoerdenmandatSearchResponse represents the XML response from the behoerdenmandat API
type BehoerdenmandatSearchResponse struct {
	XMLName xml.Name             `xml:"SearchDetailResponse"`
	NumHits int                  `xml:"numHits,attr"`
	Hits    []BehoerdenmandatHit `xml:"Hit"`
}

// BehoerdenmandatHit represents a single result in the behoerdenmandat search response
// Note: The XML uses "Behordenmandat" (without 'e') despite the API being called "behoerdenmandat"
type BehoerdenmandatHit struct {
	Behoerdenmandat Behoerdenmandat `xml:"Behordenmandat"`
}
