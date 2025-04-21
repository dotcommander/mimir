package tests

import (
	"bytes"
	"testing"

	"os" // Import os
	// "testing" // Removed duplicate import

	"mimir/cmd"
)

func TestRootCommand(t *testing.T) {
	// Capture output from root command execution.
	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the command.
	// WARNING: cmd.Execute() calls os.Exit(1) on error internally,
	// which will terminate the test run prematurely if the command fails.
	// This test primarily checks if the command runs and produces output (e.g., help text)
	// without panicking or exiting successfully. It cannot easily test error paths.
	cmd.Execute() // Call the exported Execute function

	// Restore stdout and capture output
	w.Close()
	os.Stdout = oldStdout
	buf := new(bytes.Buffer)
	buf.ReadFrom(r) // Read captured stdout
	output := buf.String()

	// Basic check: Ensure some output (likely help text) is produced.
	if output == "" {
		t.Error("Expected output from root command, got empty string")
	}
}
