# Facebook Pages API Package

This package publishes a post with photos to the ZueriRatsinfo Facebook Page
through the [Pages API](https://developers.facebook.com/docs/pages-api/posts/).
Each vote group gets the same card images as its Instagram carousel. For a
multi-vote group the post text is the heading, the title and the links, and each
vote's counts and Fraktion breakdown are the caption of its own card, followed
by that vote's own links (Kanton Zürich's Recapp segment, Stadt Zürich's vote
page); a single vote keeps all its text in the post. Publishing takes two calls:

1. `POST /{page-id}/photos` once per card, as a multipart upload with
   `published=false` and the card's `caption`, which returns a photo ID.
2. `POST /{page-id}/feed` with the `message` and one `attached_media[n]` per
   photo, in carousel order.

Unlike Instagram, nothing is hosted first: Facebook takes the JPEG bytes
directly, so there is no GitHub Pages step and no wait for it. Photos sit
unpublished for about a day before Meta deletes them, so a failed run leaves
nothing to clean up. A post with photos has no link preview card, so the links
stay in the text as plain URLs.

## Setup

Facebook rides on the Instagram setup: the same Meta Developer App, the same
Facebook Page and the same never-expiring Page access token. Do
[the Instagram setup](../igapi/README.md) first; this section covers only what
Facebook adds.

### 1. Add `pages_manage_posts` to the token

Publishing to a Page feed needs `pages_manage_posts`. The Instagram token does
not carry it, so the Page token has to be regenerated with it.

**No App Review.** Meta requires App Review only for apps that reach data their
owner does not own or manage. The bot posts to its own Page with a token that
the app's own admin issued, so Standard Access is enough. That is the same
footing the Instagram permissions are on today. Review would only become
necessary if someone without a role on the app had to authorise it.

#### 1a. Allow the permission in the app

The Graph API Explorer offers only the permissions that the app's use cases
include.

1. Open <https://developers.facebook.com/apps/>, then the **ZueriRatsinfo** app.
2. In the left sidebar, open **Use cases**, then click **Customize** on the
   Instagram use case. This opens **Customize use case**, with **Instagram API**
   selected in the dropdown at the top of its own sidebar.
3. That sidebar has four entries. The bot uses **API setup with Facebook
   login**: its section 1 lists exactly the permissions the Instagram token
   already has (`instagram_basic`, `instagram_content_publishing`,
   `pages_read_engagement`, `business_management`, `pages_show_list`). Ignore
   **API setup with Instagram login** and **API integration helper**. Their
   `instagram_business_*` permissions and `graph.instagram.com` calls belong to
   the other login flow, so don't click **Add all required permissions** there.
4. The Instagram use case's **Permissions and features** list does not offer
   `pages_manage_posts`: it covers only `instagram_*`, `pages_read_engagement`,
   `pages_show_list` and a few others. The permission belongs to the Page use
   case, which is a use case of its own. **Add more to this use case** in this
   sidebar offers only Webhooks, so leave it. Close the customize screen and
   click **Use cases** in the app's left sidebar. There, click
   **Add more use cases**, pick the **Content management** filter, tick
   **Manage everything on your Page** and click **Save**. Meta may refuse a
   combination of use cases, and says so in the dialog.
5. Switch the **Instagram API** dropdown at the top of the sidebar to
   **Manage everything on your Page**. Open its **Permissions and features**,
   find `pages_manage_posts` and click **Add**. Its status should read
   **Ready for testing**, like the Instagram permissions.

The Facebook-login page also has a **Complete app review** link, which offers
Advanced Access. Skip it, for the reason given above.

#### 1b. Regenerate the Page token

Follow [Access Token](../igapi/README.md#access-token-never-expiring-page-token)
in the Instagram setup, steps 1 to 5, with two differences:

- **Step 2:** the permission list there already includes `pages_manage_posts`.
  Make sure it is ticked in the Explorer before you click
  **Generate Access Token**. In the consent dialog, keep the ZueriRatsinfo Page
  selected and grant everything that is asked.
- **Step 5:** run the check below before storing the token.

The new token replaces `ZURICH_IG_ACCESS_TOKEN`. Instagram keeps working with
it, and the workflow hands the same secret to Facebook.

#### 1c. Check the token before storing it

Run this in the shell where you ran the Instagram steps. It needs `FB_APP_ID`
and `FB_APP_SECRET` (step 1), `FB_LONG_TOKEN` (step 3), `IG_TOKEN` (step 4) and
`FB_PAGE_ID` ([Getting IDs](../igapi/README.md#getting-ids)).

First check that the token carries the permission:

```bash
curl -s "https://graph.facebook.com/v25.0/debug_token?input_token=$IG_TOKEN&access_token=$FB_APP_ID|$FB_APP_SECRET" | jq '{type: .data.type, scopes: .data.scopes}'
```

The output should show `"type": "PAGE"`, and the scopes should include
`"pages_manage_posts"`. If the scope is missing, it was not ticked in step 2, or
1a was skipped.

Then check that your account may post on the Page:

```bash
curl -s "https://graph.facebook.com/v25.0/me/accounts?fields=id,name,tasks&access_token=$FB_LONG_TOKEN" | jq --arg page "$FB_PAGE_ID" '.data[] | select(.id == $page) | .tasks'
```

The list must contain `"CREATE_CONTENT"`. If it does not, your Page role is too
narrow: give yourself full control in the Page's settings under **Page access**.

If both checks pass, store the token with step 5 of the Instagram setup.

### 2. Set the go-live date

Facebook starts with no vote log, so every vote the sources still return reads
as unposted. A dry run on 24 October 2026 listed 40 groups back to 9 September.
The log is not seeded to hide that: a seeded log would record posts that were
never made. Instead Facebook posts only sittings from a start date, set in
`.github/workflows/bot-zhparl.yml`:

```yaml
ZURICH_FB_POST_FROM: "2026-10-17"
```

It is required whenever `ZURICH_FB_PAGE_ID` is set. The bot stops with an
error if it is missing or not a `YYYY-MM-DD` date, so deleting the line cannot
quietly reopen the backlog. Change it in the pull request that goes live, to the
first sitting the Page should show.

See what would post, without posting (the dry run reads `ZURICH_FB_POST_FROM`
from the environment):

```bash
ZURICH_FB_POST_FROM=2026-10-17 go run ./cmd/check_unposted -platform facebook -max-posts 100
```

The `📋` lines are the groups that would post, oldest first.

### 3. Switch it on

The Page ID is not a secret and is what turns Facebook on — the workflow passes
the Instagram token through under `ZURICH_FB_ACCESS_TOKEN`. With `FB_PAGE_ID` as
found under [Getting IDs](../igapi/README.md#getting-ids):

```bash
gh variable set ZURICH_FB_PAGE_ID --body "$FB_PAGE_ID"
```

Unset it to stop Facebook without touching the other platforms:

```bash
gh variable delete ZURICH_FB_PAGE_ID
```

### Environment Variables

Names are channel-scoped — the `zurich` channel reads `ZURICH_FB_PAGE_ID`.

| Variable                         | Description                                          |
| -------------------------------- | ---------------------------------------------------- |
| `<CHANNEL>_FB_PAGE_ID`           | Facebook Page ID                                     |
| `<CHANNEL>_FB_ACCESS_TOKEN`      | Page access token with `pages_manage_posts`          |
| `<CHANNEL>_FB_POST_FROM`         | First sitting date to post, `YYYY-MM-DD` (required)  |
| `<CHANNEL>_FB_MAX_POSTS_PER_RUN` | Posts per run (default 5)                            |

## Trying it locally

Preview what the bot would post, without credentials:

```bash
go run ./cmd/generate_vote_post -platform facebook -n 2
```

Publish one test fixture to the real Page:

```bash
ZURICH_FB_PAGE_ID=$FB_PAGE_ID ZURICH_FB_ACCESS_TOKEN=$IG_TOKEN go run ./cmd/post_fixture -platform facebook -fixture single-vote-angenommen
```

The output ends with the post URL. Delete the post from the Page afterwards.
