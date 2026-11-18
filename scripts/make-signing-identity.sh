#!/bin/sh
# Creates the project's code-signing identity once.
#
# macOS keys Accessibility and Input Monitoring grants to an app's
# designated requirement. An ad-hoc signature (`codesign -s -`) has
# "cdhash H"..." as its requirement, and a cdhash is a hash of the
# binary: every rebuild produced a new identity, so macOS kept the old
# grant but no longer matched it, and the app reported "Needed" while
# System Settings still showed the switch on.
#
# Signing with a fixed certificate instead makes the requirement
# "identifier ... and certificate leaf = H"..."", which survives
# rebuilds, so a grant is asked once and then kept.
#
# The keychain lives in .signing/ next to the repo and is gitignored.
# Its password only guards a throwaway development certificate.

set -eu

repo=$(cd "$(dirname "$0")/.." && pwd)
dir="$repo/.signing"
keychain="$dir/rcmd-nsgo.keychain-db"
password_file="$dir/password"
identity="rcmd-nsgo Dev"

mkdir -p "$dir"
chmod 700 "$dir"

if [ -f "$keychain" ] && security find-identity -p codesigning "$keychain" 2>/dev/null | grep -q "$identity"; then
	echo "signing identity already exists: $identity"
	exit 0
fi

password=$(openssl rand -hex 24)
printf '%s' "$password" >"$password_file"
chmod 600 "$password_file"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

openssl req -x509 -newkey rsa:2048 -nodes \
	-keyout "$work/key.pem" -out "$work/cert.pem" \
	-days 3650 -subj "/CN=$identity" \
	-addext "keyUsage=critical,digitalSignature" \
	-addext "extendedKeyUsage=critical,codeSigning" 2>/dev/null

openssl pkcs12 -export -legacy \
	-inkey "$work/key.pem" -in "$work/cert.pem" \
	-out "$work/identity.p12" -passout "pass:$password" 2>/dev/null

security create-keychain -p "$password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$password" "$keychain"
security import "$work/identity.p12" -k "$keychain" -P "$password" \
	-T /usr/bin/codesign -T /usr/bin/security >/dev/null
# The partition list is what lets codesign use the key without a prompt.
security set-key-partition-list -S apple-tool:,apple:,codesign: \
	-k "$password" "$keychain" >/dev/null 2>&1

echo "created signing identity: $identity"
echo "keychain: $keychain"
