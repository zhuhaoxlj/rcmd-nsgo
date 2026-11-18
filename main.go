// Command rcmd-nsgo is a macOS menu bar app that switches applications
// when the right Command key is held, a reimplementation of rcmd built
// on the NSGo UI framework.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/mark/rcmd-nsgo/internal/app"
)

func main() {
	nsgo.Run(App, nsgo.HideDock())
}

func App(cx *core.Scope) {
	state := app.New(cx)
	state.Start()
}
