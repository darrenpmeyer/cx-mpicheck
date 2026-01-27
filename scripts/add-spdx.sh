#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

add_go_spdx() {
  local file="$1"
  if grep -q "SPDX-License-Identifier" "$file"; then
    return
  fi
  tmp="${file}.tmp"
  {
    echo "// SPDX-License-Identifier: AGPL-3.0-only"
    cat "$file"
  } > "$tmp"
  mv "$tmp" "$file"
}

add_sh_spdx() {
  local file="$1"
  if grep -q "SPDX-License-Identifier" "$file"; then
    return
  fi
  tmp="${file}.tmp"
  {
    head -n 1 "$file"
    echo "# SPDX-License-Identifier: AGPL-3.0-only"
    tail -n +2 "$file"
  } > "$tmp"
  mv "$tmp" "$file"
}

while IFS= read -r -d '' file; do
  add_go_spdx "$file"
done < <(find cmd mpicheck -name '*.go' -print0)

if [[ -f scripts/update-version.sh ]]; then
  add_sh_spdx scripts/update-version.sh
fi
