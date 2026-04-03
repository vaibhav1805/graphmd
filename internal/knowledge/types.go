package knowledge

import "strings"

// ComponentType represents the classification of an infrastructure component.
// The taxonomy is based on common infrastructure patterns from Backstage and
// Cartography, covering the most common categories found in service-oriented
// architectures.
type ComponentType string

const (
	// ComponentTypeService represents an application service (e.g. payment-api,
	// user-service, auth-service).
	ComponentTypeService ComponentType = "service"

	// ComponentTypeDatabase represents a data store (e.g. postgres, mysql,
	// mongodb, dynamodb).
	ComponentTypeDatabase ComponentType = "database"

	// ComponentTypeCache represents an in-memory caching layer (e.g. redis,
	// memcached, varnish).
	ComponentTypeCache ComponentType = "cache"

	// ComponentTypeQueue represents a message queue (e.g. sqs, celery, sidekiq).
	ComponentTypeQueue ComponentType = "queue"

	// ComponentTypeMessageBroker represents a message broker / event streaming
	// platform (e.g. kafka, rabbitmq, nats, pulsar).
	ComponentTypeMessageBroker ComponentType = "message-broker"

	// ComponentTypeLoadBalancer represents a load balancer or reverse proxy
	// (e.g. nginx, haproxy, envoy, aws alb).
	ComponentTypeLoadBalancer ComponentType = "load-balancer"

	// ComponentTypeGateway represents an API gateway (e.g. kong, aws api-gateway,
	// traefik, ambassador).
	ComponentTypeGateway ComponentType = "gateway"

	// ComponentTypeStorage represents object or file storage (e.g. s3, gcs,
	// minio, azure blob).
	ComponentTypeStorage ComponentType = "storage"

	// ComponentTypeContainerRegistry represents a container image registry
	// (e.g. ecr, gcr, docker-hub, harbor).
	ComponentTypeContainerRegistry ComponentType = "container-registry"

	// ComponentTypeConfigServer represents a configuration management service
	// (e.g. consul, etcd, spring-cloud-config, vault).
	ComponentTypeConfigServer ComponentType = "config-server"

	// ComponentTypeMonitoring represents an observability / monitoring platform
	// (e.g. prometheus, datadog, grafana, new-relic).
	ComponentTypeMonitoring ComponentType = "monitoring"

	// ComponentTypeLogAggregator represents a log collection and search platform
	// (e.g. elasticsearch, splunk, loki, fluentd).
	ComponentTypeLogAggregator ComponentType = "log-aggregator"

	// ComponentTypeUnknown is the default type assigned when automatic detection
	// cannot determine a more specific classification.
	ComponentTypeUnknown ComponentType = "unknown"
)

// allComponentTypes is the canonical list of valid component types in display
// order.  Used by AllComponentTypes() and IsValidComponentType().
var allComponentTypes = []ComponentType{
	ComponentTypeService,
	ComponentTypeDatabase,
	ComponentTypeCache,
	ComponentTypeQueue,
	ComponentTypeMessageBroker,
	ComponentTypeLoadBalancer,
	ComponentTypeGateway,
	ComponentTypeStorage,
	ComponentTypeContainerRegistry,
	ComponentTypeConfigServer,
	ComponentTypeMonitoring,
	ComponentTypeLogAggregator,
	ComponentTypeUnknown,
}

// validComponentTypes is a set for O(1) membership testing.
var validComponentTypes map[ComponentType]bool

func init() {
	validComponentTypes = make(map[ComponentType]bool, len(allComponentTypes))
	for _, t := range allComponentTypes {
		validComponentTypes[t] = true
	}
}

// AllComponentTypes returns the 12 taxonomy types plus the "unknown" default,
// in canonical display order.
func AllComponentTypes() []ComponentType {
	out := make([]ComponentType, len(allComponentTypes))
	copy(out, allComponentTypes)
	return out
}

// IsValidComponentType returns true when t is one of the 12 taxonomy types or
// "unknown".
func IsValidComponentType(t ComponentType) bool {
	return validComponentTypes[t]
}

// ComponentTypeDescription returns a short human-readable description for each
// component type, suitable for documentation generation and CLI help text.
func ComponentTypeDescription(t ComponentType) string {
	switch t {
	case ComponentTypeService:
		return "Application service (API, backend, worker)"
	case ComponentTypeDatabase:
		return "Data store (relational, document, key-value)"
	case ComponentTypeCache:
		return "In-memory caching layer"
	case ComponentTypeQueue:
		return "Message queue for async task processing"
	case ComponentTypeMessageBroker:
		return "Message broker / event streaming platform"
	case ComponentTypeLoadBalancer:
		return "Load balancer or reverse proxy"
	case ComponentTypeGateway:
		return "API gateway"
	case ComponentTypeStorage:
		return "Object or file storage"
	case ComponentTypeContainerRegistry:
		return "Container image registry"
	case ComponentTypeConfigServer:
		return "Configuration management service"
	case ComponentTypeMonitoring:
		return "Observability and monitoring platform"
	case ComponentTypeLogAggregator:
		return "Log collection and search platform"
	case ComponentTypeUnknown:
		return "Unclassified component (default)"
	default:
		return "Unknown type"
	}
}

// componentTypePatterns maps keyword patterns (lowercase) to component types.
// Used by InferComponentType to classify components from their names and
// surrounding context.
var componentTypePatterns = map[ComponentType][]string{
	ComponentTypeService: {
		"service", "api", "server", "worker", "backend", "microservice",
		"app", "application",
	},
	ComponentTypeDatabase: {
		"database", "db", "postgres", "postgresql", "mysql", "mariadb",
		"mongodb", "mongo", "dynamodb", "cockroachdb", "cassandra",
		"couchdb", "sqlite", "rds", "aurora", "datastore", "store",
	},
	ComponentTypeCache: {
		"cache", "redis", "memcached", "memcache", "varnish", "cdn",
		"elasticache",
	},
	ComponentTypeQueue: {
		"queue", "sqs", "celery", "sidekiq", "delayed-job", "bull",
		"beanstalkd",
	},
	ComponentTypeMessageBroker: {
		"kafka", "rabbitmq", "rabbit", "nats", "pulsar", "kinesis",
		"eventbridge", "event-bus", "message-broker", "broker",
		"event-stream",
	},
	ComponentTypeLoadBalancer: {
		"load-balancer", "loadbalancer", "lb", "haproxy", "envoy",
		"alb", "elb", "nlb",
	},
	ComponentTypeGateway: {
		"gateway", "api-gateway", "kong", "traefik", "ambassador",
		"ingress",
	},
	ComponentTypeStorage: {
		"storage", "s3", "gcs", "minio", "blob", "bucket", "object-store",
		"file-store",
	},
	ComponentTypeContainerRegistry: {
		"registry", "ecr", "gcr", "docker-hub", "harbor",
		"container-registry",
	},
	ComponentTypeConfigServer: {
		"config", "consul", "etcd", "vault", "config-server",
		"spring-cloud-config", "zookeeper",
	},
	ComponentTypeMonitoring: {
		"monitoring", "prometheus", "datadog", "grafana", "new-relic",
		"newrelic", "nagios", "pagerduty", "alertmanager", "observability",
	},
	ComponentTypeLogAggregator: {
		"log", "logging", "elasticsearch", "elk", "splunk", "loki",
		"fluentd", "logstash", "kibana", "log-aggregator",
	},
}

// InferComponentType attempts to classify a component based on its name and
// optional context strings.  Returns the best-matching ComponentType and a
// confidence score in [0.4, 1.0].
//
// Matching priority:
//  1. Exact type name match (e.g. name == "database") -> 0.95
//  2. Pattern substring match in name -> 0.85
//  3. Pattern substring match in context -> 0.65
//  4. No match -> (ComponentTypeUnknown, 0.5)
func InferComponentType(name string, context ...string) (ComponentType, float64) {
	lowerName := strings.ToLower(name)

	// Priority 1: exact match against type name.
	for _, ct := range allComponentTypes {
		if ct == ComponentTypeUnknown {
			continue
		}
		if lowerName == string(ct) {
			return ct, 0.95
		}
	}

	// Priority 2: pattern match in name.
	// Track best match by specificity (longer pattern = more specific).
	bestType := ComponentTypeUnknown
	bestLen := 0
	for ct, patterns := range componentTypePatterns {
		for _, p := range patterns {
			if strings.Contains(lowerName, p) && len(p) > bestLen {
				bestType = ct
				bestLen = len(p)
			}
		}
	}
	if bestType != ComponentTypeUnknown {
		return bestType, 0.85
	}

	// Priority 3: pattern match in context strings.
	for _, ctx := range context {
		lowerCtx := strings.ToLower(ctx)
		for ct, patterns := range componentTypePatterns {
			for _, p := range patterns {
				if strings.Contains(lowerCtx, p) && len(p) > bestLen {
					bestType = ct
					bestLen = len(p)
				}
			}
		}
	}
	if bestType != ComponentTypeUnknown {
		return bestType, 0.65
	}

	return ComponentTypeUnknown, 0.5
}

// SeedConfig represents user-supplied type mappings that override automatic
// detection.  Loaded from a YAML configuration file.
type SeedConfig struct {
	// TypeMappings maps component name patterns to component types.
	// Pattern matching is case-insensitive substring.
	// Example: {"redis*": "cache", "postgres*": "database"}
	TypeMappings []SeedMapping `yaml:"type_mappings"`
}

// SeedMapping is a single pattern-to-type mapping in the seed configuration.
type SeedMapping struct {
	// Pattern is a case-insensitive substring or glob pattern to match
	// against component names.
	Pattern string `yaml:"pattern"`

	// Type is the component type to assign when the pattern matches.
	Type ComponentType `yaml:"type"`
}

// ApplySeedConfig checks name against seed config mappings and returns the
// matching type and confidence.  Seed config matches have highest priority
// (confidence 1.0).  Returns ("", 0) when no mapping matches.
func (sc *SeedConfig) ApplySeedConfig(name string) (ComponentType, float64) {
	if sc == nil {
		return "", 0
	}
	lowerName := strings.ToLower(name)
	for _, m := range sc.TypeMappings {
		pattern := strings.ToLower(m.Pattern)
		// Support simple glob: "redis*" matches "redis-cache", "redis-cluster", etc.
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(lowerName, prefix) {
				return m.Type, 1.0
			}
		} else if lowerName == pattern || strings.Contains(lowerName, pattern) {
			return m.Type, 1.0
		}
	}
	return "", 0
}
