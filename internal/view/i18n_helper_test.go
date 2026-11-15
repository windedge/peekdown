package view

import (
	"os"
	"testing"
)

// TestMain fixes the messages' language for the whole package, as the
// shell package's does: the labels the tests click stay English whatever
// the machine's language is.
func TestMain(m *testing.M) {
	os.Setenv("MYGO_LOCALE", "en")
	os.Exit(m.Run())
}
