#!/bin/sh
# Runs the Kokoro helper's tests, which need no model and no Metal library. With only the
# Command Line Tools, swift-testing is not on the default search path, so this adds it; with
# Xcode, plain `swift test` works.
set -eu
cd "$(dirname "$0")"
./fetch-misaki.sh
frameworks=/Library/Developer/CommandLineTools/Library/Developer/Frameworks
if [ "$(xcode-select -p)" = /Library/Developer/CommandLineTools ] && [ -d "$frameworks" ]; then
	exec swift test -Xswiftc "-F$frameworks" -Xlinker "-F$frameworks" \
		-Xlinker -rpath -Xlinker "$frameworks" \
		-Xlinker -rpath -Xlinker /Library/Developer/CommandLineTools/Library/Developer/usr/lib "$@"
fi
exec swift test "$@"
