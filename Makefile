GO ?= $(HOME)/sdk/go1.27.3/bin/go
# Bumped when the release version changes; a tag build overrides it.
VERSION ?= 0.1.0
SLUG := rcmd-nsgo-$(VERSION)-macos-arm64
APP := dist/rcmd-nsgo-$(VERSION)-macos_arm64/rcmd-nsgo.app
DMG := dist/releases/$(SLUG).dmg
KEYCHAIN := $(CURDIR)/.signing/rcmd-nsgo.keychain-db
SIGN_IDENTITY ?= rcmd-nsgo Dev

.PHONY: helper build dmg run clean signing-identity sign

# The native half: a small Swift executable the Go app talks to.
helper:
	mkdir -p build
	swiftc -O -framework AppKit -framework ApplicationServices helper/main.swift -o build/rcmd-helper

# A fixed certificate makes the app's code identity stable across
# rebuilds, so macOS keeps its Accessibility and Input Monitoring grants
# instead of matching them against a cdhash that changes every build.
# Without it the app falls back to ad-hoc signing and the grants have to
# be given again after each build.
#
# Making the identity is a one-time local step, so a build machine sets
# CREATE_IDENTITY=0: a throwaway certificate there would be worse than
# ad-hoc signing — every build would carry its own designated requirement,
# which is the churn the identity exists to avoid.
CREATE_IDENTITY ?= 1
signing-identity:
	./scripts/make-signing-identity.sh

sign: $(if $(filter 1,$(CREATE_IDENTITY)),$(if $(wildcard $(KEYCHAIN)),,signing-identity))
	@if [ ! -f "$(KEYCHAIN)" ]; then \
		echo "warning: no signing identity; ad-hoc signing, grants reset on every build (run make signing-identity)" >&2; \
		codesign --force --deep --sign - "$(APP)"; \
		exit 0; \
	fi
	@security unlock-keychain -p "$$(cat .signing/password)" "$(KEYCHAIN)"
	codesign --force --sign "$(SIGN_IDENTITY)" --keychain "$(KEYCHAIN)" \
		--identifier dev.mark.rcmdnsgo.helper "$(APP)/Contents/MacOS/rcmd-helper"
	codesign --force --sign "$(SIGN_IDENTITY)" --keychain "$(KEYCHAIN)" \
		--identifier dev.mark.rcmdnsgo "$(APP)"
	codesign --verify --strict "$(APP)"

# Build the NSGo app, put the helper beside its executable, sign both, then
# package the disk image.
#
# nsgo build's own installer step runs before the helper is in the bundle
# and before the bundle is signed, so its disk image is made from the wrong
# bytes (no helper, no signature). It is left on for the install scripts it
# writes — they carry the version — and the image is then replaced by the
# one `make dmg` builds out of the finished bundle.
build: helper
	$(GO) tool nsgo build -target macos_arm64 -version $(VERSION)
	mkdir -p "$(APP)/Contents/MacOS"
	cp build/rcmd-helper "$(APP)/Contents/MacOS/rcmd-helper"
	$(MAKE) sign
	$(MAKE) dmg

# A drag-to-install disk image: the signed bundle, a link to /Applications,
# and the window layout the image has always been presented with. It is
# rebuilt every time rather than compared against the bundle's directory
# timestamp, which does not move when the helper is only overwritten.
dmg:
	rm -rf build/dmg && mkdir -p build/dmg dist/releases
	cp -R "$(APP)" build/dmg/
	cp -R packaging/dmg/.background build/dmg/.background
	cp packaging/dmg/.DS_Store build/dmg/.DS_Store
	ln -s /Applications build/dmg/Applications
	hdiutil create -volname rcmd-nsgo -srcfolder build/dmg -fs HFS+ \
		-format UDZO -imagekey zlib-level=9 -ov "$(DMG)"
	cd dist/releases && shasum -a 256 "$(notdir $(DMG))" > "$(notdir $(DMG)).sha256"
	rm -rf build/dmg
	@echo "  ✓ $(DMG)"

run: helper
	$(GO) tool nsgo dev .

clean:
	rm -rf build dist
