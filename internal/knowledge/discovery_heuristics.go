package knowledge

// DefaultPackageMarkers is the default list of file names used to identify
// component boundaries in a monorepo via the package-marker discovery method.
var DefaultPackageMarkers = []string{
	"go.mod",
	"package.json",
	"pom.xml",
	"Cargo.toml",
	"pyproject.toml",
	"composer.json",
}

// DefaultConventionalDirs is the default list of conventional monorepo
// directory patterns used when no package markers are found.
// Patterns ending in "/*" match all immediate subdirectories of the named
// parent; exact names match a single component directory.
var DefaultConventionalDirs = []string{
	"services/*",
	"packages/*",
	"apps/*",
	"libs/*",
	"cmd/*",
	"pkg/*",
}

// DefaultMaxDepth is the maximum directory depth used by the depth-based
// fallback discovery method.  Depth 1 means only top-level subdirectories of
// the scan root are considered; depth 2 also includes their children; etc.
const DefaultMaxDepth = 3
