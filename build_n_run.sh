#!/bin/sh
# Builds dagger and runs it from the project root (where the SoundFont lies).
# DAGGER_DATA names the Daggerfall install (holds ARENA2 and FALL.EXE).
# Music uses SC55_zzdenis_v0.5.sf2 unless DAGGER_SF2 names another SoundFont.
set -e
cd "$(dirname "$0")"
export DAGGER_SF2="${DAGGER_SF2:-SC55_zzdenis_v0.5.sf2}"
go build -o dagger ./cmd/dagger
exec ./dagger -data "${DAGGER_DATA:-$HOME/Projects/dagger-re/gog}" "$@"
