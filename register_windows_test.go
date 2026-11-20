//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func mustString(t *testing.T, path, name string) string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		t.Fatalf("read %s %s: %v", path, name, err)
	}
	return v
}

// TestRegisterFileAssociations runs registerFileAssociations and reads
// the keys back. It writes the real ProgID keys under HKCU, so the keys
// are the registration itself: nothing cleans them up, or the .md
// default value would outlive its ProgID tree and orphan a double-click.
func TestRegisterFileAssociations(t *testing.T) {
	note, err := registerFileAssociations()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_ = note

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"md", "markdown"} {
		progID := appProgID(ext)
		if progID != "com.windedge.peekdown."+ext {
			t.Errorf("progID for .%s: %q", ext, progID)
		}
		if got := mustString(t, `Software\Classes\`+progID, ""); got != "Markdown Document" {
			t.Errorf("ProgID default: %q", got)
		}
		if got := mustString(t, `Software\Classes\`+progID, "FriendlyAppName"); got != "Peekdown" {
			t.Errorf("FriendlyAppName: %q", got)
		}
		if got := mustString(t, `Software\Classes\`+progID+`\DefaultIcon`, ""); got != exe+",0" {
			t.Errorf("DefaultIcon: %q", got)
		}
		if got := mustString(t, `Software\Classes\`+progID+`\shell\open`, "FriendlyAppName"); got != "Peekdown" {
			t.Errorf("shell open FriendlyAppName: %q", got)
		}
		if got := mustString(t, `Software\Classes\`+progID+`\shell\open\command`, ""); got != `"`+exe+`" "%1"` {
			t.Errorf("shell open command: %q", got)
		}
		if got := mustString(t, `Software\Classes\.`+ext, ""); got != progID {
			t.Errorf("extension default: %q", got)
		}
		if got := mustString(t, `Software\Classes\.`+ext+`\OpenWithProgids`, progID); got != "" {
			t.Errorf("OpenWithProgids %s: %q", progID, got)
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\.`+ext+`\OpenWithProgids`, registry.QUERY_VALUE)
		if err != nil {
			t.Fatalf("open OpenWithProgids: %v", err)
		}
		if _, _, err := k.GetStringValue(legacyProgID); err != nil {
			t.Errorf("OpenWithProgids %s: %v", legacyProgID, err)
		}
		k.Close()
	}
	exeName := filepath.Base(exe)
	apps := `Software\Classes\Applications\` + exeName
	if got := mustString(t, apps, "FriendlyAppName"); got != "Peekdown" {
		t.Errorf("Applications FriendlyAppName: %q", got)
	}
	if got := mustString(t, apps+`\shell\open`, "FriendlyAppName"); got != "Peekdown" {
		t.Errorf("Applications shell open FriendlyAppName: %q", got)
	}
	if got := mustString(t, apps+`\shell\open\command`, ""); got != `"`+exe+`" "%1"` {
		t.Errorf("Applications command: %q", got)
	}
}

// TestDeleteOrphanUserChoice exercises the orphaned-choice removal on a
// throwaway extension, never on the real .md state.
func TestDeleteOrphanUserChoice(t *testing.T) {
	const ext = "zktestassociation"
	base := `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.` + ext
	path := base + `\UserChoice`
	progID := appProgID(ext)
	class := `Software\Classes\` + progID
	otherProgID := "Other.Association.Test"
	otherClass := `Software\Classes\` + otherProgID
	t.Cleanup(func() {
		deleteTree(registry.CURRENT_USER, class)
		deleteTree(registry.CURRENT_USER, otherClass)
		deleteTree(registry.CURRENT_USER, base)
	})

	mkChoice := func(progid string) {
		t.Helper()
		k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := k.SetStringValue("ProgId", progid); err != nil {
			t.Fatalf("set ProgId: %v", err)
		}
		k.Close()
	}
	choiceExists := func() bool {
		_, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
		return err == nil
	}

	// A ghost ProgID has no defining key, so the choice is dropped.
	mkChoice("Ghost.ProgID")
	if got := userChoice(ext, progID); got != choiceOrphan {
		t.Errorf("ghost choice classified as %v", got)
	}
	deleteOrphanUserChoice(ext, progID)
	if choiceExists() {
		t.Errorf("orphaned UserChoice still present at %s", path)
	}

	// A choice naming a ProgID another app still defines is kept and
	// answered with a hint instead of a dialog.
	if _, _, err := registry.CreateKey(registry.CURRENT_USER, otherClass, registry.SET_VALUE); err != nil {
		t.Fatalf("create %s: %v", otherClass, err)
	}
	mkChoice(otherProgID)
	if got := userChoice(ext, progID); got != choiceOther {
		t.Errorf("other app choice classified as %v", got)
	}
	deleteOrphanUserChoice(ext, progID)
	if !choiceExists() {
		t.Errorf("other app UserChoice deleted at %s", path)
	}
	hint := choiceHint(ext, choiceOther)
	if !strings.Contains(hint, "Windows UserChoice") || !strings.Contains(hint, "Windows settings") {
		t.Errorf("other app hint: %q", hint)
	}
	registry.DeleteKey(registry.CURRENT_USER, path)
}

func TestChoiceHint(t *testing.T) {
	for _, c := range []choice{choiceMissing, choiceOurs} {
		if hint := choiceHint("md", c); hint != "" {
			t.Errorf("hint for %v: %q", c, hint)
		}
	}
	if hint := choiceHint("md", choiceOrphan); !strings.Contains(hint, "Removed a stale default for .md") {
		t.Errorf("orphan hint: %q", hint)
	}
}

// deleteTree removes key and its subkeys: DeleteKey refuses a key that
// has children, so the tree is walked bottom-up. Best effort: a
// failure only leaves a stale throwaway key behind.
func deleteTree(root registry.Key, path string) {
	if k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS); err == nil {
		subs, _ := k.ReadSubKeyNames(-1)
		for _, sub := range subs {
			deleteTree(root, path+"\\"+sub)
		}
		k.Close()
	}
	registry.DeleteKey(root, path)
}
