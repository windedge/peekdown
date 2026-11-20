//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// appIdentifier must match mygo.json: mygo's NSIS script derives the
// file-association ProgIDs from it.
const appIdentifier = "com.windedge.peekdown"

// appProgID derives the ProgID the mygo NSIS script writes for ext: the
// identifier filtered to ASCII letters, digits and dots, then "." plus
// the lowercased extension. --register must write the same ProgIDs the
// installer does, or the two would fight over the associations.
func appProgID(ext string) string {
	prefix := strings.Map(func(r rune) rune {
		if r < 0x80 && (r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return r
		}
		return -1
	}, appIdentifier)
	return prefix + "." + strings.ToLower(ext)
}

var shell32 = syscall.NewLazyDLL("shell32.dll")

// notifyAssocChanged tells Explorer the associations changed, so the
// Open With lists refresh without a logoff. A failure is not worth
// reporting: the registry is already updated.
func notifyAssocChanged() {
	// SHCNE_ASSOCCHANGED, SHCNF_IDLIST.
	shell32.NewProc("SHChangeNotify").Call(0x08000000, 0, 0, 0)
}

// legacyProgID is the ProgID the Rust build's --register wrote into the
// extension key's default value (and that a user then picked as the
// default): Windows stores it in FileExts .md UserChoice with a hash
// bound to the string, so once it exists the Go ProgIDs can never win a
// double-click. Re-registering deletes the orphaned choice if the key
// defining it is gone, so the system asks again.
const legacyProgID = "Peekdown.Markdown"

// registerFileAssociations writes the same keys per extension the Rust
// build's --register (registry.rs) writes, under HKCU so it needs no
// elevation: the ProgID with its DefaultIcon and shell\open\command, the
// extension's default value, and the Applications entry pointing at the
// running executable. It also writes the extension's OpenWithProgids the
// mygo installer writes, so either path lists Peekdown the same way.
//
// The extension default and its ProgID are the double-click handler.
// Windows only overrides them with a UserChoice naming an installed app;
// the returned hint covers that case.
func registerFileAssociations() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeName := filepath.Base(exe)
	open := `"` + exe + `" "%1"`
	const docName = "Markdown Document" // mygo.json fileAssociations[0].name
	var notes []string

	for _, ext := range []string{"md", "markdown"} {
		progID := appProgID(ext)
		// The ProgID root, its icon and verb mirror registry.rs. The
		// FriendlyAppName values and the command naming the exe, not the
		// running path, are Rust's; the running path is at least as
		// accurate.
		class := `Software\Classes\` + progID
		k, _, err := registry.CreateKey(registry.CURRENT_USER, class, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		for _, v := range [][2]string{
			{"", docName},
			{"FriendlyAppName", "Peekdown"},
		} {
			if err := k.SetStringValue(v[0], v[1]); err != nil {
				k.Close()
				return "", err
			}
		}
		k.Close()
		k, _, err = registry.CreateKey(registry.CURRENT_USER, class+`\DefaultIcon`, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		if err := k.SetStringValue("", exe+",0"); err != nil {
			k.Close()
			return "", err
		}
		k.Close()
		k, _, err = registry.CreateKey(registry.CURRENT_USER, class+`\shell\open`, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		if err := k.SetStringValue("FriendlyAppName", "Peekdown"); err != nil {
			k.Close()
			return "", err
		}
		k.Close()
		k, _, err = registry.CreateKey(registry.CURRENT_USER, class+`\shell\open\command`, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		if err := k.SetStringValue("", open); err != nil {
			k.Close()
			return "", err
		}
		k.Close()
		// The extension's default value selects the ProgID for a plain
		// double-click; the Rust build wrote it, so this must too.
		k, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\.`+ext, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		if err := k.SetStringValue("", progID); err != nil {
			k.Close()
			return "", err
		}
		k.Close()
		// The installer's OpenWith list entry; the default value above is
		// what makes Peekdown the plain double-click handler.
		k, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\.`+ext+`\OpenWithProgids`, registry.SET_VALUE)
		if err != nil {
			return "", err
		}
		for _, id := range []string{progID, legacyProgID} {
			if err := k.SetStringValue(id, ""); err != nil {
				k.Close()
				return "", err
			}
		}
		k.Close()
		// An orphaned UserChoice shadows the HKCR chain with no handler
		// behind it; deleting it lets the chain above take effect.
		c := userChoice(ext, progID)
		if c == choiceOrphan && !deleteOrphanUserChoice(ext, progID) {
			return "", fmt.Errorf("the default for .%s points to the removed program %q and its registry key is write-protected; rerun as administrator", ext, legacyProgID)
		}
		if hint := choiceHint(ext, c); hint != "" {
			notes = append(notes, hint)
		}
	}
	// The Applications entry makes Peekdown listed by name in Open With;
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName, registry.SET_VALUE)
	if err != nil {
		return "", err
	}
	if err := k.SetStringValue("FriendlyAppName", "Peekdown"); err != nil {
		k.Close()
		return "", err
	}
	k.Close()
	k, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\shell\open`, registry.SET_VALUE)
	if err != nil {
		return "", err
	}
	if err := k.SetStringValue("FriendlyAppName", "Peekdown"); err != nil {
		k.Close()
		return "", err
	}
	k.Close()
	k, _, err = registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return "", err
	}
	if err := k.SetStringValue("", open); err != nil {
		k.Close()
		return "", err
	}
	k.Close()

	notifyAssocChanged()
	return strings.Join(notes, "\n"), nil
}

// choice tells where the FileExts UserChoice for ext points.
type choice int

const (
	choiceMissing choice = iota
	choiceOurs
	choiceOrphan
	choiceOther
)

// userChoice classifies the UserChoice for ext against progID. A choice
// naming a ProgID another install still defines must be left alone.
func userChoice(ext, progID string) choice {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.`+ext+`\UserChoice`, registry.QUERY_VALUE)
	if err != nil {
		return choiceMissing
	}
	progid, _, _ := k.GetStringValue("ProgId")
	k.Close()
	if progid == progID {
		return choiceOurs
	}
	if k, err := registry.OpenKey(registry.CLASSES_ROOT, progid, registry.QUERY_VALUE); err == nil {
		k.Close()
		return choiceOther
	}
	return choiceOrphan
}

// choiceHint returns the CLI note for a UserChoice that needs the user's
// attention; a missing choice or one already pointing here needs none.
func choiceHint(ext string, c choice) string {
	switch c {
	case choiceOrphan:
		return fmt.Sprintf("Removed a stale default for .%s; double-clicking now uses the registered Peekdown handler.", ext)
	case choiceOther:
		return fmt.Sprintf("The default for .%s is set by Windows UserChoice to another app; change it in Windows settings if you want Peekdown.", ext)
	}
	return ""
}

// deleteOrphanUserChoice removes the FileExts UserChoice for ext when it
// names a ProgID with no defining key under HKCR: the hash binding means
// it cannot be rewritten in place, and an orphan leaves a double-click
// with no handler. It reports whether the choice is gone; the key's
// DACL can deny deletion, and callers must not treat that as success.
func deleteOrphanUserChoice(ext, progID string) bool {
	if userChoice(ext, progID) != choiceOrphan {
		return true
	}
	return registry.DeleteKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.`+ext+`\UserChoice`) == nil
}
