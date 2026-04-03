package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrPageIndexNotFound is returned when the pageindex executable cannot be
// found on PATH (or at the configured ExecutablePath).  Callers should test
// with errors.Is(err, ErrPageIndexNotFound).
var ErrPageIndexNotFound = errors.New("pageindex executable not found")

// PageIndexConfig holds configuration for the PageIndex subprocess runner.
type PageIndexConfig struct {
	// ExecutablePath is the path to the pageindex CLI binary.
	// Defaults to "pageindex" (resolved via PATH).
	ExecutablePath string

	// Model is the LLM model name passed to pageindex via --model flag.
	// Defaults to "claude-sonnet-4-5".
	Model string
}

// DefaultPageIndexConfig returns a PageIndexConfig with sensible defaults:
// pageindex resolved via PATH, claude-sonnet-4-5 as the LLM model.
func DefaultPageIndexConfig() PageIndexConfig {
	return PageIndexConfig{
		ExecutablePath: "pageindex",
		Model:          "claude-sonnet-4-5",
	}
}

// RunPageIndex calls the pageindex CLI as a subprocess to generate a
// hierarchical tree index for the given filePath.
//
// The subprocess is invoked as:
//
//	pageindex index --file <filePath> --model <cfg.Model> --format json
//
// Subprocess stderr is passed through to os.Stderr so progress output is
// visible.  Stdout is captured and unmarshaled into a FileTree.
//
// Errors:
//   - Returns (FileTree{}, ErrPageIndexNotFound wrapping) when the binary is
//     not found.
//   - Returns (FileTree{}, err) on subprocess failure or JSON parse error.
func RunPageIndex(cfg PageIndexConfig, filePath string) (FileTree, error) {
	cmd := exec.Command(
		cfg.ExecutablePath,
		"index",
		"--file", filePath,
		"--model", cfg.Model,
		"--format", "json",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		// Detect "not found" errors from exec or the OS.
		errMsg := runErr.Error()
		if strings.Contains(errMsg, "executable file not found") ||
			strings.Contains(errMsg, "no such file or directory") {
			return FileTree{}, fmt.Errorf(
				"pageindex not found: install via 'pip install pageindex' or set --pageindex-bin flag: %w",
				ErrPageIndexNotFound,
			)
		}

		// Also handle the case where exec.LookPath already failed (exec.Error).
		var execErr *exec.Error
		if errors.As(runErr, &execErr) {
			return FileTree{}, fmt.Errorf(
				"pageindex not found: install via 'pip install pageindex' or set --pageindex-bin flag: %w",
				ErrPageIndexNotFound,
			)
		}

		return FileTree{}, fmt.Errorf("pageindex: subprocess error: %w", runErr)
	}

	var ft FileTree
	if err := json.Unmarshal(stdout.Bytes(), &ft); err != nil {
		return FileTree{}, fmt.Errorf("pageindex: parse JSON output: %w", err)
	}

	return ft, nil
}
