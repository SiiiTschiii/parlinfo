# Setup Guide

## Prerequisites

- Go 1.21 or later
- X API credentials (OAuth 1.0a User Context)

## 1. Get X API Credentials

Go to https://developer.x.com/en/portal/dashboard

1. **Create a Project and App** (if you haven't already)
   - Give it a name like "Zurich Ratsinfo Bot"
2. **Set App Permissions**
   - Go to your app settings
   - Under "User authentication settings", click "Set up"
   - Enable "OAuth 1.0a"
   - Set permissions to "Read and Write"
   - Add a callback URL (can be http://localhost:3000 for testing)
   - Add a website URL (can be your GitHub repo)
3. **Get Your Credentials**
   - Go to "Keys and tokens" tab
   - Copy these 4 values:
     - API Key (Consumer Key)
     - API Key Secret (Consumer Secret)
     - Access Token
     - Access Token Secret

## 2. Install

```bash
git clone https://github.com/SiiiTschiii/parlinfo.git
cd parlinfo
```

## 3. Configure the App

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and add your credentials. Platform variables are prefixed with the
# channel key — the set of accounts they belong to.
export ZURICH_X_API_KEY=your_api_key_here
export ZURICH_X_API_SECRET=your_api_secret_here
export ZURICH_X_ACCESS_TOKEN=your_access_token_here
export ZURICH_X_ACCESS_SECRET=your_access_secret_here
```

Nothing in the program reads `.env` — the bot reads the environment, and the
file is a shell script you source. Keep the `export` on every line, or `go run`
starts with none of these set:

```bash
source .env
```

**Important**: Never commit your `.env` file to git! It's already in `.gitignore`.

## 4. Preview Posts Locally

Nothing here posts anywhere or needs credentials — every command below prints to
your terminal or writes JPEGs to disk. This is the loop for reviewing how posts
read before they go anywhere near an account.

### Post text, from live data

`generate_vote_post` fetches the most recent votes and renders them exactly as
the bot would. It ignores the vote log, so already-posted votes still show up.

```bash
# Latest Gemeinderat vote group, all three platforms
go run ./cmd/generate_vote_post

# Latest Kantonsrat vote group
go run ./cmd/generate_vote_post -jurisdiction zurich-canton

# Latest Nationalrat Schluss-/Gesamtabstimmung
go run ./cmd/generate_vote_post -jurisdiction nationalrat -platform instagram

# One platform, three groups
go run ./cmd/generate_vote_post -platform x -n 3
```

Rendering both bodies side by side is how you check a reader could not mistake
one for the other — they post to the same accounts:

```bash
go run ./cmd/generate_vote_post -jurisdiction zurich-city   -platform x -n 1
go run ./cmd/generate_vote_post -jurisdiction zurich-canton -platform x -n 1
```

Flags: `-jurisdiction` (`zurich-city`, `zurich-canton`), `-platform`
(`x`, `bluesky`, `instagram`; default all), `-n` groups to show, `-fetch` how
many individual votes to pull from the API.

### Images, from fixtures

`generate_vote_image` writes the Instagram carousel JPEGs. It runs off fixtures
rather than the live API, so it is fast, offline and reproducible.

```bash
# Every fixture, including the Kantonsrat one
go run ./cmd/generate_vote_image -out out/images

# One fixture, with the Instagram caption printed alongside
go run ./cmd/generate_vote_image -fixture kantonsrat-vote -platform instagram -out out/images
```

Open `out/images/` to compare. Each card carries a full-width band naming the
chamber in that body's colour, over a background keyed on jurisdiction plus
business number.

Fixture names come from `pkg/voteposting/testfixtures` — `single-vote-angenommen`,
`multi-vote-group`, `auswahl-vote`, `kantonsrat-vote`, and others; an unknown
name prints the full list.

### What the real bot would do right now

`check_unposted` is the dry-run mirror of `main.go`: it reads the actual vote
logs, applies each jurisdiction's age guard, and prints what would be posted —
including how the shared per-run budget is spent across both bodies. It
previews every jurisdiction, enabled or not.

```bash
go run ./cmd/check_unposted
go run ./cmd/check_unposted -platform x -jurisdiction zurich-canton -max-posts 10
```

This needs the vote logs on disk. They live on the `state-log` branch, not in
`main`, and are gitignored here:

```bash
git fetch origin state-log
mkdir -p data/zurich-city
for p in x bluesky instagram; do
  git show "FETCH_HEAD:data/zurich-city/posted_votes_$p.json" > "data/zurich-city/posted_votes_$p.json"
done
```

The logs double as an audit trail: each entry records the post it went out in
as `post_id` and `post_url` — the root of the thread on X and Bluesky, and the
same post for every vote grouped into it. Entries written before these fields
existed have neither. To find where one vote was posted, on every platform:

```bash
jq -r --arg id "<vote id>" '.platform as $p | .votes[] | select(.id == $id) | "\($p)\t\(.post_url // .post_id // "-")"' data/zurich-city/posted_votes_*.json
```

### Curating who gets tagged

Tagging works by matching names in a post against `data/<jurisdiction>/contacts.yaml`.
The names come from the parliament; the handles do not, and never can — no source
publishes them for the Kantonsrat, so every one is found and verified by a human.

`update_contacts` refreshes the names from the body's own roster. A refresh only
ever adds; removing people who have left is the separate `-prune` mode, which
deletes the entries of people who sit in no chamber and refuses to remove more than
`-max-prune` (25) at once, since a source returning far fewer people than are on
file is more likely broken than right. Someone who now sits in the other chamber is
left in place and reported: move their accounts into that chamber's file by hand.

```bash
go run ./cmd/update_contacts -jurisdiction zurich-city -prune -dry-run
```

```bash
# See what would change first — it writes to a hand-curated file
go run ./cmd/update_contacts -jurisdiction zurich-canton -dry-run

go run ./cmd/update_contacts -jurisdiction zurich-canton
```

Both chambers are seeded whole, from the mandate register for Stadt Zürich and
from OpenParlData for the Kantonsrat, so a sitting member with no handle arrives
as a bare name. For Stadt Zürich it also imports the accounts PARIS publishes;
for the Kantonsrat there are none to import. Each run ends by listing the people on file who no longer sit. Party
and Fraktion accounts carry `kind: organization` so that list leaves them out.

A handle found by search goes in as an unverified candidate, never by editing the
file. It attaches to someone already on file, cannot be written as verified, and
leaves an account already there untouched; confirming it is a human's edit.

Members with no account at all are worked through in turns. `-backlog N` prints the
next N sitting members to search for, never-searched first and then the least
recently searched, and `-mark-searched` records that people were searched for, with
a `searched: YYYY-MM-DD` date in the file. Marking is what keeps a search that
found nothing from putting the same people at the head of the list every time.

```bash
go run ./cmd/update_contacts -jurisdiction zurich-canton -backlog 5
go run ./cmd/update_contacts -jurisdiction zurich-canton -mark-searched "Anna Müller" "Beat Keller"
```

```bash
go run ./cmd/update_contacts -jurisdiction zurich-canton -add-candidate \
  -name "Anna Müller" -platform instagram -confidence medium \
  -url https://www.instagram.com/anna.mueller/
```

`generate_search_urls` prints the search links for the manual half, with party,
Fraktion and the member's page on the parliament's own site pulled live — which
is what settles whether the "Anna Müller" you found is the one who sits in the
chamber.

```bash
go run ./cmd/generate_search_urls -jurisdiction zurich-canton -platform instagram
```

Validate before committing. CI runs the same check on every `data/*/contacts.yaml`:

```bash
go run ./cmd/validate_contacts data/zurich-canton/contacts.yaml
```

An entry with no handles is valid and costs nothing: the post names that member
without tagging them. Tagging the wrong account is the failure that matters —
it puts a real person's handle next to a vote they did not cast.

Every account says whether it may be published, and only `verified: true` is
ever posted — so candidates can sit in the file while you work through them.
There is no shape that means verified without saying so. To confirm one, open
it, check it is the person named above it, then flip the flag and drop the
`confidence` line:

```yaml
    instagram:
      - url: https://www.instagram.com/tmweidmann/
        verified: true
```

`generate_search_urls` lists the candidates already on file for each gap, so
checking one is usually quicker than searching again.

### Comparing the two data sources

Stadt Zürich is served by both PARIS and OpenParlData, which is what makes the
OpenParlData adapter checkable at all — the Kantonsrat has no second source to
verify against.

```bash
go run ./cmd/compare_sources -n 40
```

It exits non-zero if any totals, decisions or titles disagree.

## The CH Parl Info bot (Nationalrat)

The Nationalrat posts to accounts of its own (CH Parl Info, `@chparlinfo`) from a
workflow of its own, [bot-chparl.yml](.github/workflows/bot-chparl.yml). It is
the `chparl` channel: every setting is the zurich one with a `CHPARL_` prefix,
and the vote log lives on a branch of its own, `state-log-chparl`.

Each platform switches itself on when its credentials are set and stays off
while they are missing, so the platforms can go live one at a time. Nothing
posts at all until the last step.

### 1. Create the state branch

The workflow refuses to run without it, because a missing log reads as "nothing
was ever posted". Create it once, with an empty log per platform:

```bash
git switch --orphan state-log-chparl
mkdir -p data/nationalrat
for p in instagram facebook x bluesky; do
  printf '{\n  "jurisdiction": "nationalrat",\n  "platform": "%s",\n  "votes": []\n}\n' "$p" > "data/nationalrat/posted_votes_$p.json"
done
git add data/nationalrat
git commit -m "Start the Nationalrat vote logs"
git push origin state-log-chparl
git switch main
```

### 2. Instagram and Facebook (one Meta setup)

The Graph API reaches an Instagram account only through the Facebook Page it is
linked to, so `@chparlinfo` on Instagram needs a CH Parl Info Facebook Page
whatever you decide about posting there. The existing Meta app serves both
channels; what differs is the Page, and so the token.

1. Create the CH Parl Info Facebook Page, switch `@chparlinfo` to a
   professional account, and link it to that Page.
2. Generate a never-expiring Page token for the **new** Page, following
   [the Instagram setup](pkg/igapi/README.md#access-token-never-expiring-page-token)
   with `pages_manage_posts` ticked as in
   [the Facebook setup](pkg/fbapi/README.md). In step 4 pick the CH Parl Info
   entry of `/me/accounts`, not ZueriRatsinfo.
3. Read the IDs as in [Getting IDs](pkg/igapi/README.md#getting-ids), for the
   new Page.
4. Set them:

```bash
gh secret set CHPARL_IG_ACCESS_TOKEN
gh secret set CHPARL_IG_USER_ID
gh variable set CHPARL_FB_PAGE_ID --body "<page id>"
gh variable set CHPARL_FB_POST_FROM --body "<go-live date, YYYY-MM-DD>"
```

Facebook uses the same token secret, so a rotation covers both. Leave
`CHPARL_FB_PAGE_ID` unset to post to Instagram only; set it and
`CHPARL_FB_POST_FROM` is required too, or the run stops.

### 3. X

Create the X account (`@chparlinfo` exists), then sign in to <https://developer.x.com> **as that
account** and create a project and app there, as in
[section 1](#1-get-x-api-credentials). Tokens generated under **Keys and tokens**
act for the account that owns the app, which is the point: reusing the Zürich
app would need a separate OAuth sign-in, and would share Zürich's free-tier
quota.

```bash
gh secret set CHPARL_X_API_KEY
gh secret set CHPARL_X_API_SECRET
gh secret set CHPARL_X_ACCESS_TOKEN
gh secret set CHPARL_X_ACCESS_SECRET
```

`CHPARL_X_MAX_CHARS` defaults to 280; set it to 2000 only if the account has
X Premium.

### 4. Bluesky

Create the account, then **Settings → Privacy and security → App passwords**,
and add one for the bot. Never use the account password.

```bash
gh variable set CHPARL_BLUESKY_HANDLE --body "chparlinfo.bsky.social"
gh secret set CHPARL_BLUESKY_PASSWORD
```

### Adding a platform once the Nationalrat is live

A platform added after `JURISDICTION_NATIONALRAT_ENABLED` is on starts from an
empty log, so its first run posts every final vote still inside
`MAX_VOTE_AGE_DAYS_NATIONALRAT`, up to its per-run cap, which defaults to 10.
Set the cap **before** the last credential, since that one switches the
platform on:

```bash
gh variable set CHPARL_X_MAX_POSTS_PER_RUN --body 1
gh variable set CHPARL_BLUESKY_MAX_POSTS_PER_RUN --body 1
```

Then see what that backlog is, per platform and against the live logs, without
posting. It runs in a throwaway worktree, because `check_unposted` reads the logs
from `data/` and would otherwise overwrite the ones a local bot run relies on:

```bash
tmp="$(mktemp -d)"
git fetch origin main state-log-chparl
git worktree add --detach "$tmp" origin/main
git -C "$tmp" checkout origin/state-log-chparl -- 'data/nationalrat/posted_votes_*.json'
(cd "$tmp" && CHANNEL=chparl MAX_VOTES_TO_CHECK=35 MAX_VOTE_AGE_DAYS_NATIONALRAT="$(gh variable get MAX_VOTE_AGE_DAYS_NATIONALRAT)" \
  go run ./cmd/check_unposted -jurisdiction nationalrat)
git worktree remove --force "$tmp"
```

### 5. Go live

Check what would be posted, then turn the switch on and run once by hand:

```bash
go run ./cmd/generate_vote_post -jurisdiction nationalrat -n 3
gh variable set JURISDICTION_NATIONALRAT_ENABLED --body true
gh workflow run bot-chparl.yml
```

The first run posts the final votes of the last 14 days
(`MAX_VOTE_AGE_DAYS_NATIONALRAT`), so outside a session it posts nothing and
the first posts follow the next session's final votes. Until the switch is set,
every scheduled run fails with "No jurisdiction is enabled". That is the switch
working as designed; `gh workflow disable bot-chparl.yml` silences it until
then. To stop posting again (the runs then fail the same way):

```bash
gh variable set JURISDICTION_NATIONALRAT_ENABLED --body false
```

## Development

### Testing

```bash
go test ./...
```

### E2E Testing with Test Accounts

E2E tests post real content to test accounts on X and Bluesky. They verify the full `Format → Post → API call` chain.

**Setup (one-time):**

1. Create test accounts on X and Bluesky (use obscure names, set to private/protected)
2. Fill in `.env.test` with the test account credentials, using the same
   channel-prefixed names as `.env.example` (`ZURICH_X_API_KEY`, …). Every
   command below resolves credentials through the channel, so unprefixed names
   are not read anywhere.
3. Edit `data/contacts_test.yaml` — replace placeholder handles with your test account handles

**Post fixtures to test accounts:**

```bash
source .env.test

# Post a single fixture to one platform
go run cmd/post_fixture/main.go --fixture=single-vote-angenommen --platform=x
go run cmd/post_fixture/main.go --fixture=multi-vote-group --platform=bluesky

# Both chambers side by side — the check that a reader can tell them apart
go run cmd/post_fixture/main.go --fixture=kantonsrat-vote --platform=bluesky

# Post all fixtures to all platforms
go run cmd/post_fixture/main.go --fixture=all

# Live votes of any parliament through the same test accounts — the newest
# Nationalrat business, or the newest one with several votes
go run cmd/post_fixture/main.go --jurisdiction=nationalrat --platform=bluesky
go run cmd/post_fixture/main.go --jurisdiction=nationalrat --min-votes=2 --fetch=300

# Use real contacts (tags real accounts — use with care)
go run cmd/post_fixture/main.go --contacts=data/zurich-city/contacts.yaml --fixture=vote-with-mentions

# Cleanup all test posts (will delete all posts made by the test accounts)
go run cmd/cleanup_posts/main.go
go run cmd/cleanup_posts/main.go --platform=x
go run cmd/cleanup_posts/main.go --platform=bluesky
```

**Test with live votes (fetches recent votes from the Zurich API):**

```bash
source .env.test
JURISDICTION_ZURICH_CITY_ENABLED=true JURISDICTION_ZURICH_CANTON_ENABLED=true SKIP_VOTE_LOG=true MAX_VOTES_TO_CHECK=5 go run main.go
```

No jurisdiction posts unless its `JURISDICTION_<KEY>_ENABLED` variable is `true`, so the switch has to be set here too.

**Regression workflow** (after formatting or posting changes):

1. `go test ./...` — automated unit tests, including the golden snapshot below
2. `source .env.test && go run cmd/post_fixture/main.go --fixture=all` — manual fixture verification
3. `source .env.test && JURISDICTION_ZURICH_CITY_ENABLED=true JURISDICTION_ZURICH_CANTON_ENABLED=true SKIP_VOTE_LOG=true MAX_VOTES_TO_CHECK=5 go run main.go` — manual live vote verification

### Golden Snapshot

`pkg/voteposting/golden` renders every fixture through all three platforms and
compares the result against `testdata/golden.txt`, verbatim. Generated images
are checked separately by their properties, because JPEG bytes differ between
CPU architectures and a byte hash would pass locally and fail in CI.

A refactor that is meant to preserve behaviour must leave this file untouched;
that is what makes the claim checkable rather than asserted. When a change is
*meant* to alter output, regenerate and read the diff before committing it:

```bash
go test ./pkg/voteposting/golden -update
git diff pkg/voteposting/golden/testdata/golden.txt
```

### Linting

Install and run [golangci-lint](https://golangci-lint.run/):

```bash
brew install golangci-lint  # macOS
golangci-lint run           # check all files
golangci-lint run --new     # only check unstaged changes
```

**VS Code**: Install the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go) - settings are already configured in `.vscode/settings.json`.

### CI Locally

Run workflows locally with [act](https://github.com/nektos/act):

```bash
brew install act
act -W .github/workflows/go-ci.yml
```
