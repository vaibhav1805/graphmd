package knowledge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanConfig controls directory and file filtering during scanning.
type ScanConfig struct {
	IgnoreDirs        []string // e.g., ["vendor", "node_modules"]
	IgnoreFiles       []string // e.g., ["CLAUDE.md", "*.lock"]
	IncludeHidden     bool     // -A flag: include .gitignore, .venv, etc.
	UseDefaultIgnores bool     // default true; uses DefaultIgnoreDirs if true
}

// DefaultIgnoreDirs contains the default set of directory patterns to skip.
var DefaultIgnoreDirs = []string{
	"vendor", "node_modules", ".git", "__pycache__",
	".venv", "dist", "build", "target", ".gradle",
	".next", "out", ".cache", "bin", "obj",
}

// hiddenDirs is the set of directory names that ScanDirectory will skip.
// All entries are lowercase; comparison is done case-insensitively on
// case-insensitive file systems, but an exact match is checked first.
var hiddenDirs = map[string]struct{}{
	".git":         {},
	".svn":         {},
	".hg":          {},
	".bzr":         {},
	"node_modules": {},
	".idea":        {},
	".vscode":      {},
	".DS_Store":    {},
}

// matchPattern checks if a name matches a glob pattern.
// Supports:
//   - Exact matches (e.g., "vendor" matches "vendor")
//   - Wildcard matches (e.g., "*.lock" matches "package.lock")
//   - Case-sensitive matching
func matchPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Handle wildcard patterns (*.ext)
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") {
		suffix := pattern[1:] // e.g., ".lock" from "*.lock"
		return strings.HasSuffix(name, suffix)
	}

	// Handle prefix wildcards (prefix*)
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := pattern[:len(pattern)-1] // e.g., "node_" from "node_*"
		return strings.HasPrefix(name, prefix)
	}

	// Exact match
	return name == pattern
}

// ScanDirectory walks the directory tree rooted at root and returns one
// Document for every ".md" file found.
//
// Rules applied during the walk:
//   - Only regular files with a ".md" extension are collected.
//   - Directories whose names begin with "." are skipped entirely
//     (hidden directories such as .git, .cache, …) unless IncludeHidden is true.
//   - Well-known vendor/tooling directories (node_modules, etc.) are skipped.
//   - Directories and files matching ignore patterns are skipped.
//   - Symbolic links are never followed (os.Lstat is used); this prevents
//     infinite loops caused by circular symlinks.
//
// The returned slice is sorted by RelPath (ascending, forward-slash separators).
// Returns an error if root cannot be accessed.
func ScanDirectory(root string, config ScanConfig) ([]Document, error) {
	// Validate root before walking.
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("knowledge.ScanDirectory: cannot access root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("knowledge.ScanDirectory: root %q is not a directory", root)
	}

	// Ensure root is absolute so RelPath computation works regardless of the
	// caller's working directory.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("knowledge.ScanDirectory: abs(%q): %w", root, err)
	}

	// Build effective ignore list by combining defaults and custom ignores.
	effectiveIgnoreDirs := []string{}
	if config.UseDefaultIgnores {
		effectiveIgnoreDirs = append(effectiveIgnoreDirs, DefaultIgnoreDirs...)
	}
	effectiveIgnoreDirs = append(effectiveIgnoreDirs, config.IgnoreDirs...)

	var docs []Document

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		// Propagate hard walk errors (e.g. permission denied on a parent dir).
		if walkErr != nil {
			return walkErr
		}

		name := d.Name()

		// --- Directory filtering ---
		if d.IsDir() {
			// Skip the root itself but let the walk descend into it.
			if path == absRoot {
				return nil
			}

			// Skip all hidden directories (name starts with ".") unless IncludeHidden is true.
			if strings.HasPrefix(name, ".") && !config.IncludeHidden {
				return filepath.SkipDir
			}

			// Skip well-known vendor/tooling directories.
			if _, skip := hiddenDirs[name]; skip {
				return filepath.SkipDir
			}

			// Skip directories matching ignore patterns.
			for _, pattern := range effectiveIgnoreDirs {
				if matchPattern(name, pattern) {
					return filepath.SkipDir
				}
			}

			return nil
		}

		// --- Symlink guard ---
		// d.Type().IsRegular() returns false for symlinks; we use Lstat to
		// confirm it is a regular file rather than following the link.
		if d.Type()&fs.ModeSymlink != 0 {
			// It is a symbolic link — skip it unconditionally.
			return nil
		}

		// We only want regular markdown files.
		if !d.Type().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}

		// Skip files matching ignore patterns.
		for _, pattern := range config.IgnoreFiles {
			if matchPattern(name, pattern) {
				return nil
			}
		}

		// Compute RelPath relative to the root (forward-slash separators).
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("knowledge.ScanDirectory: rel(%q, %q): %w", absRoot, path, err)
		}
		relPath := filepath.ToSlash(rel)

		doc, err := DocumentFromFile(path, relPath)
		if err != nil {
			// Soft-skip unreadable files; log is not available in stdlib-only
			// code, so we return the error and let the caller decide.
			return fmt.Errorf("knowledge.ScanDirectory: load %q: %w", path, err)
		}

		docs = append(docs, *doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("knowledge.ScanDirectory: walk %q: %w", absRoot, err)
	}

	// Sort by relative path for deterministic ordering.
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].RelPath < docs[j].RelPath
	})

	return docs, nil
}
