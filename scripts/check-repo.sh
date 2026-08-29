#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(dirname -- "$script_dir")
base_ref=

usage() {
  echo "usage: ./scripts/check-repo.sh [--base-ref <git-ref>]" >&2
  exit 2
}

if [ "$#" -gt 0 ]; then
  [ "$#" -eq 2 ] || usage
  [ "$1" = "--base-ref" ] || usage
  [ -n "$2" ] || usage
  base_ref=$2
fi

cd "$repository_root"

required_files='AGENTS.md
CLAUDE.md
CONTRIBUTING.md
LICENSE
README.md
SECURITY.md
go.mod
.editorconfig
.gitattributes
.gitignore
.github/PULL_REQUEST_TEMPLATE.md
.github/workflows/checker-payload-candidate.yml
.github/workflows/pr-check.yml
cmd/radishaxiom-checker-distribution-accept/main.go
cmd/radishaxiom-checker-payload-archive/main.go
cmd/radishaxiom-checker-payload-distribution/main.go
docs/checker-payload-distribution-v0.1.md
docs/checker-payload-retention-v0.1.md
docs/repository-governance.md
internal/distributionaccept/accept.go
internal/distributionaccept/accept_test.go
internal/payloadarchive/archive.go
internal/payloadarchive/archive_test.go
internal/payloaddistribution/distribution.go
internal/payloaddistribution/distribution_test.go
scripts/check-module-closure.sh
scripts/check-repo.sh
scripts/check-source-identity.sh
scripts/update-source-identity.sh
source-identity/checker-source-v0.1.jcs'

printf '%s\n' "$required_files" | while IFS= read -r path; do
  if [ ! -f "$path" ]; then
    echo "required repository file is missing: $path" >&2
    exit 1
  fi
done

if ! cmp -s AGENTS.md CLAUDE.md; then
  echo "AGENTS.md and CLAUDE.md must be byte-identical." >&2
  exit 1
fi

git diff --check
git diff --cached --check

text_files=$(
  {
    git ls-files '*.go' '*.md' '*.mod' '*.sh' '*.yaml' '*.yml' '.editorconfig' '.gitattributes' '.gitignore'
    git ls-files --others --exclude-standard -- '*.go' '*.md' '*.mod' '*.sh' '*.yaml' '*.yml' '.editorconfig' '.gitattributes' '.gitignore'
  } | LC_ALL=C sort -u
)
printf '%s\n' "$text_files" | while IFS= read -r path; do
  [ -n "$path" ] || continue
  [ -f "$path" ] || continue

  first_octets=$(LC_ALL=C head -c 3 "$path" | od -An -tx1 | tr -d ' \n')
  if [ "$first_octets" = "efbbbf" ]; then
    echo "UTF-8 BOM is forbidden: $path" >&2
    exit 1
  fi

  if LC_ALL=C grep -n "$(printf '\r')" "$path" >/dev/null 2>&1; then
    echo "CR bytes are forbidden: $path" >&2
    exit 1
  fi

  if LC_ALL=C grep -n '[[:blank:]]$' "$path" >/dev/null 2>&1; then
    echo "trailing whitespace is forbidden: $path" >&2
    exit 1
  fi

  if [ -s "$path" ]; then
    last_octet=$(tail -c 1 "$path" | od -An -tu1 | tr -d ' \n')
    if [ "$last_octet" != "10" ]; then
      echo "text file must end with LF: $path" >&2
      exit 1
    fi
  fi
done

workflow=.github/workflows/pr-check.yml
candidate_workflow=.github/workflows/checker-payload-candidate.yml
checkout_pin='actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd'
setup_go_pin='actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e'
upload_artifact_pin='actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a'
download_artifact_pin='actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c'

grep -F "$checkout_pin" "$workflow" >/dev/null
grep -F "$setup_go_pin" "$workflow" >/dev/null
grep -F 'name: Candidate Quality' "$workflow" >/dev/null
grep -F 'contents: read' "$workflow" >/dev/null
grep -F '  pull_request:' "$workflow" >/dev/null

if LC_ALL=C grep -E '^  push:' "$workflow" >/dev/null 2>&1; then
  echo "checker PR workflow must not run on ordinary branch pushes." >&2
  exit 1
fi

grep -F 'workflow_dispatch:' "$candidate_workflow" >/dev/null
grep -F 'confirm_candidate_upload:' "$candidate_workflow" >/dev/null
grep -F 'actions: read' "$candidate_workflow" >/dev/null
grep -F 'contents: read' "$candidate_workflow" >/dev/null
grep -F 'runs-on: macos-15' "$candidate_workflow" >/dev/null
grep -F 'retention-days: 90' "$candidate_workflow" >/dev/null
grep -F 'archive: false' "$candidate_workflow" >/dev/null
grep -F 'artifact-ids: ${{ needs.build_upload.outputs.artifact_id }}' "$candidate_workflow" >/dev/null
grep -F 'go run ./cmd/radishaxiom-checker-distribution-accept' "$candidate_workflow" >/dev/null
grep -F 'go run ./cmd/radishaxiom-checker-payload-distribution' "$candidate_workflow" >/dev/null
grep -F '.distribution.tar' "$candidate_workflow" >/dev/null
grep -F "$checkout_pin" "$candidate_workflow" >/dev/null
grep -F "$setup_go_pin" "$candidate_workflow" >/dev/null
grep -F "$upload_artifact_pin" "$candidate_workflow" >/dev/null
grep -F "$download_artifact_pin" "$candidate_workflow" >/dev/null

if LC_ALL=C grep -E '^  (push|pull_request|schedule|workflow_run|repository_dispatch):' "$candidate_workflow" >/dev/null 2>&1; then
  echo "checker payload candidate workflow must remain manual-only." >&2
  exit 1
fi

if LC_ALL=C grep -E '^[[:space:]]*uses:' "$workflow" "$candidate_workflow" |
  LC_ALL=C grep -Ev 'uses: [^@#[:space:]]+@[0-9a-f]{40}([[:space:]]|$)' >/dev/null 2>&1; then
  echo "GitHub Actions must use exact 40-character lowercase commit pins." >&2
  exit 1
fi

if [ -n "$base_ref" ]; then
  git rev-parse --verify "$base_ref^{commit}" >/dev/null
  subjects=$(git log --format='%s' "$base_ref..HEAD")
  printf '%s\n' "$subjects" | while IFS= read -r subject; do
    [ -n "$subject" ] || continue
    if printf '%s\n' "$subject" | LC_ALL=C grep -E '^(feat|fix|docs|refactor|test|chore|ci|build|perf|revert)(\([a-z0-9][a-z0-9._/-]*\))?!?: .+$|^Merge .+$|^Revert ".+"$' >/dev/null; then
      continue
    fi
    echo "commit subject is not Conventional Commits: $subject" >&2
    exit 1
  done
fi

echo "Repository governance checks passed."
