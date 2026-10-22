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

[[ -z "$(git status --porcelain)" ]] || { echo "error: working tree not clean" >&2; exit 1; }
branch=$(git symbolic-ref --short -q HEAD) || { echo "error: detached HEAD" >&2; exit 1; }

# Latest stable semver tag (ignores pre-release tags like v1.2.3-rc1)
latest=$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)
latest=${latest:-v0.0.0}

IFS=. read -r major minor patch <<<"${latest#v}"
# NB: not ((x++)) — returns 1 when x==0 and trips `set -e`
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
new="v${major}.${minor}.${patch}"

git tag -a "$new" -m "Release $new"
echo "$latest -> $new (tagged $(git rev-parse --short HEAD))"

if $push; then
  git push --atomic origin "$branch" "$new"
else
  echo "not pushed. run: git push --atomic origin $branch $new"
fi
