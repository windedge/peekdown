// Peekdown is a Markdown viewer, rewritten in Go on mygo's native UI.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// view builds the window's interface from the app's state.
func view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		ui.Text(c, "Peekdown").FontSize(28).Bold()
		ui.Text(c, "Markdown viewer").FontSize(14).TextColor(t.TextMuted)
	})
}

func main() {
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "Peekdown",
			Width:     1024,
			Height:    768,
			MinWidth:  480,
			MinHeight: 320,
			StateKey:  "main",
			Content:   ui.View(view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
