//go:build windows

package main

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// deleteTree removes key and its subkeys: DeleteKey refuses a key that
// has children, so the tree is walked bottom-up. Best effort, like reg
// delete /f: a failure only leaves a stale key behind.
func deleteTree(root registry.Key, path string) {
	if k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS); err == nil {
		subs, _ := k.ReadSubKeyNames(-1)
		for _, sub := range subs {
			deleteTree(root, path+`\`+sub)
		}
		k.Close()
	}
	registry.DeleteKey(root, path)
}

// cleanupRegistry deletes what the test wrote: the ProgID trees and the
// Applications entry whole, the OpenWithProgids values only, so entries
// an installer wrote next to them survive.
func cleanupRegistry() {
	for _, ext := range []string{"md", "markdown"} {
		progID := appProgID(ext)
		deleteTree(registry.CURRENT_USER, `Software\Classes\`+progID)
		if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\.`+ext+`\OpenWithProgids`, registry.SET_VALUE); err == nil {
			k.DeleteValue(progID)
			k.Close()
		}
	}
	deleteTree(registry.CURRENT_USER, `Software\Classes\Applications\Peekdown.exe`)
}

func mustString(t *testing.T, path, name string) string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		t.Fatalf("read %s\\%s: %v", path, name, err)
	}
	return v
}

// TestRegisterFileAssociations runs registerFileAssociations and reads
// the keys back. It writes the real ProgID keys under HKCU, so the
// cleanup removes what the test wrote and nothing more.
func TestRegisterFileAssociations(t *testing.T) {
	note, err := registerFileAssociations()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_ = note
	t.Cleanup(cleanupRegistry)

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
		if got := mustString(t, `Software\Classes\`+progID+`\shell\open\command`, ""); got != `"`+exe+`" "%1"` {
			t.Errorf("shell\\open\\command: %q", got)
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
	got := mustString(t, `Software\Classes\Applications\Peekdown.exe\shell\open\command`, "")
	if !strings.HasPrefix(got, `"`) || !strings.Contains(got, exe) {
		t.Errorf("Applications command: %q", got)
	}
}

// TestDeleteOrphanUserChoice exercises the orphaned-choice removal.
// Windows gives FileExts UserChoice a DACL that denies key creation to
// everything but itself, so a real machine takes the guarded branch and
// the full removal path only runs where the DACL is absent.
func TestDeleteOrphanUserChoice(t *testing.T) {
	const ext = "md"
	path := `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.` + ext + `\UserChoice`
	progID := appProgID(ext)
	class := `Software\Classes\` + progID
	t.Cleanup(func() { deleteTree(registry.CURRENT_USER, class) })

	// The guard must stay quiet and keep a choice it cannot rewrite.
	if _, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err == nil {
		deleteOrphanUserChoice(ext, progID)
		if _, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err != nil {
			t.Errorf("UserChoice deleted while writes are denied at %s", path)
		}
		return
	}

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
	deleteOrphanUserChoice(ext, progID)
	if choiceExists() {
		t.Errorf("orphaned UserChoice still present at %s", path)
	}

	// A choice naming a ProgID an install still defines is kept.
	if _, _, err := registry.CreateKey(registry.CURRENT_USER, class, registry.SET_VALUE); err != nil {
		t.Fatalf("create %s: %v", class, err)
	}
	mkChoice(progID)
	deleteOrphanUserChoice(ext, progID)
	if !choiceExists() {
		t.Errorf("defined UserChoice deleted at %s", path)
	}
	registry.DeleteKey(registry.CURRENT_USER, path)
}
