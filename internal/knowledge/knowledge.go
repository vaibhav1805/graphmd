package knowledge

import (
	"fmt"
	"path/filepath"
)

// Knowledge is a domain service that encapsulates knowledge management operations
// including scanning, indexing, and querying markdown documentation.
//
// It holds configuration for how to scan and process files, allowing customization
// of ignore patterns and other scanning behavior through the ScanConfig field.
type Knowledge struct {
	// ScanConfig controls directory and file filtering during scanning operations.
	// It is applied to all ScanDirectory calls made by Knowledge methods.
	// If not explicitly set, defaults to ScanConfig{UseDefaultIgnores: true}.
	ScanConfig ScanConfig
}

// NewKnowledge creates a new Knowledge instance with the given ScanConfig.
// If you need a default configuration, use Knowledge{ScanConfig: ScanConfig{UseDefaultIgnores: true}}.
func NewKnowledge(scanConfig ScanConfig) *Knowledge {
	return &Knowledge{
		ScanConfig: scanConfig,
	}
}

// DefaultKnowledge creates a new Knowledge instance with default scanning configuration.
// This respects the default ignore directories but can be customized by modifying
// the ScanConfig field after creation.
func DefaultKnowledge() *Knowledge {
	return &Knowledge{
		ScanConfig: ScanConfig{UseDefaultIgnores: true},
	}
}

// Scan scans a directory for markdown files using the Knowledge's ScanConfig.
// It returns a slice of Document objects representing all .md files found in the directory tree.
func (k *Knowledge) Scan(dir string) ([]Document, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("Knowledge.Scan: resolve dir %q: %w", dir, err)
	}
	return ScanDirectory(absDir, k.ScanConfig)
}
