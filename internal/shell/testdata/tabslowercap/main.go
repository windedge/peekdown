//go:build ignore

// Like tabfloatcap: a headless 1400x900 light frame with two tabs, for
// eyeballing the lowered tabs. Run:
//
//	go run -tags tabslowercap internal/shell/testdata/tabslowercap
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

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
	for _, p := range []string{dir + "/a.md", dir + "/b.md"} {
		if err := os.WriteFile(p, []byte("# "+filepath.Base(p)+"\n\nbody\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
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
	cap(tt, dir+"/tabs-lower.png")
}
