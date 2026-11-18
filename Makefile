GO ?= $(HOME)/sdk/go1.27.3/bin/go
APP := dist/rcmd-nsgo-0.1.0-macos_arm64/rcmd-nsgo.app

.PHONY: helper build run clean

# The native half: a small Swift executable the Go app talks to.
helper:
	mkdir -p build
	swiftc -O -framework AppKit -framework ApplicationServices helper/main.swift -o build/rcmd-helper

# Build the NSGo app, put the helper beside its executable, and sign both.
build: helper
	$(GO) tool nsgo build -target macos_arm64
	mkdir -p "$(APP)/Contents/MacOS"
	cp build/rcmd-helper "$(APP)/Contents/MacOS/rcmd-helper"
	codesign --force --deep --sign - "$(APP)"

run: helper
	$(GO) tool nsgo dev .

clean:
	rm -rf build dist
