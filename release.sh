#!/usr/bin/env bash
# release.sh <major|minor|patch> [--push]
set -euo pipefail

usage() { echo "usage: $0 <major|minor|patch> [--push]" >&2; exit 1; }

bump="" push=false
for arg in "$@"; do
  case "$arg" in
    major|minor|patch) bump="$arg" ;;
    --push) push=true ;;
    *) usage ;;
  esac
done
[[ -n "$bump" ]] || usage

cd "$(git rev-parse --show-toplevel)"

[[ -z "$(git status --porcelain)" ]] || { echo "error: working tree not clean" >&2; exit 1; }
branch=$(git symbolic-ref --short -q HEAD) || { echo "error: detached HEAD" >&2; exit 1; }

# Source of truth: VERSION file; bootstrap from latest stable tag, else 0.0.0
if [[ -f VERSION ]]; then
  current=$(tr -d '[:space:]' < VERSION)
else
  current=$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)
  current=${current#v}
  current=${current:-0.0.0}
fi
[[ "$current" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "error: invalid version '$current' in VERSION" >&2; exit 1; }

IFS=. read -r major minor patch <<<"$current"
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
new="${major}.${minor}.${patch}"
tag="v${new}"

git rev-parse -q --verify "refs/tags/$tag" >/dev/null && { echo "error: tag $tag already exists" >&2; exit 1; }

printf '%s\n' "$new" > VERSION
git add VERSION
git commit -m "release: $tag"
git tag -a "$tag" -m "Release $tag"
echo "$current -> $new (commit $(git rev-parse --short HEAD), tag $tag)"

if $push; then
  git push --atomic origin "$branch" "$tag"
else
  echo "not pushed. run: git push --atomic origin $branch $tag"
fi
