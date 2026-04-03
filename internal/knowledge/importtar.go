package knowledge

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportResult holds the result of importing a knowledge tar.
type ImportResult struct {
	ExtractDir string             // Directory where files were extracted
	Metadata   *KnowledgeMetadata // Parsed metadata from the archive
	DBPath     string             // Path to the extracted database
	FileCount  int                // Number of markdown files extracted
}

// ImportKnowledgeTar extracts a knowledge tar.gz archive to the specified
// directory. If destDir is empty, a temporary directory is created.
//
// The archive is expected to contain:
//   - knowledge.json (metadata)
//   - *.md files (markdown documents, possibly in subdirectories)
//   - .bmd/knowledge.db (pre-built index database)
//
// Returns an ImportResult with paths to the extracted content.
func ImportKnowledgeTar(tarPath, destDir string) (*ImportResult, error) {
	// Validate tar file exists.
	if _, err := os.Stat(tarPath); err != nil {
		return nil, fmt.Errorf("import: tar file %q: %w", tarPath, err)
	}

	// Create destination directory.
	if destDir == "" {
		var err error
		destDir, err = os.MkdirTemp("", "bmd-knowledge-*")
		if err != nil {
			return nil, fmt.Errorf("import: create temp dir: %w", err)
		}
	} else {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return nil, fmt.Errorf("import: create dir %q: %w", destDir, err)
		}
	}

	// Open and decompress.
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("import: open %q: %w", tarPath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("import: gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var meta *KnowledgeMetadata
	var fileCount int
	var dbPath string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("import: read tar: %w", err)
		}

		// Sanitize path to prevent directory traversal.
		cleanName := filepath.Clean(hdr.Name)
		if strings.Contains(cleanName, "..") {
			continue // skip suspicious paths
		}

		destPath := filepath.Join(destDir, cleanName)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return nil, fmt.Errorf("import: mkdir %q: %w", destPath, err)
			}
		case tar.TypeReg:
			// Create parent directories.
			parentDir := filepath.Dir(destPath)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return nil, fmt.Errorf("import: mkdir parent %q: %w", parentDir, err)
			}

			outFile, err := os.Create(destPath)
			if err != nil {
				return nil, fmt.Errorf("import: create %q: %w", destPath, err)
			}

			// Limit file size to prevent decompression bombs (1GB max per file).
			if _, err := io.Copy(outFile, io.LimitReader(tr, 1<<30)); err != nil {
				outFile.Close()
				return nil, fmt.Errorf("import: write %q: %w", destPath, err)
			}
			outFile.Close()

			// Track what we extracted.
			if cleanName == "knowledge.json" {
				data, err := os.ReadFile(destPath)
				if err == nil {
					var m KnowledgeMetadata
					if err := json.Unmarshal(data, &m); err == nil {
						meta = &m
					}
				}
			}
			if strings.HasSuffix(cleanName, ".md") {
				fileCount++
			}
			if cleanName == ".bmd/knowledge.db" || cleanName == "knowledge.db" {
				dbPath = destPath
			}
		}
	}

	// Validate we got a proper knowledge archive.
	if meta == nil {
		return nil, fmt.Errorf("import: archive missing knowledge.json metadata")
	}

	// Validate checksum if present.
	if meta.Checksum != "" {
		if err := ValidateChecksum(destDir, *meta); err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}
	}

	// If dbPath was extracted to .bmd/knowledge.db, that's expected.
	// If it was at root level, move it to .bmd/knowledge.db for consistency.
	if dbPath != "" && filepath.Dir(dbPath) == destDir {
		bmdDir := filepath.Join(destDir, ".bmd")
		_ = os.MkdirAll(bmdDir, 0o755)
		newPath := filepath.Join(bmdDir, "knowledge.db")
		if err := os.Rename(dbPath, newPath); err == nil {
			dbPath = newPath
		}
	}

	return &ImportResult{
		ExtractDir: destDir,
		Metadata:   meta,
		DBPath:     dbPath,
		FileCount:  fileCount,
	}, nil
}

// ImportKnowledgeFromS3 downloads a knowledge archive from S3 and imports it.
// Returns the same result as ImportKnowledgeTar.
func ImportKnowledgeFromS3(s3URI, destDir string) (*ImportResult, error) {
	localPath, err := DownloadFromS3(s3URI)
	if err != nil {
		return nil, fmt.Errorf("import from s3: %w", err)
	}
	defer os.Remove(localPath)

	return ImportKnowledgeTar(localPath, destDir)
}

// LoadFromKnowledgeTar extracts a knowledge tar and opens the pre-built
// database, ready for queries. The caller should defer db.Close().
//
// Returns the database, the extract directory path, and any error.
func LoadFromKnowledgeTar(tarPath, destDir string) (*Database, string, error) {
	result, err := ImportKnowledgeTar(tarPath, destDir)
	if err != nil {
		return nil, "", err
	}

	if result.DBPath == "" {
		return nil, result.ExtractDir, fmt.Errorf("import: archive does not contain a knowledge database")
	}

	db, err := OpenDB(result.DBPath)
	if err != nil {
		return nil, result.ExtractDir, fmt.Errorf("import: open database: %w", err)
	}

	return db, result.ExtractDir, nil
}
