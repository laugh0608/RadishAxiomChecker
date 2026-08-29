#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(dirname -- "$script_dir")

cd "$repository_root"

expected_module=radishaxiom.dev/independent-checker-go
actual_modules=$(GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go list -m all)

if [ "$actual_modules" != "$expected_module" ]; then
  echo "Go module closure contains entries other than $expected_module:" >&2
  printf '%s\n' "$actual_modules" >&2
  exit 1
fi

if [ -e go.sum ] || [ -d vendor ]; then
  echo "Go module closure must not contain go.sum or vendor." >&2
  exit 1
fi

echo "Go module closure contains only $expected_module."
