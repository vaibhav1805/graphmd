package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TreeNode represents a single node in a PageIndex hierarchical document tree.
// Each node corresponds to a heading section in a markdown file.
type TreeNode struct {
	Heading   string      `json:"heading"`
	Summary   string      `json:"summary"`
	Content   string      `json:"content,omitempty"`
	LineStart int         `json:"line_start"`
	LineEnd   int         `json:"line_end"`
	Children  []*TreeNode `json:"children,omitempty"`
}

// FileTree is the top-level per-file tree document written by the PageIndex
// strategy.  It wraps a TreeNode hierarchy for a single markdown file.
type FileTree struct {
	File string    `json:"file"`
	Root *TreeNode `json:"root"`
}

// SaveTreeFile persists ft to a .json file in dir/.bmd/trees/.  The output
// filename is derived from ft.File: e.g. "docs/api.md" → ".bmd/trees/api.json".
//
// Creates .bmd/trees/ directory if it doesn't exist.
// The file is written with 0644 permissions and 2-space indented JSON.
func SaveTreeFile(dir string, ft FileTree) error {
	// Create .bmd/trees directory if it doesn't exist
	treesDir := filepath.Join(dir, ".bmd", "trees")
	if err := os.MkdirAll(treesDir, 0755); err != nil {
		return fmt.Errorf("SaveTreeFile: mkdir %q: %w", treesDir, err)
	}

	base := filepath.Base(ft.File)
	stem := strings.TrimSuffix(base, ".md")
	outPath := filepath.Join(treesDir, stem+".json")

	data, err := json.MarshalIndent(ft, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveTreeFile: marshal %q: %w", ft.File, err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("SaveTreeFile: write %q: %w", outPath, err)
	}

	return nil
}

// LoadTreeFiles reads all *.json files from dir/.bmd/trees/ and unmarshals them
// into FileTree values.  Malformed files are logged to stderr and skipped.
// Returns a nil (empty) slice with no error when dir/.bmd/trees contains no tree files.
func LoadTreeFiles(dir string) ([]FileTree, error) {
	treesDir := filepath.Join(dir, ".bmd", "trees")
	pattern := filepath.Join(treesDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("LoadTreeFiles: glob %q: %w", pattern, err)
	}

	var trees []FileTree
	for _, path := range matches {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "LoadTreeFiles: read %q: %v (skipping)\n", path, readErr)
			continue
		}

		var ft FileTree
		if unmarshalErr := json.Unmarshal(data, &ft); unmarshalErr != nil {
			fmt.Fprintf(os.Stderr, "LoadTreeFiles: parse %q: %v (skipping)\n", path, unmarshalErr)
			continue
		}

		trees = append(trees, ft)
	}

	return trees, nil
}
