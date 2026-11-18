GO ?= $(HOME)/sdk/go1.27.3/bin/go
APP := dist/rcmd-nsgo-0.1.0-macos_arm64/rcmd-nsgo.app
KEYCHAIN := $(CURDIR)/.signing/rcmd-nsgo.keychain-db
SIGN_IDENTITY ?= rcmd-nsgo Dev

.PHONY: helper build run clean signing-identity sign

# The native half: a small Swift executable the Go app talks to.
helper:
	mkdir -p build
	swiftc -O -framework AppKit -framework ApplicationServices helper/main.swift -o build/rcmd-helper

# A fixed certificate makes the app's code identity stable across
# rebuilds, so macOS keeps its Accessibility and Input Monitoring grants
# instead of matching them against a cdhash that changes every build.
# Without it the app falls back to ad-hoc signing and the grants have to
# be given again after each build.
signing-identity:
	./scripts/make-signing-identity.sh

sign: $(if $(wildcard $(KEYCHAIN)),,signing-identity)
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

# Build the NSGo app, put the helper beside its executable, and sign both.
build: helper
	$(GO) tool nsgo build -target macos_arm64
	mkdir -p "$(APP)/Contents/MacOS"
	cp build/rcmd-helper "$(APP)/Contents/MacOS/rcmd-helper"
	$(MAKE) sign

run: helper
	$(GO) tool nsgo dev .

clean:
	rm -rf build dist
