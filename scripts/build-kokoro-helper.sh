#!/bin/sh
# Builds the Kokoro helper with xcodebuild, which MLX Swift needs for its Metal shaders, and
# installs it with its frameworks and resource bundles in one directory:
#   ~/.local/share/koinon-voice/bin/kokoro/koinon-voice-kokoro
# Run the helper by that path: MLX and Misaki find their bundles beside the real binary, not
# beside a symlink to it. Usage: scripts/build-kokoro-helper.sh [INSTALL_DIR]
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
dest="${1:-$HOME/.local/share/koinon-voice/bin/kokoro}"
derived="$root/helpers/kokoro/.build/xcode"

xcrun -f metal >/dev/null 2>&1 || {
	echo "build-kokoro-helper: no Metal compiler; install Xcode, then run: xcodebuild -downloadComponent MetalToolchain" >&2
	exit 1
}

mkdir -p "$derived" "$(dirname "$dest")"
"$root/helpers/kokoro/fetch-misaki.sh"
cd "$root/helpers/kokoro"
# Without the coverage settings, Xcode instruments the package and the helper writes
# default.profraw in its working directory.
xcodebuild -scheme koinon-voice-kokoro -configuration Release -destination 'platform=macOS' \
	-derivedDataPath "$derived" -skipMacroValidation \
	CLANG_ENABLE_CODE_COVERAGE=NO CLANG_COVERAGE_MAPPING=NO build >"$derived.log" 2>&1 || {
	tail -30 "$derived.log" >&2
	echo "build-kokoro-helper: xcodebuild failed; the whole log is $derived.log" >&2
	exit 1
}

products="$derived/Build/Products/Release"
stage="$dest.new"
rm -rf "$stage"
mkdir -p "$stage/Frameworks"
cp "$products/koinon-voice-kokoro" "$stage/"
cp -R "$products"/*.bundle "$stage/"
cp -R "$products"/PackageFrameworks/*.framework "$stage/Frameworks/"
# The text cases run the real G2P; a failure keeps the helper installed before.
"$stage/koinon-voice-kokoro" check-text >"$derived-check-text.log" 2>&1 || {
	cat "$derived-check-text.log" >&2
	echo "build-kokoro-helper: check-text failed; $dest is unchanged" >&2
	exit 1
}
rm -rf "$dest"
mv "$stage" "$dest"
chmod 0700 "$dest"
echo "$dest/koinon-voice-kokoro"
