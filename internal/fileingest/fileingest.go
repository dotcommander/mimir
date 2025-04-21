package fileingest

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// FileMeta holds metadata about a file to be ingested.
type FileMeta struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// ReadFileContent reads the entire content of the file at the given path.
func ReadFileContent(path string) ([]byte, error) {
	return os.ReadFile(path)
}

/*
DiscoverMarkdownFiles recursively finds all .md files under rootDir.

It returns a slice of FileMeta for each discovered file.
*/
func DiscoverMarkdownFiles(ctx context.Context, rootDir string) ([]FileMeta, error) {
	var files []FileMeta
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// If we can't access a file/dir, skip it but report error
			return err
		}
		// Skip directories
		if d.IsDir() {
			return nil
		}
		// Only .md files (case-insensitive)
		if filepath.Ext(d.Name()) == ".md" || filepath.Ext(d.Name()) == ".MD" {
			meta, metaErr := ExtractFileMeta(path)
			if metaErr != nil {
				// Skip files we can't stat, but continue
				return nil
			}
			files = append(files, meta)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

/*
ExtractFileMeta extracts metadata from a given file path.

Returns FileMeta with Name, Path, Size, and ModTime.
*/
func ExtractFileMeta(path string) (FileMeta, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileMeta{}, err
	}
	return FileMeta{
		Path:    path,
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}
