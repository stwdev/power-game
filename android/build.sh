#!/bin/sh
# Builds a signed APK from dist/ using only the Android SDK command-line
# build tools (aapt2, javac, d8, zipalign, apksigner): no Gradle, no
# plugins and nothing downloaded during the build.
#
#   android/build.sh <version> <signing-key.b64> <signing-cert.pem> <out.apk>
#
# Needs ANDROID_HOME (preinstalled on GitHub's Ubuntu runners), a JDK and
# a built dist/ (./build.sh).
set -eu
cd "$(dirname "$0")/.."

VERSION=$1 KEY_B64=$2 CERT=$3 OUT=$4
case "$VERSION" in
  *[!0-9.]* | "" | .* | *. | *..*) echo "version must look like 1.2.3" >&2; exit 1 ;;
esac
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION.0" | cut -d. -f2)
PATCH=$(echo "$VERSION.0.0" | cut -d. -f3)
CODE=$((MAJOR * 10000 + MINOR * 100 + PATCH))

: "${ANDROID_HOME:?set ANDROID_HOME to the Android SDK}"
# Newest stable versions only: skip betas, release candidates and previews.
BT=$(ls -d "$ANDROID_HOME"/build-tools/* | grep -E '/[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1)
PLATFORM=$(ls -d "$ANDROID_HOME"/platforms/android-* | grep -E '/android-[0-9]+$' | sort -V | tail -n 1)
[ -n "$BT" ] && [ -n "$PLATFORM" ] || { echo "no stable Android build-tools/platform found" >&2; exit 1; }
JAR="$PLATFORM/android.jar"
echo "build-tools: $BT"
echo "platform:    $PLATFORM"
echo "version:     $VERSION ($CODE)"

[ -f dist/game.wasm ] || { echo "run ./build.sh first" >&2; exit 1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/assets/www" "$WORK/classes" "$WORK/dex"
cp -R dist/. "$WORK/assets/www/"

"$BT/aapt2" compile --dir android/res -o "$WORK/res.zip"
"$BT/aapt2" link -o "$WORK/unsigned.apk" -I "$JAR" \
  --manifest android/AndroidManifest.xml -A "$WORK/assets" \
  --min-sdk-version 24 --target-sdk-version 34 \
  --version-code "$CODE" --version-name "$VERSION" \
  "$WORK/res.zip"

javac -Xlint:-options -source 8 -target 8 -bootclasspath "$JAR" -classpath "$JAR" \
  -d "$WORK/classes" $(find android/src -name '*.java')
"$BT/d8" --release --min-api 24 --lib "$JAR" --output "$WORK/dex" $(find "$WORK/classes" -name '*.class')
(cd "$WORK/dex" && zip -q -X "$WORK/unsigned.apk" classes.dex)

"$BT/zipalign" -f -p 4 "$WORK/unsigned.apk" "$WORK/aligned.apk"

# The key only ever exists as a private temp file.
umask 077
base64 -d "$KEY_B64" > "$WORK/key.pk8"
"$BT/apksigner" sign --key "$WORK/key.pk8" --cert "$CERT" --out "$OUT" "$WORK/aligned.apk"
rm -f "$WORK/key.pk8" "$OUT.idsig"
"$BT/apksigner" verify --print-certs "$OUT"
echo "built $OUT"
