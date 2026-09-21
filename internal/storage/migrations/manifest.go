// Package migrations owns the embedded, versioned SQL schema for every core
// storage backend. Dialect adapters consume this package; they do not own DDL.
package migrations

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

// Dialect identifies one SQL dialect in the paired migration catalog.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]{3})_([a-z0-9]+(?:_[a-z0-9]+)*)\.sql$`)

// embeddedFiles is the release artifact's complete schema catalog. Runtime
// code never reads an external migration directory.
//
//go:embed sqlite/*.sql postgres/*.sql metadata/*.sql
var embeddedFiles embed.FS

// Migration is one immutable logical migration. SQL is kept as the exact
// embedded file text so its checksum detects any post-release drift.
type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
	Path     string
}

// Manifest is the validated pair of dialect-specific migration sequences.
type Manifest struct {
	byDialect map[Dialect][]Migration
}

// Embedded loads the catalog compiled into the executable.
func Embedded() (Manifest, error) { return Load(embeddedFiles) }

// Load reads and validates a migration catalog rooted at fsys.
func Load(fsys fs.FS) (Manifest, error) {
	manifest := Manifest{byDialect: make(map[Dialect][]Migration, 2)}
	for _, dialect := range []Dialect{SQLite, Postgres} {
		migrations, err := loadDialect(fsys, dialect)
		if err != nil {
			return Manifest{}, err
		}
		manifest.byDialect[dialect] = migrations
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// ValidateFS validates a migration catalog without exposing its parsed data.
func ValidateFS(fsys fs.FS) error {
	_, err := Load(fsys)
	return err
}

// Migrations returns a copy of the ordered migrations for dialect.
func (m Manifest) Migrations(dialect Dialect) []Migration {
	items := m.byDialect[dialect]
	return append([]Migration(nil), items...)
}

// Validate checks ordering and cross-dialect identity invariants on a loaded
// manifest. It is intentionally repeatable so callers can gate startup too.
func (m Manifest) Validate() error {
	if len(m.byDialect[SQLite]) == 0 {
		return fmt.Errorf("migration manifest: no sqlite migrations")
	}
	if len(m.byDialect[Postgres]) == 0 {
		return fmt.Errorf("migration manifest: no postgres migrations")
	}
	for _, dialect := range []Dialect{SQLite, Postgres} {
		items := m.byDialect[dialect]
		for i, item := range items {
			want := int64(i + 1)
			if item.Version != want {
				return fmt.Errorf("migration manifest: missing migration version %d before %s/%s", want, dialect, item.Path)
			}
		}
	}

	sqlite := m.byDialect[SQLite]
	postgres := m.byDialect[Postgres]
	if len(sqlite) != len(postgres) {
		if len(sqlite) > len(postgres) {
			missing := sqlite[len(postgres)]
			return fmt.Errorf("migration manifest: missing postgres migration %d (%s)", missing.Version, missing.Name)
		}
		missing := postgres[len(sqlite)]
		return fmt.Errorf("migration manifest: missing sqlite migration %d (%s)", missing.Version, missing.Name)
	}
	for i := range sqlite {
		if sqlite[i].Version != postgres[i].Version {
			return fmt.Errorf("migration manifest: dialect version mismatch at index %d: sqlite=%d postgres=%d", i, sqlite[i].Version, postgres[i].Version)
		}
		if sqlite[i].Name != postgres[i].Name {
			return fmt.Errorf("migration manifest: migration %d name mismatch: sqlite=%q postgres=%q", sqlite[i].Version, sqlite[i].Name, postgres[i].Name)
		}
	}
	return nil
}

func loadDialect(fsys fs.FS, dialect Dialect) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, string(dialect))
	if err != nil {
		return nil, fmt.Errorf("migration manifest: read %s directory: %w", dialect, err)
	}
	items := make([]Migration, 0, len(entries))
	seen := make(map[int64]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("migration manifest: invalid migration filename %s/%s", dialect, entry.Name())
		}
		match := migrationFilenamePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("migration manifest: invalid migration filename %s/%s", dialect, entry.Name())
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration manifest: parse version %s/%s: %w", dialect, entry.Name(), err)
		}
		if prior, ok := seen[version]; ok {
			return nil, fmt.Errorf("migration manifest: duplicate migration version %d in %s (%s and %s)", version, dialect, prior, entry.Name())
		}
		path := string(dialect) + "/" + entry.Name()
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("migration manifest: read %s: %w", path, err)
		}
		digest := sha256.Sum256(data)
		seen[version] = entry.Name()
		items = append(items, Migration{
			Version:  version,
			Name:     match[2],
			SQL:      string(data),
			Checksum: hex.EncodeToString(digest[:]),
			Path:     path,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Version < items[j].Version })
	return items, nil
}
