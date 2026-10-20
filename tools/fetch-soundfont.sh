#!/bin/sh
# Fetches the GeneralUser GS SoundFont (S. Christian Collins, v2.0.3, free to use and
# redistribute, see LICENSE-GeneralUser-GS.txt) into the project's root, where the game
# loads the first .sf2 file it finds in its working directory. It is ~32 MB, so it stays out
# of git. Any other General MIDI SoundFont dropped there works too.
set -e
dir=$(cd "$(dirname "$0")/.." && pwd)
rev=684543d5e5efaef08d02be50dcda8d552478fa60 # github.com/mrbumpy409/GeneralUser-GS, v2.0.3
sum=9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe
f="$dir/GeneralUser-GS.sf2"
if [ -f "$f" ] && [ "$(shasum -a 256 "$f" | cut -d' ' -f1)" = "$sum" ]; then exit 0; fi
curl -fL -o "$f.tmp" "https://raw.githubusercontent.com/mrbumpy409/GeneralUser-GS/$rev/GeneralUser-GS.sf2"
got=$(shasum -a 256 "$f.tmp" | cut -d' ' -f1)
if [ "$got" != "$sum" ]; then rm -f "$f.tmp"; echo "checksum mismatch: $got" >&2; exit 1; fi
mv "$f.tmp" "$f"
