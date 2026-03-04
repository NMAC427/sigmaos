package pyenv

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	kpflate "github.com/klauspost/compress/flate"
)

// safeJoin prevents zip-slip (e.g. "../../etc/passwd").
func safeJoin(base, rel string) (string, error) {
	rel = filepath.Clean(rel)
	if rel == "." || rel == string(filepath.Separator) {
		return "", errors.New("invalid path")
	}
	// zip paths are '/' separated; filepath.Clean converts to OS separators.
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("zip-slip path: %q", rel)
	}
	p := filepath.Join(base, rel)
	return p, nil
}

func wheelEntryToSitePackagesPath(name string) (string, bool) {
	// ZIP uses forward slashes.
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return "", false
	}

	// If in *.data/(purelib|platlib)/..., map into site-packages stripping up to that.
	for i := 0; i+1 < len(parts); i++ {
		if strings.HasSuffix(parts[i], ".data") && i+2 < len(parts) {
			kind := parts[i+1]
			if kind == "purelib" || kind == "platlib" {
				rest := strings.Join(parts[i+2:], "/")
				if rest == "" || strings.HasSuffix(name, "/") {
					return "", false
				}
				return rest, true
			}
			// Other .data kinds (scripts, data, headers, etc.) ignored for this use case.
			return "", false
		}
	}

	// Not in *.data at all: install as-is into site-packages.
	// Ignore directories.
	if strings.HasSuffix(name, "/") {
		return "", false
	}
	return name, true
}

func ExtractWheelToSitePackages(wheelPath, targetDir string) error {
	sitePkgs := filepath.Join(targetDir, "site-packages")
	if err := os.MkdirAll(sitePkgs, 0o755); err != nil {
		return err
	}

	r, err := zip.OpenReader(wheelPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Speed: use klauspost flate for DEFLATE entries (most wheel content).
	r.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return io.NopCloser(kpflate.NewReader(r))
	})

	buf := make([]byte, 1024*128)

	for _, f := range r.File {
		rel, ok := wheelEntryToSitePackagesPath(f.Name)
		if !ok {
			continue
		}

		dstPath, err := safeJoin(sitePkgs, rel)
		if err != nil {
			return err
		}

		// Create parent dirs.
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		// Write file (atomic-ish: write temp then rename).
		tmp := dstPath + ".tmp"
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.CopyBuffer(out, rc, buf)
		closeErr := out.Close()
		rc.Close()

		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}

		// Preserve executable bit if present in zip metadata (rarely matters for site-packages).
		if mode := f.Mode(); mode != 0 {
			_ = os.Chmod(tmp, mode)
		}

		if err := os.Rename(tmp, dstPath); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}

	return nil
}
