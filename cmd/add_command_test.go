package cmd

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestAddCommand_DBConnectionError(t *testing.T) {
	cmd := exec.Command("./mimir", "add", "--input", "repomix.md", "--source", "local", "--title", "repomix-mimir")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err == nil {
		t.Fatalf("expected error but command succeeded")
	}

	output := stderr.String()

	expectedErr := "failed to initialize app: init primary store: unable to ping database: failed to connect"
	expectedSQLState := "FATAL: role \"user\" does not exist (SQLSTATE 28000)"

	if !strings.Contains(output, expectedErr) {
		t.Errorf("expected error message to contain %q, got: %s", expectedErr, output)
	}

	if !strings.Contains(output, expectedSQLState) {
		t.Errorf("expected error message to contain %q, got: %s", expectedSQLState, output)
	}
}
