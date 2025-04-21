package inputprocessor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time" // Add this line to resolve the undefined time errors
	// "strings" // Removed unused import
)

// Result holds extracted content details
// Renamed from ProcessedInput for brevity within this package
type Result struct {
	Body        string
	ContentType string
	FilePath    *string    // Pointer to store absolute path if applicable
	FileSize    *int64     // Pointer to store size if applicable
	URL         *string    // Pointer to store URL if applicable
	Mtime       *time.Time // Pointer to store file modification time if applicable
	Metadata    map[string]interface{}
}

// Processor defines the interface for processing input strings
type Processor interface {
	Process(ctx context.Context, input string) (Result, error)
}

// New creates a default processor implementation
// Renamed from NewDefaultProcessor for convention
func New() Processor {
	// You could add dependencies like a custom http.Client here if needed later
	return &defaultProcessor{}
}

type defaultProcessor struct {
	// client *http.Client // Example dependency
}

// Process implements the Processor interface
// Contains the logic moved from ContentService.PrepareContentInput
func (p *defaultProcessor) Process(ctx context.Context, input string) (Result, error) {
	res := Result{Metadata: map[string]interface{}{}}

	// --- Detect File ---
	fi, err := os.Stat(input)
	if err == nil { // Stat succeeded, it's something on the filesystem
		if fi.IsDir() {
			// Handle directories if needed, for now, log and treat as raw string below
			log.Printf("Input '%s' is a directory, treating as raw string for now.", input)
			// Or return an error: return res, fmt.Errorf("input '%s' is a directory, not a file", input)
		} else {
			// It's a file
			log.Printf("Input '%s' detected as a file.", input)
			data, readErr := os.ReadFile(input)
			if readErr != nil {
				// Check for specific errors like permission denied
				if errors.Is(readErr, os.ErrPermission) {
					return res, fmt.Errorf("permission denied reading file '%s': %w", input, readErr)
				}
				return res, fmt.Errorf("failed to read file '%s': %w", input, readErr)
			}

			// Detect content type (MIME)
			ct := http.DetectContentType(data)
			// Get absolute path
			absPath, pathErr := filepath.Abs(input)
			if pathErr != nil {
				log.Printf("WARN: Failed to get absolute path for '%s': %v. Using original path.", input, pathErr)
				absPath = input // Fallback to original path
			}
			fileSize := fi.Size()
			mtime := fi.ModTime() // Get modification time

			res.Body = string(data)
			res.ContentType = ct // Store pointer to mtime
			res.FilePath = &absPath  // Store pointer to absolute path
			res.FileSize = &fileSize // Store pointer to size
			res.Mtime = &mtime       // Store pointer to mtime
			res.Metadata["input_type"] = "file"
			res.Metadata["mtime"] = mtime.Format(time.RFC3339)
			return res, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// Stat failed with an error other than "does not exist"
		return res, fmt.Errorf("failed to stat input '%s': %w", input, err)
	}
	// If we reach here, it's either not on the filesystem or it's a directory we're treating as raw string

	// --- Detect URL ---
	parsedURL, urlErr := url.Parse(input)
	if urlErr == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
		log.Printf("Input '%s' detected as a URL.", input)
		// Use default client for simplicity, consider adding timeout via context
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, input, nil)
		if reqErr != nil {
			return res, fmt.Errorf("failed to create request for URL '%s': %w", input, reqErr)
		}

		resp, httpErr := http.DefaultClient.Do(req)
		if httpErr != nil {
			return res, fmt.Errorf("failed to fetch URL '%s': %w", input, httpErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Attempt to read body for context, but don't fail if reading fails
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // Limit read size
			return res, fmt.Errorf("failed to fetch URL '%s': status code %d %s - Body Hint: %s", input, resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return res, fmt.Errorf("failed to read response body from URL '%s': %w", input, readErr)
		}

		// Get Content-Type header, fallback to detection
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = http.DetectContentType(bodyBytes)
			log.Printf("Content-Type header missing for URL '%s', detected as '%s'", input, ct)
		} else {
			// Often includes charset, e.g., "text/html; charset=utf-8"
			// Keep the full header for now, might need parsing later.
		}

		urlStr := parsedURL.String() // Get the cleaned URL string
		res.Body = string(bodyBytes)
		res.ContentType = ct
		res.URL = &urlStr // Store pointer to URL string
		res.Metadata["input_type"] = "url"
		return res, nil
	}
	// Not a valid URL or scheme not http/https

	// --- Default: Treat as Raw String ---
	log.Printf("Input '%s' is not a valid file or URL, treating as raw string.", input)
	res.Body = input
	// Use a more specific content type for plain text
	res.ContentType = "text/plain; charset=utf-8"
	res.Metadata["input_type"] = "raw"
	return res, nil
}

// Ensure defaultProcessor satisfies the Processor interface.
var _ Processor = (*defaultProcessor)(nil)
