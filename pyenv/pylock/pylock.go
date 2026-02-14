// pylock.toml specification:
// https://packaging.python.org/en/latest/specifications/pylock-toml/

package pylock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Top-level pylock structure
type Pylock struct {
	LockVersion      string         `toml:"lock-version"`
	Environments     []string       `toml:"environments"`
	RequiresPython   string         `toml:"requires-python"`
	Extras           []string       `toml:"extras"`
	DependencyGroups []string       `toml:"dependency-groups"`
	DefaultGroups    []string       `toml:"default-groups"`
	CreatedBy        string         `toml:"created-by"`
	Packages         []Package      `toml:"packages"`
	Tool             map[string]any `toml:"tool"`
}

// Package entry (one element per [[packages]])
type Package struct {
	Name           string     `toml:"name"`
	Version        string     `toml:"version"`
	Marker         string     `toml:"marker"`
	RequiresPython string     `toml:"requires-python"`
	VCS            *VCS       `toml:"vcs"`
	Directory      *Directory `toml:"directory"`
	Archive        *Archive   `toml:"archive"`
	Sdist          *Sdist     `toml:"sdist"`
	Wheels         []Wheel    `toml:"wheels"`
}

type PackageRef struct {
	Name      string     `toml:"name"`
	Version   string     `toml:"version"`
	VCS       *VCS       `toml:"vcs"`
	Directory *Directory `toml:"directory"`
	Archive   *Archive   `toml:"archive"`
}

type VCS struct {
	Type              string `toml:"type"`
	URL               string `toml:"url"`
	Path              string `toml:"path"`
	RequestedRevision string `toml:"requested-revision"`
	CommitID          string `toml:"commit-id"`
	Subdirectory      string `toml:"subdirectory"`
}

type Directory struct {
	Path         string `toml:"path"`
	Editable     bool   `toml:"editable"`
	Subdirectory string `toml:"subdirectory"`
}

type Archive struct {
	URL          string            `toml:"url"`
	Path         string            `toml:"path"`
	Size         *int64            `toml:"size"`
	UploadTime   *time.Time        `toml:"upload-time"`
	Hashes       map[string]string `toml:"hashes"`
	Subdirectory string            `toml:"subdirectory"`
}

type Sdist struct {
	Name       string            `toml:"name"`
	URL        string            `toml:"url"`
	Path       string            `toml:"path"`
	Size       *int64            `toml:"size"`
	UploadTime *time.Time        `toml:"upload-time"`
	Hashes     map[string]string `toml:"hashes"`
}

type Wheel struct {
	Name       string            `toml:"name"`
	URL        string            `toml:"url"`
	Path       string            `toml:"path"`
	Size       *int64            `toml:"size"`
	UploadTime *time.Time        `toml:"upload-time"`
	Hashes     map[string]string `toml:"hashes"`
}

func ParsePylock(path string) (*Pylock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var p Pylock
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("toml decode error: %w", err)
	}

	// Basic validation
	if p.LockVersion == "" {
		return nil, errors.New(`missing required key "lock-version"`)
	}
	if p.LockVersion != "1.0" {
		// spec: only "1.0" valid for initial version
		return nil, fmt.Errorf(`unsupported lock-version %q (only "1.0" supported)`, p.LockVersion)
	}

	if strings.TrimSpace(p.CreatedBy) == "" {
		return nil, errors.New(`missing required key "created-by"`)
	}

	if len(p.Packages) == 0 {
		return nil, errors.New(`missing required array of tables [[packages]]`)
	}

	// Validate packages content: mutually exclusive sources, required hashes etc.
	for i := range p.Packages {
		pkg := &p.Packages[i]
		if strings.TrimSpace(pkg.Name) == "" {
			return nil, fmt.Errorf("package entry %d missing required field: name", i)
		}
		// Count source kinds set
		sourceCount := 0
		if pkg.VCS != nil {
			sourceCount++
			// VCS requires commit-id
			if strings.TrimSpace(pkg.VCS.CommitID) == "" {
				return nil, fmt.Errorf("packages[%d] (name=%s): vcs.commit-id is required when vcs is used", i, pkg.Name)
			}
			if strings.TrimSpace(pkg.VCS.Type) == "" {
				return nil, fmt.Errorf("packages[%d] (name=%s): vcs.type is required when vcs is used", i, pkg.Name)
			}
		}
		if pkg.Directory != nil {
			sourceCount++
			if strings.TrimSpace(pkg.Directory.Path) == "" {
				return nil, fmt.Errorf("packages[%d] (name=%s): directory.path required", i, pkg.Name)
			}
		}
		if pkg.Archive != nil {
			sourceCount++
			if len(pkg.Archive.Hashes) == 0 {
				return nil, fmt.Errorf("packages[%d] (name=%s): archive.hashes must contain at least one entry", i, pkg.Name)
			}
		}
		if pkg.Sdist != nil {
			sourceCount++
			if len(pkg.Sdist.Hashes) == 0 {
				return nil, fmt.Errorf("packages[%d] (name=%s): sdist.hashes must contain at least one entry", i, pkg.Name)
			}
		}
		if len(pkg.Wheels) > 0 {
			sourceCount++
			for j := range pkg.Wheels {
				w := &pkg.Wheels[j]
				if len(w.Hashes) == 0 {
					return nil, fmt.Errorf("packages[%d].wheels[%d] (name=%s): wheels.hashes must contain at least one entry", i, j, pkg.Name)
				}
			}
		}

		if sourceCount == 0 {
			return nil, fmt.Errorf("packages[%d] (name=%s): one of vcs/directory/archive/sdist/wheels must be present", i, pkg.Name)
		}

		// Specific mutual-exclusion checks:
		if pkg.VCS != nil && (pkg.Directory != nil || pkg.Archive != nil || pkg.Sdist != nil || len(pkg.Wheels) > 0) {
			return nil, fmt.Errorf("packages[%d] (name=%s): vcs is mutually exclusive with directory/archive/sdist/wheels", i, pkg.Name)
		}
		if pkg.Directory != nil && (pkg.VCS != nil || pkg.Archive != nil || pkg.Sdist != nil || len(pkg.Wheels) > 0) {
			return nil, fmt.Errorf("packages[%d] (name=%s): directory is mutually exclusive with vcs/archive/sdist/wheels", i, pkg.Name)
		}
		if pkg.Archive != nil && (pkg.VCS != nil || pkg.Directory != nil) {
			return nil, fmt.Errorf("packages[%d] (name=%s): archive is mutually exclusive with vcs/directory", i, pkg.Name)
		}
		if pkg.Sdist != nil && (pkg.VCS != nil || pkg.Directory != nil) {
			return nil, fmt.Errorf("packages[%d] (name=%s): sdist is mutually exclusive with vcs/directory", i, pkg.Name)
		}
		if len(pkg.Wheels) > 0 && (pkg.VCS != nil || pkg.Directory != nil) {
			return nil, fmt.Errorf("packages[%d] (name=%s): wheels are mutually exclusive with vcs/directory", i, pkg.Name)
		}
	}

	// Compute names for sdist/wheels with no names
	for i := range p.Packages {
		pkg := &p.Packages[i]
		if pkg.Sdist != nil && pkg.Sdist.Name == "" {
			sdist := pkg.Sdist
			if sdist.URL != "" {
				sdist.Name = filepath.Base(sdist.URL)
			} else if sdist.Path != "" {
				sdist.Name = filepath.Base(sdist.Path)
			}
		}

		for j := range pkg.Wheels {
			w := &pkg.Wheels[j]
			if w.Name == "" {
				if w.URL != "" {
					w.Name = filepath.Base(w.URL)
				} else if w.Path != "" {
					w.Name = filepath.Base(w.Path)
				}
			}
		}
	}

	return &p, nil
}
