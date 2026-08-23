#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(dirname -- "$script_dir")

cd "$repository_root"
RADISHAXIOM_UPDATE_SOURCE_IDENTITY=1 GOTOOLCHAIN=local CGO_ENABLED=0 go test -count=1 -run '^TestRepositoryManifestMatches$' ./internal/sourceidentity
