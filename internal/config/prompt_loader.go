package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultPromptDir is the subdirectory within the user's config directory.
const defaultPromptDir = ".config/mimir/prompts"

// LoadPromptContent resolves the path for a prompt template and reads its content.
// If configuredPath is absolute, it's used directly.
// If configuredPath is relative or empty, it's treated as a filename within ~/.config/mimir/prompts/.
func LoadPromptContent(configuredPath, defaultFilename string) (string, error) {
	finalPath := configuredPath

	// If the path is relative or empty, resolve it against the default user config directory.
	if !filepath.IsAbs(configuredPath) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		// Use defaultFilename if configuredPath is empty, otherwise use configuredPath as the filename.
		filename := configuredPath
		if filename == "" {
			filename = defaultFilename
		}

		finalPath = filepath.Join(homeDir, defaultPromptDir, filename)
	}

	// Ensure the directory exists if we are using the default path logic
	if !filepath.IsAbs(configuredPath) {
		dir := filepath.Dir(finalPath)
		if err := os.MkdirAll(dir, 0750); err != nil { // Use 0750 for permissions
			return "", fmt.Errorf("failed to create default prompt directory '%s': %w", dir, err)
		}
	}


	// Read the file content from the final resolved path.
	promptBytes, err := os.ReadFile(finalPath)
	if err != nil {
		// Provide a more helpful error message if the file doesn't exist at the default location
		if os.IsNotExist(err) && !filepath.IsAbs(configuredPath) {
			return "", fmt.Errorf("prompt file not found at default location '%s'. Please create it or specify an absolute path in config.yaml: %w", finalPath, err)
		}
		return "", fmt.Errorf("failed to read prompt file '%s': %w", finalPath, err)
	}

	return string(promptBytes), nil
}
