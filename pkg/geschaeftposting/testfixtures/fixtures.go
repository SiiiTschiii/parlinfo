// Package testfixtures holds real Geschäfte from the PARIS API, condensed to
// the fields geschaeftposting reads, for format and selection tests.
//
// Each is a genuine filing, fetched from /api/geschaeft and cut back to its
// state on the day it was filed: only the "Eingang" Ablaufschritt is kept,
// because that is all a Vorstoss carries when the bot first sees it. The two
// Dringlich fixtures are the exception that proves the headline: their urgency
// was declared a sitting or two later, and is kept so the urgent form is
// exercised on real titles and signers. Names keep
// the non-breaking space PARIS puts between first and last name, since
// normalising it is part of what the formatters must do.
package testfixtures

import "github.com/siiitschiii/parlinfo/pkg/zurichapi"

func intPtr(n int) *int { return &n }

// MotionMember is 2026/541. Motion filed by a member with two named co-signers and no further count.
func MotionMember() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "9be11ce93061407f8b0f2d6038b11e0e",
		GRNr:              "2026/541",
		Titel:             "Ausscheidung einer der im kommunalen Richtplan Siedlung, Landschaft, öffentliche Bauten und Anlagen festgelegten Grössenordnung entsprechenden Freihaltezone auf dem Schlachthofareal",
		Geschaeftsart:     "Motion",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "4e3a94fca0a944579778489fcb088278", Name: "Brigitte Fürer", Partei: "Grüne"}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "c8223578a62d4cadac429d5f22429c27", Name: "Tanja Maag", Partei: "AL"},
			{OBJGUID: "967c0420fe744c048d5472f51a601167", Name: "Pascal Lamprecht", Partei: "SP"},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 6 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "2027-03-30T00:00:00.000", Text: "30.03.2027"},
		}},
	}
}

// MotionCounted is 2026/529. Motion with named co-signers and eight more counted but not named.
func MotionCounted() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "90ad8eaf4a25464787e52022880d381f",
		GRNr:              "2026/529",
		Titel:             "Aufbau einer dezentralen Infrastruktur zur Speicherung und Nutzung von Regenwasser aus Starkregen-Ereignissen für Hitze- und Trockenperioden",
		Geschaeftsart:     "Motion",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-23T00:00:00.000", Text: "23.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "f8196acd43e645319c5e12f7e8a0b3fc", Name: "Selina Frey", Partei: "GLP"}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "274ef2af504040d68e5da5867fd9dbf8", Name: "Guy Krayenbühl", Partei: "GLP"},
			{OBJGUID: "f001aea011ef400f8794350121275505", Name: "Marisa Kappeler-Schudel", Partei: "GLP"},
		},
		AnzahlMitunterzeichnende: intPtr(8),
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 6 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-23T00:00:00.000", Text: "23.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// MotionFraktion is 2026/458. Motion filed by a whole Fraktion, which PARIS records as a signer named after the party.
func MotionFraktion() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "bd3a2b2078ca489bb3d442809cf82111",
		GRNr:              "2026/458",
		Titel:             "Kopplung des Mietzinses an die Einkommensentwicklung der Mieterschaft, Ergänzung der Verordnung über die Grundsätze der Vermietung von städtischen Wohnungen (AS 846.100; VGV)",
		Geschaeftsart:     "Motion",
		Beginn:            zurichapi.Zeitraum{Start: "2026-08-26T00:00:00.000", Text: "26.08.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "82f765845175438f83c542a0ec0ce386", Name: "FDP", IstGremium: true}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "8a2fb92e26e4436aab80a3618106dfcd", Name: "SVP", IstGremium: true},
			{OBJGUID: "b511df3c164243e98466ea0c7e30d592", Name: "Die Mitte", IstGremium: true},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 6 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-08-26T00:00:00.000", Text: "26.08.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// MotionDringlich is 2026/347. Motion already marked Dringlich, with eleven unnamed co-signers.
func MotionDringlich() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "db96dcdd4fbd49a0b90a67b04825f91a",
		GRNr:              "2026/347",
		Titel:             "Optimierung der Strukturen und Prozesse der Stadt zur Bewältigung von extremen Hitzeereignissen",
		Geschaeftsart:     "Motion",
		Dringlich:         true,
		Beginn:            zurichapi.Zeitraum{Start: "2026-07-01T00:00:00.000", Text: "01.07.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "9cd24adc27264051be2d0f193a6554f0", Name: "Janina Flückiger", Partei: "Grüne"}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "5ef16289476643e9a2a7ee234f58714a", Name: "Leonora Seiler", Partei: "Grüne"},
		},
		AnzahlMitunterzeichnende: intPtr(11),
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 6 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-07-01T00:00:00.000", Text: "01.07.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// PostulatFraktionen is 2026/548. Postulat filed by one Fraktion and co-signed by two others.
func PostulatFraktionen() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "d5fbf7666c9340b791cc785a8237564b",
		GRNr:              "2026/548",
		Titel:             "Verhinderung der Einführung oder Massnahmen zur Regulierung von selbstfahrenden Robotaxis",
		Geschaeftsart:     "Postulat",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "f179673bb7c7404b8330303715862f1f", Name: "SP", IstGremium: true}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "8bc3b6c03c914b13bfea67e1e2855339", Name: "AL", IstGremium: true},
			{OBJGUID: "b415a00ff14141f7ae632fe01f06b4c4", Name: "Grüne", IstGremium: true},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "2027-01-06T00:00:00.000", Text: "06.01.2027"},
		}},
	}
}

// PostulatManySigners is 2026/546. Postulat with five named co-signers and a long title — the most names any recent Vorstoss carries.
func PostulatManySigners() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "fa30b4570fdd41c7b0c57794f6eb78e4",
		GRNr:              "2026/546",
		Titel:             "Förderung und Ausbau von Angeboten zur Sicherstellung einer mittel- bis langfristig sicheren Unterkunft für Opfer von häuslicher Gewalt sowie Prüfung weiterer Angebote für kurzfristige Wohnmöglichkeiten in Akutsituationen",
		Geschaeftsart:     "Postulat",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "f179673bb7c7404b8330303715862f1f", Name: "SP", IstGremium: true}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "82f765845175438f83c542a0ec0ce386", Name: "FDP", IstGremium: true},
			{OBJGUID: "74261076640a4be0b42a59cbb82b58f7", Name: "GLP", IstGremium: true},
			{OBJGUID: "b415a00ff14141f7ae632fe01f06b4c4", Name: "Grüne", IstGremium: true},
			{OBJGUID: "8bc3b6c03c914b13bfea67e1e2855339", Name: "AL", IstGremium: true},
			{OBJGUID: "b511df3c164243e98466ea0c7e30d592", Name: "Die Mitte", IstGremium: true},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// PostulatLongestTitle is 2026/48. Postulat with the longest title among a year of Motionen and Postulate (295 characters).
func PostulatLongestTitle() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "a836a64de25a48c8ab784bee3fd4d178",
		GRNr:              "2026/48",
		Titel:             "Umbau der Unterführungen unter dem Bahnhofquai/Bahnhofplatz, Umsetzbarkeit einer getrennten Führung von Veloverkehr und motorisiertem Individualverkehr (MIV), Prüfung des sinnvollsten Standorts für eine Velostation sowie eines vorgezogenen Umbaus für die Realisierung der Vorhaben des Weissbuchs",
		Geschaeftsart:     "Postulat",
		Beginn:            zurichapi.Zeitraum{Start: "2026-01-28T00:00:00.000", Text: "28.01.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "f179673bb7c7404b8330303715862f1f", Name: "SP", IstGremium: true}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "b415a00ff14141f7ae632fe01f06b4c4", Name: "Grüne", IstGremium: true},
			{OBJGUID: "74261076640a4be0b42a59cbb82b58f7", Name: "GLP", IstGremium: true},
			{OBJGUID: "8bc3b6c03c914b13bfea67e1e2855339", Name: "AL", IstGremium: true},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-01-28T00:00:00.000", Text: "28.01.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// PostulatCounted is 2026/517. Postulat with thirty co-signers counted but not named.
func PostulatCounted() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "1b5b447bcbca459b8fe34d6251c8fff3",
		GRNr:              "2026/517",
		Titel:             "Verbesserung des Schutzes vor sexueller Belästigung und sexualisierten Übergriffen an Grossveranstaltungen in der Stadt",
		Geschaeftsart:     "Postulat",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-16T00:00:00.000", Text: "16.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "1e576dfb67584a638bae9da0a704da48", Name: "Valentina Fazlija", Partei: "FDP"}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "0d5da60d2e1e4ba1bd25b31381b092d2", Name: "Nina Eggenschwiler", Partei: "SP"},
			{OBJGUID: "967c0420fe744c048d5472f51a601167", Name: "Pascal Lamprecht", Partei: "SP"},
		},
		AnzahlMitunterzeichnende: intPtr(30),
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-16T00:00:00.000", Text: "16.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// PostulatDringlich is 2026/109. Postulat already marked Dringlich, filed by a Fraktion.
func PostulatDringlich() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "5c0e03b744554a06a45319afaa626f20",
		GRNr:              "2026/109",
		Titel:             "Strassenraum entlang der Tramtangente Nord, stadtverträgliche und platzsparende Planung und Realisierung",
		Geschaeftsart:     "Postulat",
		Dringlich:         true,
		Beginn:            zurichapi.Zeitraum{Start: "2026-03-11T00:00:00.000", Text: "11.03.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "b415a00ff14141f7ae632fe01f06b4c4", Name: "Grüne", IstGremium: true}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "f179673bb7c7404b8330303715862f1f", Name: "SP", IstGremium: true},
			{OBJGUID: "74261076640a4be0b42a59cbb82b58f7", Name: "GLP", IstGremium: true},
			{OBJGUID: "8bc3b6c03c914b13bfea67e1e2855339", Name: "AL", IstGremium: true},
		},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-03-11T00:00:00.000", Text: "11.03.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// SchriftlicheAnfrage is 2026/554. Schriftliche Anfrage: not announced, kept to prove the filter.
func SchriftlicheAnfrage() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:           "1fcf47b6b99b44e5a060d9edb98a663e",
		GRNr:              "2026/554",
		Titel:             "Künstliche Intelligenz im Schulalltag, Stand der Strategie zum Umgang mit KI-Systemen, Sicherstellung des Erlernens eines verantwortlichen Umgangs, stadtweit geltende pädagogische Leitplanken und Unterstützung der Lehrpersonen und Schulleitungen bei dieser Aufgabe, Konsequenzen für Hausaufgaben, Prüfungen und Leistungsnachweise sowie Gewährleistung eines chancengerechten und datenschutzkonformen Zugangs zu geeigneten Anwendungen",
		Geschaeftsart:     "Schriftliche Anfrage",
		Beginn:            zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
		Erstunterzeichner: []zurichapi.KontaktGremium{{OBJGUID: "aab17a31f79f405095586219d83e2fca", Name: "Patricia Petermann Loewe", Partei: "SP"}},
		Mitunterzeichner: []zurichapi.KontaktGremium{
			{OBJGUID: "0d5da60d2e1e4ba1bd25b31381b092d2", Name: "Nina Eggenschwiler", Partei: "SP"},
			{OBJGUID: "6af35f85c14a48ce89f69a834cd4f627", Name: "Florian Blättler", Partei: "SP"},
		},
		AnzahlMitunterzeichnende: intPtr(1),
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Eingang, Frist 3 Monate",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "2027-01-06T00:00:00.000", Text: "06.01.2027"},
		}},
	}
}

// Weisung is 2026/537. Weisung: not announced, kept to prove the filter.
func Weisung() zurichapi.Geschaeft {
	return zurichapi.Geschaeft{
		OBJGUID:       "617820d8a5d249328d6771c7d2a8c899",
		GRNr:          "2026/537",
		Titel:         "Finanzdepartement, Selbständige öffentlich-rechtliche Anstalten, Budgets 2027, Finanz- und Aufgabenpläne 2027–2030, Sammelvorlage",
		Geschaeftsart: "Weisung",
		Beginn:        zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
		Ablaufschritte: []zurichapi.Aufgabe{{
			AblaufschrittName: "Stadtrat, Weisung",
			Datum:             zurichapi.Zeitraum{Start: "2026-09-30T00:00:00.000", Text: "30.09.2026"},
			FristBis:          zurichapi.Zeitraum{Start: "", Text: ""},
		}},
	}
}

// Supported returns every fixture of a type geschaeftposting announces, keyed
// by name.
func Supported() map[string]zurichapi.Geschaeft {
	return map[string]zurichapi.Geschaeft{
		"MotionMember":         MotionMember(),
		"MotionCounted":        MotionCounted(),
		"MotionFraktion":       MotionFraktion(),
		"MotionDringlich":      MotionDringlich(),
		"PostulatFraktionen":   PostulatFraktionen(),
		"PostulatManySigners":  PostulatManySigners(),
		"PostulatLongestTitle": PostulatLongestTitle(),
		"PostulatCounted":      PostulatCounted(),
		"PostulatDringlich":    PostulatDringlich(),
	}
}

// Unsupported returns fixtures of types that are deliberately not announced.
func Unsupported() map[string]zurichapi.Geschaeft {
	return map[string]zurichapi.Geschaeft{
		"SchriftlicheAnfrage": SchriftlicheAnfrage(),
		"Weisung":             Weisung(),
	}
}
