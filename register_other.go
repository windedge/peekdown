//go:build !windows

package main

import "errors"

// registerFileAssociations fills in per platform: macOS and Linux each
// register file associations in their own way.
func registerFileAssociations() error {
	return errors.New("file association registration is only supported on Windows")
}
