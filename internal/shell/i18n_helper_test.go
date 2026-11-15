package shell

import (
	"os"
	"testing"
)

// TestMain fixes the messages' language for the whole package: without it
// the catalog's accessors resolve MYGO_LOCALE, else the system language,
// which on a zh-CN machine would answer the tests' labels in Chinese.
// Every label assertion below reads English.
func TestMain(m *testing.M) {
	os.Setenv("MYGO_LOCALE", "en")
	os.Exit(m.Run())
}
