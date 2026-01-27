#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

create_tag=false
version_arg=""
while getopts ":t:" opt; do
  case "$opt" in
    t)
      create_tag=true
      version_arg="$OPTARG"
      ;;
    *)
      echo "Usage: $0 [-t version] [version]" >&2
      exit 1
      ;;
  esac
done
shift $((OPTIND - 1))

if [[ -n "${version_arg}" && $# -gt 0 ]]; then
  echo "Provide version either as argument or with -t, not both" >&2
  exit 1
fi

if [[ -n "${version_arg}" ]]; then
  version="$version_arg"
  version="${version#v}"
elif [[ $# -gt 0 ]]; then
  version="$1"
  version="${version#v}"
else
  latest_tag="$(git tag --list 'v*' --sort=-v:refname | head -n 1)"
  if [[ -z "$latest_tag" ]]; then
    echo "No tags found matching v*" >&2
    exit 1
  fi

  base_version="${latest_tag#v}"

  if git merge-base --is-ancestor "${latest_tag}" HEAD; then
    if git merge-base --is-ancestor HEAD "${latest_tag}"; then
      version="$base_version"
    else
      short_sha="$(git rev-parse --short HEAD)"
      version="${base_version}-${short_sha}"
    fi
  else
    echo "Current HEAD is behind latest tag ${latest_tag}" >&2
    exit 1
  fi
fi

if [[ -z "$version" ]]; then
  echo "Failed to compute version" >&2
  exit 1
fi

update_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    return
  fi
  tmp="${file}.tmp"
  sed -E "s/(version|Version) = \"[^\"]+\"/\1 = \"${version}\"/" "$file" > "$tmp"
  mv "$tmp" "$file"
}

update_file "cmd/cx-mpicheck/version.go"
update_file "mpicheck/version.go"

echo "Updated version to ${version}"

if [[ "${create_tag}" == "true" ]]; then
  git add cmd/cx-mpicheck/version.go mpicheck/version.go
  if ! git diff --cached --quiet; then
    git commit -m "Bump version to ${version}"
  fi
  tag="v${version}"
  if git rev-parse --verify --quiet "refs/tags/${tag}" >/dev/null; then
    echo "Tag ${tag} already exists" >&2
    exit 1
  fi
  git tag "${tag}"
  echo "Created tag ${tag}"
fi
