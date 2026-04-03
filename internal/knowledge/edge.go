package knowledge

import (
	"fmt"
	"strings"
)

// EdgeType categorises the semantic relationship between two documents.
// Constants are defined as strings so they are human-readable in JSON output
// and debug logs.
type EdgeType string

const (
	// EdgeReferences indicates the source document contains a markdown link to
	// the target document. Confidence: 1.0 (explicit, structural).
	EdgeReferences EdgeType = "references"

	// EdgeDependsOn indicates the source document explicitly states it depends
	// on the target (e.g. "depends on", "requires" in prose). Confidence: 0.7.
	EdgeDependsOn EdgeType = "depends-on"

	// EdgeCalls indicates the source document contains a code snippet that
	// calls a function or method defined in the target. Confidence: 0.9.
	EdgeCalls EdgeType = "calls"

	// EdgeImplements indicates the source document describes an implementation
	// of a contract or interface described in the target. Confidence: 0.7.
	EdgeImplements EdgeType = "implements"

	// EdgeMentions indicates the source document contains a textual mention of
	// the target (e.g. a service name following "integrates with"). Confidence: 0.7.
	EdgeMentions EdgeType = "mentions"

	// EdgeRelated indicates two documents are semantically related based on
	// TF-IDF vector similarity. Confidence range: [0.5, 0.75].
	EdgeRelated EdgeType = "related"
)

// Confidence values used by the three extractor types.
const (
	// ConfidenceLink is assigned to edges derived from explicit markdown links.
	ConfidenceLink float64 = 1.0

	// ConfidenceCode is assigned to edges derived from code import/call patterns.
	ConfidenceCode float64 = 0.9

	// ConfidenceMention is assigned to edges derived from prose mention patterns.
	ConfidenceMention float64 = 0.7

	// ConfidenceUnresolved is assigned to edges whose target file does not exist
	// on disk at graph-construction time.
	ConfidenceUnresolved float64 = 0.5
)

// Edge represents a directed relationship between two documents.
//
// Source and Target hold document IDs (== relative paths from the index root,
// forward-slash separated).  Type describes the relationship category, and
// Confidence is a normalised [0.0, 1.0] score indicating how certain the
// extractor is that the relationship is real.  Evidence is a human-readable
// string (e.g. the raw link text or matched phrase) that explains how the
// edge was discovered.
type Edge struct {
	// ID is a stable, deterministic identifier derived from Source, Target, and
	// Type.  It is used to deduplicate edges in the Graph.
	ID string

	// Source is the document ID of the node that references or depends on Target.
	Source string

	// Target is the document ID of the node being referenced or depended upon.
	Target string

	// Type describes the semantic relationship.
	Type EdgeType

	// Confidence is a score in [0.0, 1.0] expressing extraction certainty.
	Confidence float64

	// Evidence is a short human-readable description of where the relationship
	// was found (e.g. "[link text](./path.md)" or "import \"pkg\"").
	Evidence string
}

// NewEdge creates and validates an Edge.
//
// Returns an error when:
//   - source or target is empty
//   - confidence is outside [0.0, 1.0]
//   - source == target (self-loops are not allowed)
func NewEdge(source, target string, edgeType EdgeType, confidence float64, evidence string) (*Edge, error) {
	if source == "" {
		return nil, fmt.Errorf("knowledge.NewEdge: source must not be empty")
	}
	if target == "" {
		return nil, fmt.Errorf("knowledge.NewEdge: target must not be empty")
	}
	if confidence < 0.0 || confidence > 1.0 {
		return nil, fmt.Errorf("knowledge.NewEdge: confidence %.4f is outside [0.0, 1.0]", confidence)
	}
	if source == target {
		return nil, fmt.Errorf("knowledge.NewEdge: self-loop not allowed (source == target == %q)", source)
	}

	return &Edge{
		ID:         edgeID(source, target, edgeType),
		Source:     source,
		Target:     target,
		Type:       edgeType,
		Confidence: confidence,
		Evidence:   evidence,
	}, nil
}

// String returns a human-readable representation of the edge suitable for
// logging and debugging output.
//
// Format: "<source> --[type:confidence]--> <target>  (evidence)"
func (e *Edge) String() string {
	evidencePart := ""
	if e.Evidence != "" {
		evidencePart = fmt.Sprintf("  (%s)", e.Evidence)
	}
	return fmt.Sprintf("%s --[%s:%.2f]--> %s%s",
		e.Source, e.Type, e.Confidence, e.Target, evidencePart)
}

// Equal returns true when two edges connect the same source/target pair with
// the same type.  Confidence and Evidence are intentionally excluded from
// equality so that higher-confidence versions replace lower-confidence ones
// during deduplication without creating separate edges.
func (e *Edge) Equal(other *Edge) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.Source == other.Source &&
		e.Target == other.Target &&
		e.Type == other.Type
}

// edgeID computes a stable, deterministic string key for a directed edge.
// The format is "source\x00target\x00type" — the null-byte separator is chosen
// because it cannot appear in a file path and avoids collisions between paths
// that share a common prefix.
func edgeID(source, target string, t EdgeType) string {
	var b strings.Builder
	b.WriteString(source)
	b.WriteByte(0)
	b.WriteString(target)
	b.WriteByte(0)
	b.WriteString(string(t))
	return b.String()
}
