#!/bin/sh
# Puts MisakiSwift 1.0.6, the version kokoro-ios 1.0.11 pins, with the patches in patches/
# into .misaki/MisakiSwift. Package.swift uses that copy in place of the remote package, for
# kokoro-ios too. The patch: MisakiSwift 1.0.6 reads the position of an inline form
# [word](/phonemes/) in UTF-16 units as a position in characters (Codex's review of PR #34).
# Run by test.sh and scripts/build-kokoro-helper.sh; it does nothing when the copy is current.
set -eu
cd "$(dirname "$0")"
url=https://github.com/mlalma/MisakiSwift
tag=1.0.6
commit=6835a1ce4a8854075c89f18ff75c74b13ef58e15
dest=.misaki/MisakiSwift
stamp="$commit $(cat patches/*.patch | shasum -a 256 | cut -d' ' -f1)"

if [ -f "$dest/.koinon-voice-stamp" ] && [ "$(cat "$dest/.koinon-voice-stamp")" = "$stamp" ]; then
	exit 0
fi
rm -rf .misaki.new
mkdir -p .misaki.new
git -c advice.detachedHead=false clone -q --depth 1 --branch "$tag" "$url" .misaki.new/MisakiSwift
got="$(git -C .misaki.new/MisakiSwift rev-parse HEAD)"
if [ "$got" != "$commit" ]; then
	echo "fetch-misaki: $url tag $tag is $got, want $commit" >&2
	exit 1
fi
for p in patches/*.patch; do
	git -C .misaki.new/MisakiSwift apply "$PWD/$p"
done
echo "$stamp" >.misaki.new/MisakiSwift/.koinon-voice-stamp
rm -rf .misaki
mv .misaki.new .misaki
