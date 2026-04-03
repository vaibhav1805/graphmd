package knowledge

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExportArgs holds parsed arguments for CmdExport.
type ExportArgs struct {
	From       string // source directory to export
	Output     string // output tar.gz file path
	DB         string // database path override
	Version    string // semantic version (e.g. "2.0.0")
	GitVersion bool   // auto-detect version from git describe --tags
	Publish    string // optional S3 URI (e.g. "s3://bucket/prefix")
}

// KnowledgeMetadata is the metadata stored in knowledge.json inside the archive.
type KnowledgeMetadata struct {
	Version   string    `json:"version"`
	Checksum  string    `json:"checksum,omitempty"` // "sha256:<hex>"
	CreatedAt time.Time `json:"created_at"`
	FileCount int       `json:"file_count"`
	DBSize    int64     `json:"db_size_bytes"`
	SourceDir string    `json:"source_dir"`
	FromRepo  string    `json:"from_repo,omitempty"`
	GitTag    string    `json:"git_tag,omitempty"`
	GitCommit string    `json:"git_commit,omitempty"`
}

// ParseExportArgs parses raw CLI arguments for the export command.
//
// Usage: bmd export --from <path> --output <path> [--db <path>] [--version VER] [--git-version] [--publish S3_URI]
func ParseExportArgs(args []string) (*ExportArgs, error) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var a ExportArgs
	fs.StringVar(&a.From, "from", ".", "Source directory to export")
	fs.StringVar(&a.Output, "output", "knowledge.tar.gz", "Output tar.gz file path")
	fs.StringVar(&a.DB, "db", "", "Database path override (default: .bmd/knowledge.db inside source dir)")
	fs.StringVar(&a.Version, "version", "", "Semantic version tag (e.g. 2.0.0)")
	fs.BoolVar(&a.GitVersion, "git-version", false, "Auto-detect version from git describe --tags")
	fs.StringVar(&a.Publish, "publish", "", "S3 URI to publish artifact (e.g. s3://bucket/path)")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	// Positional argument overrides --from.
	if pos := fs.Args(); len(pos) > 0 {
		a.From = pos[0]
	}

	return &a, nil
}

// CmdExport implements `bmd export`. It scans the source directory, builds
// fresh indexes, and packages everything into a versioned tar.gz archive
// with SHA256 checksums and optional git provenance metadata.
func CmdExport(args []string) error {
	a, err := ParseExportArgs(args)
	if err != nil {
		return err
	}

	absFrom, err := filepath.Abs(a.From)
	if err != nil {
		return fmt.Errorf("export: resolve dir %q: %w", a.From, err)
	}

	// Verify source directory exists.
	info, err := os.Stat(absFrom)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("export: source directory %q does not exist or is not a directory", absFrom)
	}

	// Resolve version: explicit flag > git auto-detect > default.
	version := a.Version
	if version == "" && a.GitVersion {
		if gitVer, gitErr := DetectGitVersion(absFrom); gitErr == nil && gitVer != "" {
			version = gitVer
			fmt.Fprintf(os.Stderr, "  Auto-detected version from git: %s\n", version)
		} else {
			fmt.Fprintf(os.Stderr, "  Warning: git version detection failed, using default\n")
		}
	}
	if version == "" {
		version = "1.0.0"
	}

	fmt.Fprintf(os.Stderr, "Exporting knowledge from %s (v%s)...\n", absFrom, version)
	start := time.Now()

	// Step 1: Build fresh indexes.
	dbPath := a.DB
	if dbPath == "" {
		dbPath = filepath.Join(absFrom, ".bmd", "knowledge.db")
	}
	fmt.Fprintf(os.Stderr, "  Building fresh indexes...\n")
	if err := buildIndex(absFrom, dbPath); err != nil {
		return fmt.Errorf("export: index build: %w", err)
	}

	// Step 2: Scan markdown files.
	// Use default Knowledge configuration for backward compatibility.
	k := DefaultKnowledge()
	docs, err := k.Scan(absFrom)
	if err != nil {
		return fmt.Errorf("export: scan: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  %d markdown files found\n", len(docs))

	// Step 3: Get DB size.
	var dbSize int64
	if stat, err := os.Stat(dbPath); err == nil {
		dbSize = stat.Size()
	}

	// Step 4: Collect all files for checksum and archiving.
	var entries []ArchiveFile

	// Add markdown files.
	for _, doc := range docs {
		entries = append(entries, ArchiveFile{
			DiskPath:    doc.Path,
			ArchivePath: filepath.ToSlash(doc.RelPath),
		})
	}

	// Add database file.
	if _, err := os.Stat(dbPath); err == nil {
		entries = append(entries, ArchiveFile{
			DiskPath:    dbPath,
			ArchivePath: ".bmd/knowledge.db",
		})
	}

	// Step 5: Compute SHA256 checksum over all content files.
	checksum, err := ComputeArchiveChecksum(entries)
	if err != nil {
		return fmt.Errorf("export: compute checksum: %w", err)
	}

	// Step 6: Detect git provenance.
	fromRepo, gitTag, gitCommit := DetectGitProvenance(absFrom)

	// Step 7: Create metadata with version, checksum, and git info.
	meta := KnowledgeMetadata{
		Version:   version,
		Checksum:  "sha256:" + checksum,
		CreatedAt: time.Now().UTC(),
		FileCount: len(docs),
		DBSize:    dbSize,
		SourceDir: absFrom,
		FromRepo:  fromRepo,
		GitTag:    gitTag,
		GitCommit: gitCommit,
	}

	// Step 8: Create tar.gz archive.
	outputPath := a.Output
	if !strings.HasSuffix(strings.ToLower(outputPath), ".tar.gz") && !strings.HasSuffix(strings.ToLower(outputPath), ".tgz") {
		outputPath += ".tar.gz"
	}

	// Ensure output directory exists.
	if dir := filepath.Dir(outputPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("export: create output dir %q: %w", dir, err)
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("export: create output %q: %w", outputPath, err)
	}
	defer outFile.Close()

	gzw := gzip.NewWriter(outFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Add knowledge.json metadata first.
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("export: marshal metadata: %w", err)
	}
	if err := addBytesToTar(tw, "knowledge.json", metaJSON); err != nil {
		return fmt.Errorf("export: add metadata: %w", err)
	}

	// Add content files.
	for _, entry := range entries {
		if err := addFileToTar(tw, entry.DiskPath, entry.ArchivePath); err != nil {
			return fmt.Errorf("export: add file %q: %w", entry.ArchivePath, err)
		}
	}

	elapsed := time.Since(start)
	absOutput, _ := filepath.Abs(outputPath)
	outStat, _ := os.Stat(outputPath)
	var sizeStr string
	if outStat != nil {
		sizeStr = humanBytes(outStat.Size())
	}

	fmt.Fprintf(os.Stderr, "  Archive: %s (%s)\n", absOutput, sizeStr)
	fmt.Fprintf(os.Stderr, "  Version: %s\n", version)
	fmt.Fprintf(os.Stderr, "  Checksum: sha256:%s\n", checksum)
	fmt.Fprintf(os.Stderr, "  Files: %d markdown + database + metadata\n", len(docs))
	fmt.Fprintf(os.Stderr, "  Completed in %dms\n", elapsed.Milliseconds())

	// Step 9: Publish to S3 if requested.
	if a.Publish != "" {
		fmt.Fprintf(os.Stderr, "  Publishing to %s...\n", a.Publish)
		if err := PublishToS3(absOutput, a.Publish, version); err != nil {
			return fmt.Errorf("export: publish: %w", err)
		}
		fmt.Fprintf(os.Stderr, "  Published successfully\n")
	}

	return nil
}

// addFileToTar adds a file from disk to the tar archive at the given archive path.
func addFileToTar(tw *tar.Writer, diskPath, archivePath string) error {
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    archivePath,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, f)
	return err
}

// addBytesToTar adds in-memory bytes to the tar archive at the given path.
func addBytesToTar(tw *tar.Writer, archivePath string, data []byte) error {
	header := &tar.Header{
		Name:    archivePath,
		Size:    int64(len(data)),
		Mode:    0o644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err := tw.Write(data)
	return err
}

// ─── checksum functions ──────────────────────────────────────────────────────

// ArchiveFile represents a file to be included in a knowledge archive.
type ArchiveFile struct {
	DiskPath    string
	ArchivePath string
}

// ComputeArchiveChecksum computes a SHA256 checksum over all files that will
// be included in the archive. Each file contributes its archive path (sorted
// for determinism) and content to the hash.
func ComputeArchiveChecksum(entries interface{}) (string, error) {
	var items []ArchiveFile

	switch v := entries.(type) {
	case []ArchiveFile:
		items = v
	default:
		return "", fmt.Errorf("unsupported entries type")
	}

	// Sort by archive path for deterministic checksums.
	sorted := make([]ArchiveFile, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ArchivePath < sorted[j].ArchivePath
	})

	h := sha256.New()
	for _, entry := range sorted {
		data, err := os.ReadFile(entry.DiskPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", entry.ArchivePath, err)
		}
		h.Write([]byte(entry.ArchivePath))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDirectoryChecksum computes SHA256 over extracted files in a directory,
// matching the export checksum algorithm. Skips knowledge.json itself since
// that file contains the checksum being verified.
func ComputeDirectoryChecksum(dir string) (string, error) {
	type entry struct {
		relPath string
		absPath string
	}
	var entries []entry

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Skip knowledge.json from checksum (it contains the checksum itself).
		if d.Name() == "knowledge.json" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{relPath: filepath.ToSlash(rel), absPath: path})
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort by relative path for deterministic checksums.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	h := sha256.New()
	for _, e := range entries {
		data, err := os.ReadFile(e.absPath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.relPath, err)
		}
		h.Write([]byte(e.relPath))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ValidateChecksum verifies that the extracted files match the checksum in
// the metadata. Returns nil if valid, error if mismatch or computation fails.
func ValidateChecksum(extractDir string, meta KnowledgeMetadata) error {
	if meta.Checksum == "" {
		return nil // no checksum to validate
	}

	expected := strings.TrimPrefix(meta.Checksum, "sha256:")

	actual, err := ComputeDirectoryChecksum(extractDir)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected sha256:%s, got sha256:%s (artifact may be corrupted)", expected, actual)
	}

	return nil
}

// ─── git integration ─────────────────────────────────────────────────────────

// DetectGitVersion attempts to determine the version from git tags.
// Returns the version string (without leading 'v') or empty string on failure.
func DetectGitVersion(dir string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	ver := strings.TrimSpace(string(out))
	ver = strings.TrimPrefix(ver, "v")
	return ver, nil
}

// DetectGitProvenance extracts git metadata (remote URL, current tag, commit hash).
func DetectGitProvenance(dir string) (fromRepo, gitTag, gitCommit string) {
	// Get remote URL.
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		fromRepo = strings.TrimSpace(string(out))
	}

	// Get current tag (exact match only).
	cmd = exec.Command("git", "describe", "--tags", "--exact-match")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		gitTag = strings.TrimSpace(string(out))
	}

	// Get commit hash.
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		gitCommit = strings.TrimSpace(string(out))
	}

	return
}

// ─── S3 distribution ─────────────────────────────────────────────────────────

// PublishToS3 uploads a local file to an S3 URI using the AWS CLI.
// Returns a descriptive error if the AWS CLI is not installed.
func PublishToS3(localPath, s3URI, version string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI not found. Install via: pip install awscli\n" +
			"Or download from: https://aws.amazon.com/cli/")
	}

	// Construct the full S3 destination path.
	destURI := s3URI
	if !strings.HasSuffix(destURI, ".tar.gz") {
		if !strings.HasSuffix(destURI, "/") {
			destURI += "/"
		}
		destURI += fmt.Sprintf("knowledge-v%s.tar.gz", version)
	}

	cmd := exec.Command("aws", "s3", "cp", localPath, destURI)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws s3 cp failed: %w", err)
	}

	return nil
}

// DownloadFromS3 downloads a file from S3 to a temporary local file.
// Returns the path to the temp file (caller must clean up).
func DownloadFromS3(s3URI string) (string, error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return "", fmt.Errorf("AWS CLI not found. Install via: pip install awscli\n" +
			"Or download from: https://aws.amazon.com/cli/")
	}

	tmpFile, err := os.CreateTemp("", "bmd-download-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmpFile.Close()

	cmd := exec.Command("aws", "s3", "cp", s3URI, tmpFile.Name())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("aws s3 cp failed: %w", err)
	}

	return tmpFile.Name(), nil
}
