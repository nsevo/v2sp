#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 vX.Y.Z [release-notes]"
  exit 1
fi

VERSION=$1
NOTES=${2:-"Release $VERSION"}

if ! [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must look like v1.2.3"
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree is dirty. Commit or stash changes first."
  exit 1
fi

git push origin HEAD:main
git tag "$VERSION"
git push origin "$VERSION"

if command -v gh >/dev/null; then
  gh release create "$VERSION" --notes "$NOTES"
else
  echo "Tag $VERSION pushed. Create the release manually or install GitHub CLI."
fi

