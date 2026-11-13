//go:build ignore

// Same approach as headlesscap: headless frames for eyeballing the
// active tab's lighter shadow, two tabs open. Run:
//
//	go run -tags headlesscap internal/shell/testdata/tabfloatcap
package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/windedge/peekdown/internal/shell"
)

func cap(tester *ui.Tester, out string) {
	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, tester.Image()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("WROTE", out)
}

func main() {
	dir := os.Getenv("PD_CAP_DIR")
	if dir == "" {
		dir = "C:/tmp"
	}
	s := shell.New()
	s.SetTheme = func(mygo.ThemeSource) {}
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		s.View(c)
	}, 1400, 900)
	tt.SetTitleBar(ui.TitleBar{Height: shell.TitleBarHeight})
	tt.Frame()

	s.OpenPaths([]string{dir + "/a.md", dir + "/b.md"})
	tt.Frame()
	tt.Frame()
	cap(tt, dir+"/tabs-float-light.png")

	tt2 := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		c.SetTheme(ui.DarkTheme())
		s.View(c)
	}, 1400, 900)
	tt2.SetTitleBar(ui.TitleBar{Height: shell.TitleBarHeight})
	tt2.Frame()
	tt2.Frame()
	cap(tt2, dir+"/tabs-float-dark.png")
}
