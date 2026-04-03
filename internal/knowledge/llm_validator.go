package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrLLMValidatorNotFound indicates that the LLM validator binary is not available.
var ErrLLMValidatorNotFound = errors.New("llm validator executable not found")

// LLMValidatorConfig holds configuration for the LLM validator subprocess.
type LLMValidatorConfig struct {
	// ExecutablePath is the path to the LLM validator binary.
	// Default: "pageindex"
	ExecutablePath string

	// Model is the LLM model name passed to the validator.
	// Default: "claude-sonnet-4-5"
	Model string

	// TimeoutSecs is the subprocess timeout in seconds.
	// Default: 30 (not currently enforced, but reserved for future use)
	TimeoutSecs int
}

// DefaultLLMValidatorConfig returns a LLMValidatorConfig with sensible defaults.
func DefaultLLMValidatorConfig() LLMValidatorConfig {
	return LLMValidatorConfig{
		ExecutablePath: "pageindex",
		Model:          "claude-sonnet-4-5",
		TimeoutSecs:    30,
	}
}

// ValidationResult is the response from the LLM validator.
type ValidationResult struct {
	// Valid indicates whether the LLM thinks this relationship exists.
	Valid bool `json:"valid"`

	// Confidence is the LLM's stated confidence [0.0, 1.0].
	Confidence float64 `json:"confidence"`

	// Reasoning is a one-sentence explanation.
	Reasoning string `json:"reasoning"`
}

// ValidateRelationship invokes the LLM validator subprocess to assess a single
// relationship. It builds a prompt from the relationship signals and sends it to
// the validator binary.
//
// On success, returns the parsed ValidationResult.
// If the validator binary is not found, returns ErrLLMValidatorNotFound.
// On other subprocess errors, returns nil result (graceful degradation).
func ValidateRelationship(cfg LLMValidatorConfig, rel *ManifestRelationship) (ValidationResult, error) {
	prompt := buildValidationPrompt(rel)
	bin := cfg.ExecutablePath
	if bin == "" {
		bin = "pageindex"
	}
	model := cfg.Model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	// Invoke the validator binary.
	cmd := exec.Command(bin, "query",
		"--query", prompt,
		"--model", model,
		"--format", "json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		errMsg := runErr.Error()
		if strings.Contains(errMsg, "executable file not found") ||
			strings.Contains(errMsg, "no such file or directory") {
			return ValidationResult{}, fmt.Errorf("llm validator not found: %w", ErrLLMValidatorNotFound)
		}
		var execErr *exec.Error
		if errors.As(runErr, &execErr) {
			return ValidationResult{}, fmt.Errorf("llm validator not found: %w", ErrLLMValidatorNotFound)
		}
		// Other subprocess errors — graceful degradation, return empty.
		return ValidationResult{}, nil
	}

	return parseValidationResponse(stdout.Bytes())
}

// buildValidationPrompt constructs the prompt sent to the LLM validator.
func buildValidationPrompt(rel *ManifestRelationship) string {
	// Build evidence summary from signals.
	var evidenceLines []string
	for _, sig := range rel.Signals {
		evidenceLines = append(evidenceLines, fmt.Sprintf("- [%s] %s", sig.Type, sig.Evidence))
	}
	evidenceBlock := strings.Join(evidenceLines, "\n")
	if evidenceBlock == "" {
		evidenceBlock = "- (no signals provided)"
	}

	return fmt.Sprintf(`You are a software architecture expert validating a discovered relationship between documents.

Source: %s
Target: %s
Relationship Type: %s

Discovery evidence (signals that triggered this relationship):
%s

Does this relationship actually exist in the software system?
Respond ONLY with valid JSON, no prose:
{"valid": true, "confidence": 0.85, "reasoning": "one sentence explanation"}`,
		rel.Source, rel.Target, rel.Type, evidenceBlock)
}

// parseValidationResponse parses the JSON response from the LLM validator.
// It uses bracket-finding to extract JSON from prose-wrapped responses.
func parseValidationResponse(raw []byte) (ValidationResult, error) {
	start := bytes.IndexByte(raw, '{')
	end := bytes.LastIndexByte(raw, '}')
	if start < 0 || end <= start {
		return ValidationResult{}, fmt.Errorf("llm validator: no JSON object in response")
	}
	jsonPart := raw[start : end+1]

	var result ValidationResult
	if err := json.Unmarshal(jsonPart, &result); err != nil {
		return ValidationResult{}, fmt.Errorf("llm validator: parse JSON: %w", err)
	}
	return result, nil
}
