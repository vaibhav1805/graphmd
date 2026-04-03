package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// All fields verified against spec. No schema changes needed.
// ManifestRelationship: source/target/type/confidence/signals/reviewed/status/user_notes — all present.
// ManifestSignal: type/value/evidence — all present.
// ManifestSummary.Pending computed as Total - Reviewed (Accepted + Rejected).
// File constants match spec exactly.

// Manifest file names.
const (
	// DiscoveredManifestFile is the auto-generated file containing all
	// discovered relationships with their evidence signals.  This file is
	// overwritten on every index run and should be treated as read-only.
	DiscoveredManifestFile = ".bmd-relationships-discovered.yaml"

	// AcceptedManifestFile is the user-edited file that persists review
	// decisions (accepted / rejected).  It is never overwritten by indexing.
	AcceptedManifestFile = ".bmd-relationships.yaml"
)

// ManifestSignal captures a single piece of evidence for a relationship.
type ManifestSignal struct {
	Type     string  `yaml:"type"`
	Value    float64 `yaml:"value"`
	Evidence string  `yaml:"evidence"`
}

// ManifestRelationship is a single relationship entry in the manifest YAML.
type ManifestRelationship struct {
	Source     string           `yaml:"source"`
	Target     string           `yaml:"target"`
	Type       string           `yaml:"type"`
	Confidence float64          `yaml:"confidence"`
	Signals    []ManifestSignal `yaml:"signals"`

	// Review fields (only meaningful in the accepted manifest).
	Reviewed  bool   `yaml:"reviewed"`
	Status    string `yaml:"status"`     // "pending", "accepted", "rejected"
	UserNotes string `yaml:"user_notes"` // user commentary
}

// AlgorithmVersions records the version of each extraction algorithm used.
type AlgorithmVersions struct {
	Cooccurrence string `yaml:"cooccurrence"`
	Structural   string `yaml:"structural"`
	Semantic     string `yaml:"semantic"`
	NER          string `yaml:"ner"`
}

// RelationshipManifest is the top-level YAML document.
type RelationshipManifest struct {
	Version           string                 `yaml:"version"`
	Generated         string                 `yaml:"generated"`
	AlgorithmVersions AlgorithmVersions      `yaml:"algorithm_versions"`
	Relationships     []ManifestRelationship `yaml:"relationships"`
}

// manifestKey returns a stable key for grouping edges by (source, target).
func manifestKey(source, target string) string {
	return source + "\x00" + target
}

// manifestGroup aggregates signals for a single (source, target) pair during
// manifest generation.
type manifestGroup struct {
	source     string
	target     string
	edgeType   string
	confidence float64
	signals    []ManifestSignal
}

// GenerateRelationshipManifest creates a RelationshipManifest from a set of
// graph edges and an optional ComponentRegistry.
//
// Edges are grouped by (source, target) pair.  When a registry is provided,
// its signals are included in the manifest; otherwise only edge evidence is
// used.
func GenerateRelationshipManifest(edges []*Edge, registry *ComponentRegistry) *RelationshipManifest {
	groups := make(map[string]*manifestGroup)

	for _, e := range edges {
		key := manifestKey(e.Source, e.Target)
		g, ok := groups[key]
		if !ok {
			g = &manifestGroup{
				source:   e.Source,
				target:   e.Target,
				edgeType: string(e.Type),
			}
			groups[key] = g
		}

		sig := ManifestSignal{
			Type:     signalTypeFromEdge(e),
			Value:    e.Confidence,
			Evidence: e.Evidence,
		}
		g.signals = append(g.signals, sig)

		if e.Confidence > g.confidence {
			g.confidence = e.Confidence
			g.edgeType = string(e.Type)
		}
	}

	// Merge registry signals if available.
	if registry != nil {
		mergeRegistrySignals(groups, registry)
	}

	// Flatten groups into sorted slice.
	// Sort signals within each group for deterministic YAML output.
	rels := make([]ManifestRelationship, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.signals, func(i, j int) bool {
			ki := g.signals[i].Type + "\x00" + g.signals[i].Evidence
			kj := g.signals[j].Type + "\x00" + g.signals[j].Evidence
			return ki < kj
		})
		rels = append(rels, ManifestRelationship{
			Source:     g.source,
			Target:     g.target,
			Type:       g.edgeType,
			Confidence: roundFloat(g.confidence, 4),
			Signals:    g.signals,
			Reviewed:   false,
			Status:     "pending",
		})
	}

	// Sort by confidence descending, then source+target for deterministic output.
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].Confidence != rels[j].Confidence {
			return rels[i].Confidence > rels[j].Confidence
		}
		if rels[i].Source != rels[j].Source {
			return rels[i].Source < rels[j].Source
		}
		return rels[i].Target < rels[j].Target
	})

	return &RelationshipManifest{
		Version:   "1.0",
		Generated: time.Now().UTC().Format(time.RFC3339),
		AlgorithmVersions: AlgorithmVersions{
			Cooccurrence: "1.0",
			Structural:   "1.0",
			Semantic:     "1.0",
			NER:          "1.0",
		},
		Relationships: rels,
	}
}

// signalTypeFromEdge maps an edge type to a human-readable signal type name.
func signalTypeFromEdge(e *Edge) string {
	switch e.Type {
	case EdgeReferences:
		return "structural"
	case EdgeMentions:
		return "mention"
	case EdgeCalls:
		return "structural"
	case EdgeDependsOn:
		return "structural"
	case EdgeImplements:
		return "structural"
	default:
		return "unknown"
	}
}

// mergeRegistrySignals adds registry-based signals to the group map.
func mergeRegistrySignals(groups map[string]*manifestGroup, registry *ComponentRegistry) {
	for _, rel := range registry.Relationships {
		fromComp := registry.GetComponent(rel.FromComponent)
		toComp := registry.GetComponent(rel.ToComponent)
		if fromComp == nil || toComp == nil {
			continue
		}

		source := fromComp.FileRef
		target := toComp.FileRef
		if source == "" || target == "" || source == target {
			continue
		}

		key := manifestKey(source, target)

		for _, sig := range rel.Signals {
			ms := ManifestSignal{
				Type:     string(sig.SourceType),
				Value:    sig.Confidence,
				Evidence: sig.Evidence,
			}

			g, ok := groups[key]
			if !ok {
				g = &manifestGroup{
					source:   source,
					target:   target,
					edgeType: "mentions",
				}
				groups[key] = g
			}
			g.signals = append(g.signals, ms)
			if sig.Confidence > g.confidence {
				g.confidence = sig.Confidence
			}
		}
	}
}

// SaveRelationshipManifest writes a manifest to a YAML file.
func SaveRelationshipManifest(manifest *RelationshipManifest, path string) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("SaveRelationshipManifest: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Clean(path), data, 0o644); err != nil {
		return fmt.Errorf("SaveRelationshipManifest: write %q: %w", path, err)
	}
	return nil
}

// LoadRelationshipManifest reads a manifest from a YAML file.
// Returns nil, nil when the file does not exist.
func LoadRelationshipManifest(path string) (*RelationshipManifest, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("LoadRelationshipManifest: read %q: %w", path, err)
	}
	var m RelationshipManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("LoadRelationshipManifest: parse %q: %w", path, err)
	}
	return &m, nil
}

// ManifestSummary holds summary statistics about a manifest.
type ManifestSummary struct {
	Total    int
	Reviewed int
	Accepted int
	Rejected int
	Pending  int
}

// Summarize computes summary statistics for a manifest.
func (m *RelationshipManifest) Summarize() ManifestSummary {
	s := ManifestSummary{Total: len(m.Relationships)}
	for _, r := range m.Relationships {
		switch r.Status {
		case "accepted":
			s.Reviewed++
			s.Accepted++
		case "rejected":
			s.Reviewed++
			s.Rejected++
		default:
			s.Pending++
		}
	}
	return s
}

// AcceptAll sets all relationships to accepted status.
func (m *RelationshipManifest) AcceptAll() {
	for i := range m.Relationships {
		m.Relationships[i].Reviewed = true
		m.Relationships[i].Status = "accepted"
	}
}

// RejectAll sets all relationships to rejected status.
func (m *RelationshipManifest) RejectAll() {
	for i := range m.Relationships {
		m.Relationships[i].Reviewed = true
		m.Relationships[i].Status = "rejected"
	}
}

// MergeUserEdits applies user review decisions from accepted manifest onto
// the discovered manifest.  For each relationship in discovered that has a
// matching (source, target) entry in accepted, the review fields are copied.
func (m *RelationshipManifest) MergeUserEdits(accepted *RelationshipManifest) {
	if accepted == nil {
		return
	}

	lookup := make(map[string]*ManifestRelationship, len(accepted.Relationships))
	for i := range accepted.Relationships {
		r := &accepted.Relationships[i]
		key := manifestKey(r.Source, r.Target)
		lookup[key] = r
	}

	for i := range m.Relationships {
		key := manifestKey(m.Relationships[i].Source, m.Relationships[i].Target)
		if prev, ok := lookup[key]; ok {
			m.Relationships[i].Reviewed = prev.Reviewed
			m.Relationships[i].Status = prev.Status
			m.Relationships[i].UserNotes = prev.UserNotes
		}
	}
}

// LoadAcceptedRelationships reads .bmd-relationships.yaml and returns edges
// for all relationships with status "accepted".
func LoadAcceptedRelationships(path string) ([]*Edge, error) {
	manifest, err := LoadRelationshipManifest(path)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, nil
	}

	var edges []*Edge
	for _, r := range manifest.Relationships {
		if r.Status != "accepted" {
			continue
		}

		edgeType := EdgeMentions // default
		switch r.Type {
		case string(EdgeReferences):
			edgeType = EdgeReferences
		case string(EdgeDependsOn):
			edgeType = EdgeDependsOn
		case string(EdgeCalls):
			edgeType = EdgeCalls
		case string(EdgeImplements):
			edgeType = EdgeImplements
		case string(EdgeMentions):
			edgeType = EdgeMentions
		}

		evidence := "accepted via bmd relationships-review"
		if r.UserNotes != "" {
			evidence = r.UserNotes
		}

		e, err := NewEdge(r.Source, r.Target, edgeType, r.Confidence, evidence)
		if err != nil {
			continue
		}
		edges = append(edges, e)
	}
	return edges, nil
}
