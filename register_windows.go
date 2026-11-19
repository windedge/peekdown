//go:build windows

package main

import (
	"os"
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

// registerFileAssociations writes the HKCU keys for portable installs:
// the installer's ProgIDs plus the extension default values the Rust
// build wrote, and the Open With application entry, pointing at the
// running executable.
func registerFileAssociations() (note string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	open := `"` + exe + `" "%1"`
	const docName = "Markdown Document" // mygo.json fileAssociations[0].name

	for _, ext := range []string{"md", "markdown"} {
		progID := appProgID(ext)
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
		// A UserChoice naming a ProgID whose key no longer exists leaves a
		// double-click without a handler; drop it so Windows asks again.
		if !deleteOrphanUserChoice(ext, progID) {
			// Only the legacy ProgID is orphaned in the wild; the hint
			// applies to the extension the failed choice guards.
			note = "The current default for ." + ext + " points to the removed program (" + legacyProgID + "); it could not be cleared without elevation. Pick Peekdown once under Open With, or run this command again as administrator."
		}
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\Applications\Peekdown.exe\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return "", err
	}
	if err := k.SetStringValue("", open); err != nil {
		k.Close()
		return "", err
	}
	k.Close()

	notifyAssocChanged()
	return note, nil
}

// deleteOrphanUserChoice removes the FileExts UserChoice for ext when it
// names a ProgID with no defining key under HKCR: the hash binding means
// it cannot be rewritten in place, and an orphan leaves a double-click
// with no handler. It reports whether the choice is gone; deletion can
// be denied by the key's DACL without elevation, which the caller turns
// into a hint.
func deleteOrphanUserChoice(ext, progID string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.`+ext+`\UserChoice`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	progid, _, _ := k.GetStringValue("ProgId")
	k.Close()
	if progid == progID {
		return true
	}
	// A choice naming a ProgID some install still defines is kept, so a
	// Rust or installer build keeps working; the legacy ProgID itself is
	// only kept when its key exists.
	if k, err := registry.OpenKey(registry.CLASSES_ROOT, progid, registry.QUERY_VALUE); err == nil {
		k.Close()
		return true
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.`+ext+`\UserChoice`); err != nil {
		return false
	}
	return true
}
