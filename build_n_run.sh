#!/bin/sh
# Builds dagger and runs it from the project root (where the SoundFont lies).
# DAGGER_DATA names the Daggerfall install (holds ARENA2 and FALL.EXE).
set -e
cd "$(dirname "$0")"
go build -o dagger ./cmd/dagger
exec ./dagger -data "${DAGGER_DATA:-$HOME/Projects/dagger-re/gog}" "$@"
