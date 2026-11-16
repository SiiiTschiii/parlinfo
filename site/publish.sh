#!/usr/bin/env bash
# Publishes a built site to the gh-pages branch, which GitHub Pages serves.
#
#   site/publish.sh <built-site-dir> [remote] [branch]
#
# The branch is shared. The bots commit carousel images for Instagram under
# ig-images/ through the Contents API (pkg/igapi/hosting.go), several of them
# side by side and at any moment, and delete them again once a post is live.
# So this script:
#
#   - replaces every path on the branch except ig-images/, which it neither
#     adds to, changes nor removes: the commit carries that directory exactly
#     as the commit below it has it;
#   - never forces. A bot commit that lands between the fetch and the push
#     makes the push fail as a non-fast-forward, and the whole step is redone
#     on the new head, so the bot's commit is kept and the site goes on top;
#   - refuses a build that contains ig-images/.
#
# The other direction needs nothing new: a site commit landing in the middle
# of a bot's write gives the bot a 409, which it already retries.
set -euo pipefail

site=${1:?usage: publish.sh <built-site-dir> [remote] [branch]}
remote=${2:-origin}
branch=${3:-gh-pages}
keep=ig-images
attempts=6

site=$(cd "$site" && pwd)
if [ ! -f "$site/index.html" ]; then
  echo "::error::$site has no index.html; refusing to publish an empty site." >&2
  exit 1
fi
if [ -e "$site/$keep" ]; then
  echo "::error::The built site contains $keep/, which belongs to the bots." >&2
  exit 1
fi

work=$(mktemp -d)
trap 'git worktree remove --force "$work" 2>/dev/null || rm -rf "$work"' EXIT
git fetch --quiet --depth=1 "$remote" "$branch"
git worktree add --quiet --detach "$work" FETCH_HEAD
cd "$work"

for attempt in $(seq 1 "$attempts"); do
  git fetch --quiet --depth=1 "$remote" "$branch"
  git reset --quiet --hard FETCH_HEAD
  git clean --quiet -fdx

  # Out with the previous site, in with the new one. Removing first is what
  # makes a page deleted on main disappear from the branch.
  git rm --quiet -r --ignore-unmatch -- . ":(exclude)$keep"
  cp -R "$site/." .
  git add --all -- . ":(exclude)$keep"

  if git diff --cached --quiet; then
    echo "Site on $branch is already up to date."
    exit 0
  fi
  # Belt and braces: the pathspecs above already exclude it.
  if git diff --cached --name-only | grep -q "^$keep/"; then
    echo "::error::The commit would touch $keep/; aborting." >&2
    exit 1
  fi

  git commit --quiet -m "publish site from ${GITHUB_SHA:-a local build}"
  if git push --quiet "$remote" "HEAD:refs/heads/$branch"; then
    echo "Site published to $branch."
    exit 0
  fi
  echo "Push rejected (attempt $attempt of $attempts): $branch moved, most likely a bot's image commit. Retrying on the new head."
  sleep $((attempt * 2))
done

echo "::error::Could not publish: $branch kept moving for $attempts attempts." >&2
exit 1
