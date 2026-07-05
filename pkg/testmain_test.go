package decoder

import (
	"os"
	"testing"
)

// nopLogger satisfies Logger without producing output, so tests can exercise
// code paths that log unconditionally (e.g. error branches).
type nopLogger struct{}

func (nopLogger) Info(message string, module string) {}
func (nopLogger) Error(message string)               {}

// TestMain installs the package-level dependencies that decoder/main.go
// normally provides (logger and configuration), so every test starts from a
// silent, verbosity-0 baseline. Tests that need different settings must set
// them explicitly and restore the previous value before returning.
func TestMain(m *testing.M) {
	SetLogger(nopLogger{})
	SetConfiguration(Configuration{})
	os.Exit(m.Run())
}
