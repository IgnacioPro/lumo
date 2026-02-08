#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

failures=0

check_required_files() {
  local files=(
    "README.md"
    "LICENSE"
    "CONTRIBUTING.md"
    "CODE_OF_CONDUCT.md"
    "SECURITY.md"
    "SUPPORT.md"
    ".github/workflows/ci.yml"
  )

  for file in "${files[@]}"; do
    if [[ ! -f "$file" ]]; then
      echo "missing required file: $file"
      failures=$((failures + 1))
    fi
  done
}

check_tracked_artifacts() {
  local banned=()
  local path

  while IFS= read -r path; do
    [[ -e "$path" ]] || continue

    if [[ "$path" =~ ^\.claude/ ]] || [[ "$path" =~ ^\.gemini-clipboard/ ]] || [[ "$path" =~ ^crashing-pod-.*\.yaml$ ]] || [[ "$path" =~ ^website/(node_modules|\.next)/ ]]; then
      banned+=("$path")
    fi
  done < <(git ls-files)

  if [[ "${#banned[@]}" -gt 0 ]]; then
    echo "tracked publish-risk artifacts detected:"
    printf '%s\n' "${banned[@]}"
    failures=$((failures + 1))
  fi
}

run_secret_scan() {
  if command -v gitleaks >/dev/null 2>&1; then
    local cmd=(gitleaks git --redact)
    if [[ -f ".gitleaks.toml" ]]; then
      cmd+=(--config .gitleaks.toml)
    fi

    if ! "${cmd[@]}" >/dev/null; then
      echo "gitleaks reported potential leaks"
      failures=$((failures + 1))
    fi
  else
    echo "warning: gitleaks not installed; skipping secret scan"
  fi
}

check_required_files
check_tracked_artifacts
run_secret_scan

if [[ "$failures" -gt 0 ]]; then
  echo "publish audit failed with $failures issue(s)"
  exit 1
fi

echo "publish audit passed"
