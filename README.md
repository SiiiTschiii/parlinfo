# parlinfo

<p align="center">
  <img src="assets/brand/parlinfo-readme-header.png" alt="parlinfo" width="100%">
</p>

<p align="center">
  <a href="https://github.com/SiiiTschiii/parlinfo/actions/workflows/go-ci.yml">
    <img src="https://github.com/SiiiTschiii/parlinfo/actions/workflows/go-ci.yml/badge.svg" alt="Go CI">
  </a>
  <a href="https://github.com/SiiiTschiii/parlinfo/actions/workflows/validate-contacts.yml">
    <img src="https://github.com/SiiiTschiii/parlinfo/actions/workflows/validate-contacts.yml/badge.svg" alt="Validate Contacts">
  </a>
  <a href="https://github.com/SiiiTschiii/parlinfo/actions/workflows/bot.yml">
    <img src="https://img.shields.io/github/last-commit/SiiiTschiii/parlinfo/main?label=last%20bot%20run&logo=github" alt="Last Bot Run">
  </a>
</p>

A civic tech bot that brings parliamentary decisions to social media and tags the politicians involved, using a curated mapping of their accounts. It is built to grow: more parliaments, more platforms.

We built it because a healthy democracy rests on two things that are easy to lose sight of:

- **Participation**: citizens who can actually follow and take part in the decisions that affect them.
- **Accountability**: what politicians actually do, from how they vote to which motions, postulates and petitions they put their name on, visible right where people already are.

**🇨🇭 Auf Deutsch**

Ein Civic-Tech-Bot, der parlamentarische Entscheide auf Social Media bringt und die beteiligten Politikerinnen und Politiker markiert, anhand einer manuell gepflegten Zuordnung ihrer Accounts. Das Projekt ist auf Wachstum angelegt: mehr Parlamente, mehr Plattformen.

Wir haben das Projekt ins Leben gerufen, weil eine gesunde Demokratie auf zwei Dingen beruht, die leicht aus dem Blick geraten:

- **Partizipation**: Bürgerinnen und Bürger, die die Entscheide, die sie betreffen, tatsächlich mitverfolgen und mitgestalten können.
- **Rechenschaft**: was Politikerinnen und Politiker wirklich tun, vom Abstimmungsverhalten bis zu den Motionen, Postulaten und Petitionen, die sie mittragen, sichtbar dort, wo die Menschen ohnehin schon sind.

## Covered Bodies

| Body                         | Data source                                                              | Status  |
| ---------------------------- | ------------------------------------------------------------------------ | ------- |
| **Gemeinderat Stadt Zürich** | [PARIS API](https://opendatazurich.github.io/paris-api/), City of Zurich | ✅ Live |
| **Kantonsrat Zürich**        | [OpenParlData](https://api.openparldata.ch/documentation)                | ✅ Live |
| **Nationalrat**              | [parlament.ch OData](https://ws.parlament.ch/odata.svc/), Parlamentsdienste | ✅ Live |

## Supported Platforms

| Platform    | Status     | Zürich Ratsinfo (Gemeinderat, Kantonsrat)                                            | CH Parl Info (Nationalrat)                                                           |
| ----------- | ---------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| Instagram   | ✅ Active  | [@zueriratsinfo](https://www.instagram.com/zueriratsinfo)                            | [@chparlinfo](https://www.instagram.com/chparlinfo)                                  |
| Facebook    | ✅ Active  | [ZueriRatsinfo](https://www.facebook.com/profile.php?id=1063164230212573)            | [CH Parl Info](https://www.facebook.com/profile.php?id=61595771216866)               |
| X (Twitter) | ✅ Active  | [@zuerichratsinfo](https://x.com/zuerichratsinfo)                                    | [@chparlinfo](https://x.com/chparlinfo)                                              |
| Bluesky     | ✅ Active  | [@zuerichratsinfo.bsky.social](https://bsky.app/profile/zuerichratsinfo.bsky.social) | [@chparlinfo.bsky.social](https://bsky.app/profile/chparlinfo.bsky.social)           |
| LinkedIn    | ❌ Planned | -                                                                                    | -                                                                                    |
| TikTok      | ❌ Planned | -                                                                                    | -                                                                                    |

_Politicians the bot can tag per platform, from the curated mapping in [data/](data/): LinkedIn 120, Instagram 103, Facebook 96, X 69, Bluesky 27, TikTok 19. Tagging is not available for every body yet._

## What It Does

- **Automated Vote Posts**: Shares vote results (Abstimmungen) with the full per-faction breakdown, as image carousels. Each post names the body that voted, in the text and on the image. Posts go out once the source publishes the data, see the data sources above.
- **Politician Tagging**: Automatically tags mentioned politicians using their social media accounts when available in our mapping
  - _Example: "Postulat von Ivo Bieri @ivo_bieri (SP) und Liv Mahrer @LivMahrer (SP)..."_
- **Social Media Mapping**: Curates a mapping of politicians to their social media accounts (X, Facebook, Instagram, LinkedIn, Bluesky, TikTok), see [data/](data/)

## Tech Stack

- Go
- Zurich Council PARIS API, see [pkg/zurichapi/README.md](pkg/zurichapi/README.md)
- OpenParlData API for the Kantonsrat, see [pkg/openparldata](pkg/openparldata)
- parlament.ch OData for the Nationalrat, see [pkg/parlamentch](pkg/parlamentch)
- X API v2 with OAuth 1.0a, see [pkg/xapi/README.md](pkg/xapi/README.md)
- Bluesky AT Protocol (app.bsky), see [pkg/voteposting/platforms/bluesky](pkg/voteposting/platforms/bluesky)
- Instagram Graph API with image carousel publishing, see [pkg/igapi/README.md](pkg/igapi/README.md)
- Facebook Pages API with photo posts, see [pkg/fbapi/README.md](pkg/fbapi/README.md)
- Vote image generation (1080×1350 JPEG carousels), see [pkg/imagegen](pkg/imagegen)

## Setup

See [SETUP.md](SETUP.md) for installation and configuration instructions.

## Project Progress

See the [open issues](https://github.com/SiiiTschiii/parlinfo/issues) for current tasks and roadmap.

## Recognition

zuerichratsinfo is featured on the [City of Zurich's Open Government Data portal](https://www.stadt-zuerich.ch/de/politik-und-verwaltung/statistik-und-daten/open-government-data/anwendungen/anwendungen-2026/zuerichratsinfo.html) as an exemplary application using public government data to improve democratic transparency.

## Support This Project

Help keep @zuerichratsinfo running! Your support covers the costs for X Premium account and API access.

<a href="https://buymeacoffee.com/zuerichratsinfo" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

This is a non-profit civic tech project. Every contribution helps make local politics more accessible! 🙏

## Acknowledgments

Special thanks to:

- **[Alexander Guentert](https://github.com/alexanderguentert)** from [Open Data Zurich](https://opendatazurich.github.io) for support in integrating the Paris-API, Gemeinderat Stadt Zürich
- **[OpenParlData](https://opendata.ch/projects/openparldata/)** for harmonised Swiss parliamentary data (CC BY 4.0), which is what makes the Kantonsrat, and any further canton, possible ([API docs](https://api.openparldata.ch))

## License

MIT
