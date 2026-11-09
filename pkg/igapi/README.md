# Instagram API Package

This package implements a Go client for posting images to Instagram using the [Instagram Graph API](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/content-publishing) via Facebook Login.

## Packages

- **`igapi.Client`** — Instagram Graph API client for carousel publishing (create containers, publish, poll status, look up the permalink)
- **`igapi.ImageHoster`** — GitHub Pages image hosting via the GitHub Contents API (upload/cleanup JPEG files on `gh-pages` branch)

## Setup

### Prerequisites

- A **Facebook Page** for ZueriRatsinfo
- A **professional Instagram account** linked to the Facebook Page
- A **Meta Developer App** with the Instagram use case configured using **API setup with Facebook Login**
- Permissions: `instagram_basic`, `instagram_content_publish`, `pages_read_engagement`, `pages_show_list`, `business_management`, and `pages_manage_posts` for [Facebook](../fbapi/README.md), which posts with the same token
- A **GitHub repository** with a `gh-pages` branch and GitHub Pages enabled (see below)
- [`jq`](https://jqlang.github.io/jq/) locally, for the setup commands below (`brew install jq`)

### GitHub Pages Setup

Create an orphan `gh-pages` branch for image hosting:

```bash
git switch --orphan gh-pages
git commit --allow-empty -m "initialize gh-pages branch"
git push origin gh-pages
git switch -
```

Then enable GitHub Pages in the repo: **Settings → Pages → Source: Deploy from a branch → Branch: `gh-pages` / `/ (root)`**.

### Getting IDs

Every block below can be pasted unedited, in order. Values that come back from
the API are captured straight into shell variables; values that only exist on a
web page or in a log are prompted for, so nothing needs hand-substituting.
These variables live in the current shell only, and are separate from the
`ZURICH_*` variables the bot itself reads.

Both lookups need a user access token — generate one as in
[step 2](#2-generate-a-short-lived-user-token) below, then export it. `read -rs`
keeps it off the screen and out of your shell history:

```bash
printf 'User token: '; read -rs FB_USER_TOKEN; export FB_USER_TOKEN; echo
```

**Facebook Page ID** — call `GET /me/accounts`:

```bash
curl -s "https://graph.facebook.com/v25.0/me/accounts?access_token=$FB_USER_TOKEN" | jq
```

Find the ZueriRatsinfo entry in that list and give it its `id`:

```bash
printf 'Page id: '; read -r FB_PAGE_ID; export FB_PAGE_ID
```

**Instagram User ID** — call `GET /{page-id}?fields=instagram_business_account`:

```bash
export IG_USER_ID=$(curl -s "https://graph.facebook.com/v25.0/$FB_PAGE_ID?fields=instagram_business_account&access_token=$FB_USER_TOKEN" | jq -r .instagram_business_account.id)
echo "IG user id: $IG_USER_ID"
```

That is the value for `<CHANNEL>_IG_USER_ID`, and the [API Flow](#api-flow)
commands further down read it from the environment. An empty or `null` result
means the Page has no linked professional Instagram account — rerun the `curl`
without the `jq` pipe to see the full response.

### Access Token (never-expiring Page token)

The bot uses a **never-expiring Page access token**. Generating one is a chain
— app credentials, then a short-lived user token, then a long-lived user token,
then the Page token — and each step exports what the next one reads.

#### 1. Get App ID and App Secret

Both live in the app dashboard:

1. Open <https://developers.facebook.com/apps/> and click the **ZueriRatsinfo**
   app. The list shows each app's ID beneath its name, and the ID is also the
   number in the dashboard URL:
   `https://developers.facebook.com/apps/1234567890123456/dashboard/`.
2. In the left sidebar open **App settings → Basic**. Older dashboards label
   this **Settings → Basic**; in the newer "Use cases" layout the link sits
   near the bottom of the sidebar.
3. **App ID** is the first field on that page. **App Secret** is the next one,
   masked behind a **Show** button — clicking it asks for your Facebook
   password before revealing the value.

Export both:

```bash
printf 'App id: ';     read -r  FB_APP_ID;     export FB_APP_ID
printf 'App secret: '; read -rs FB_APP_SECRET; export FB_APP_SECRET; echo
```

If the App Secret is lost rather than merely hidden, the same page can reset it
— but a reset invalidates every other integration signed with the old secret,
so treat that as a last resort.

#### 2. Generate a short-lived User Token

- Open the [Graph API Explorer](https://developers.facebook.com/tools/explorer/)
- Select the ZueriRatsinfo app
- Add permissions: `instagram_basic`, `instagram_content_publish`, `pages_read_engagement`, `pages_show_list`, `business_management`, `pages_manage_posts`
  — the Explorer lists `pages_manage_posts` only once the app allows it; see
  [Facebook setup, 1a](../fbapi/README.md#1a-allow-the-permission-in-the-app)
- Click **Generate Access Token** and authorize

Copy the token it prints and export it:

```bash
printf 'Short-lived user token: '; read -rs FB_SHORT_TOKEN; export FB_SHORT_TOKEN; echo
```

#### 3. Exchange for a long-lived User Token (~60 days)

```bash
export FB_LONG_TOKEN=$(curl -s "https://graph.facebook.com/v25.0/oauth/access_token?grant_type=fb_exchange_token&client_id=$FB_APP_ID&client_secret=$FB_APP_SECRET&fb_exchange_token=$FB_SHORT_TOKEN" | jq -r .access_token)
```

Confirm it worked — an empty or `null` value means the response carried an
error instead of a token, so rerun the `curl` without the `jq` pipe to read it:

```bash
echo "${FB_LONG_TOKEN:0:12}…"
```

#### 4. Get the never-expiring Page Token

`GET /me/accounts` returns one entry per Page you administer. Pick the one
matching `FB_PAGE_ID` from [Getting IDs](#getting-ids):

```bash
export IG_TOKEN=$(curl -s "https://graph.facebook.com/v25.0/me/accounts?access_token=$FB_LONG_TOKEN" | jq -r --arg page "$FB_PAGE_ID" '.data[] | select(.id == $page) | .access_token')
```

To read the whole list instead — for instance if `FB_PAGE_ID` is not set yet:

```bash
curl -s "https://graph.facebook.com/v25.0/me/accounts?access_token=$FB_LONG_TOKEN" | jq
```

The `access_token` field in the response is a **permanent Page token** that won't expire as long as:

- You remain an admin of the Facebook Page
- App permissions are not revoked
- The Facebook account's password does not change — that invalidates the
  underlying session, and the Page token with it

#### 5. Update the GitHub Actions secret

First confirm the token actually works — step 4 yields an empty string if no
Page matched `FB_PAGE_ID`, and `null` if the response was an error rather than
a token. Storing either would replace a working secret with a broken one:

```bash
curl -s "https://graph.facebook.com/v25.0/$IG_USER_ID?fields=username&access_token=$IG_TOKEN" | jq
```

A working token returns the account's `username`. Anything else — an `error`
object, or a complaint about the token — means go back to step 2 rather than
storing what you have.

The secret is channel-scoped, so the `zurich` channel reads
`ZURICH_IG_ACCESS_TOKEN`. The guard refuses to write a token that step 4 failed
to produce:

```bash
if [ -n "$IG_TOKEN" ] && [ "$IG_TOKEN" != "null" ]; then
  gh secret set ZURICH_IG_ACCESS_TOKEN --body "$IG_TOKEN"
else
  echo "IG_TOKEN is empty or null — rerun step 4; existing secret left alone." >&2
fi
```

Or paste it by hand under **GitHub → Settings → Secrets and variables → Actions**.
For local runs, set the same value in `.env`.

**Ref**: https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived

### Environment Variables

Names are channel-scoped — the `zurich` channel reads `ZURICH_IG_USER_ID`.

| Variable                     | Description                                                    |
| ---------------------------- | -------------------------------------------------------------- |
| `<CHANNEL>_IG_USER_ID`       | Instagram professional account ID                              |
| `<CHANNEL>_IG_ACCESS_TOKEN`  | Long-lived Page access token                                   |
| `<CHANNEL>_IMAGE_HOST_REPO`  | `owner/name` of the repo serving the images (see below)        |
| `<CHANNEL>_IMAGE_HOST_TOKEN` | Token with Contents read/write on that repo                    |

### Image Hosting

The last two are a **GitHub** repo and a **GitHub** token, despite serving Instagram: the Graph API fetches carousel images by URL rather than accepting an upload, so each image is committed to the `gh-pages` branch of `IMAGE_HOST_REPO`, published by GitHub Pages, and deleted once the post is live.

`IMAGE_HOST_TOKEN` needs **Contents read/write** on that repo.

- **GitHub Actions**: the workflow passes `${{ github.repository }}` and `${{ github.token }}`. Nothing to configure — and nothing that *can* be configured, since `github.token` only has write access to the repo the workflow runs in.
- **Local testing**: create a **fine-grained Personal Access Token** at https://github.com/settings/tokens:
  1. Click **Generate new token** → **Fine-grained token**
  2. Set **Repository access** to **Only select repositories** → select the hosting repo
  3. Under **Permissions → Repository permissions**, set **Contents** to **Read and write**
  4. Generate and set as `<CHANNEL>_IMAGE_HOST_TOKEN`

## API Flow

These are the calls the Go client makes, written out for manual debugging. They
read `IG_USER_ID` and `IG_TOKEN` from the environment, so export those first:

```bash
printf 'IG user id: '; read -r  IG_USER_ID; export IG_USER_ID
printf 'Page token: '; read -rs IG_TOKEN;   export IG_TOKEN; echo
```

### Carousel Publishing

Publishing a carousel is a multi-step process:

#### Step 1 — Upload images to GitHub Pages

Images are committed to the `gh-pages` branch under `ig-images/` using the GitHub Contents API. After a short delay for GitHub Pages deployment, the images become publicly accessible.

#### Step 2 — Create carousel item containers

For each image, create a media container with `is_carousel_item=true`:

Capture each returned id — the next step combines them:

```bash
export IG_CHILD_1=$(curl -s -X POST "https://graph.facebook.com/v25.0/$IG_USER_ID/media" \
  -d "image_url=https://example.github.io/repo/ig-images/img1.jpg" \
  -d "is_carousel_item=true" \
  -d "access_token=$IG_TOKEN" | jq -r .id)

export IG_CHILD_2=$(curl -s -X POST "https://graph.facebook.com/v25.0/$IG_USER_ID/media" \
  -d "image_url=https://example.github.io/repo/ig-images/img2.jpg" \
  -d "is_carousel_item=true" \
  -d "access_token=$IG_TOKEN" | jq -r .id)

echo "children: $IG_CHILD_1 $IG_CHILD_2"
```

A `null` id means the container was rejected; drop the `| jq -r .id` to read
the error.

#### Step 3 — Create carousel container

Combine all child containers into a carousel with a caption:

```bash
export IG_CAROUSEL_ID=$(curl -s -X POST "https://graph.facebook.com/v25.0/$IG_USER_ID/media" \
  -d "media_type=CAROUSEL" \
  -d "children=[\"$IG_CHILD_1\",\"$IG_CHILD_2\"]" \
  -d "caption=Your caption here" \
  -d "access_token=$IG_TOKEN" | jq -r .id)

echo "carousel: $IG_CAROUSEL_ID"
```

#### Step 4 — Publish the carousel

```bash
export IG_MEDIA_ID=$(curl -s -X POST "https://graph.facebook.com/v25.0/$IG_USER_ID/media_publish" \
  -d "creation_id=$IG_CAROUSEL_ID" \
  -d "access_token=$IG_TOKEN" | jq -r .id)

echo "media: $IG_MEDIA_ID"
```

#### Step 5 — Poll for PUBLISHED status

```bash
curl -s "https://graph.facebook.com/v25.0/$IG_CAROUSEL_ID?fields=status_code&access_token=$IG_TOKEN" | jq
```

Possible values: `EXPIRED`, `ERROR`, `FINISHED`, `IN_PROGRESS`, `PUBLISHED`

#### Step 6 — Clean up hosted images

After successful publishing, the hosted images are removed from the `gh-pages` branch.

### Mentions and first comments

- Caption mentions are supported via `caption` using `@username` (users get mention notifications).
- An automated first comment is possible after publishing via:

```bash
curl -X POST "https://graph.facebook.com/v25.0/$IG_MEDIA_ID/comments" \
  -d "message=..." \
  -d "access_token=$IG_TOKEN"
```

## Rate Limits

- 100 API-published posts per 24-hour rolling period
- Check current usage:

```bash
curl -s "https://graph.facebook.com/v25.0/$IG_USER_ID/content_publishing_limit?access_token=$IG_TOKEN" | jq
```

## Troubleshooting

**Token invalidated (code 190, subcode 460)** — every Graph API call fails with:

```json
{"error":{"message":"Error validating access token: The session has been invalidated because the user changed their password or Facebook has changed the session for security reasons.","type":"OAuthException","code":190,"error_subcode":460}}
```

Nothing is wrong with the code or the images — the run typically gets as far as
uploading them to `gh-pages` before failing. The Page token derives from a
Facebook *user* session, so changing that account's password invalidates it:
"never-expiring" means no expiry timer, not immune to invalidation. Meta also
invalidates sessions on its own initiative.

The fix is to regenerate, starting from
[step 2](#2-generate-a-short-lived-user-token) — the App ID and Secret from
step 1 are unaffected — and to update `ZURICH_IG_ACCESS_TOKEN`.

Neighbouring subcodes under code 190 point elsewhere: `463` is an expired
token and `467` an invalid one, both of which suggest a *user* token was stored
where a Page token belongs, since a Page token carries no expiry. Inspect what
is actually stored before regenerating:

```bash
curl -s -G "https://graph.facebook.com/v25.0/debug_token" \
  --data-urlencode "input_token=$IG_TOKEN" \
  --data-urlencode "access_token=$FB_APP_ID|$FB_APP_SECRET" | jq
```

A healthy Page token reports `"type": "PAGE"` and `"expires_at": 0`.

**Container status** — if `media_publish` doesn't return a media ID, check the container status:

```bash
printf 'Container id: '; read -r IG_CONTAINER_ID; export IG_CONTAINER_ID
curl -s "https://graph.facebook.com/v25.0/$IG_CONTAINER_ID?fields=status_code&access_token=$IG_TOKEN" | jq
```

**Page Publishing Authorization (PPA)** — if posting fails with an authorization error, the linked Facebook Page may require PPA. Complete it at [facebook.com/business](https://www.facebook.com/business/m/one-sheeters/page-publishing-authorization).
