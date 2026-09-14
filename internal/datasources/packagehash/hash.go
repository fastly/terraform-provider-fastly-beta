package packagehash

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"fmt"
	"io"
	"os"
	"sort"
)

// maxPackageSize matches the Compute package size limit:
// https://developer.fastly.com/learning/compute/#limitations-and-constraints
const maxPackageSize int64 = 100_000_000

// hashPackage returns a SHA512 hash of all files (in sorted order) within the
// gzipped tar archive at path.
func hashPackage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open package %q: %w", path, err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to create a gzip reader: %w", err)
	}
	defer zr.Close()

	files, err := readPackageFiles(tar.NewReader(zr), maxPackageSize)
	if err != nil {
		return "", fmt.Errorf("failed to read files within the package: %w", err)
	}

	if err := zr.Close(); err != nil {
		return "", fmt.Errorf("failed to finish reading gzip package: %w", err)
	}

	return hashFiles(files), nil
}

func readPackageFiles(tr *tar.Reader, maxSize int64) (map[string]*bytes.Buffer, error) {
	contents := make(map[string]*bytes.Buffer)

	var pkgSize int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Avoids a decompression-bomb DoS: track the uncompressed size as we go
		// rather than trusting the archive up front.
		pkgSize += hdr.Size
		if pkgSize > maxSize {
			return nil, fmt.Errorf("package size exceeded %d byte limit", maxSize)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if _, exists := contents[hdr.Name]; exists {
			return nil, fmt.Errorf("package contains duplicate file %q", hdr.Name)
		}

		contents[hdr.Name] = &bytes.Buffer{}
		if _, err := io.CopyN(contents[hdr.Name], tr, hdr.Size); err != nil {
			return nil, err
		}
	}

	return contents, nil
}

// hashFiles returns a SHA512 hash of contents, read in sorted filename order so
// the result doesn't depend on the archive's internal file ordering.
func hashFiles(contents map[string]*bytes.Buffer) string {
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha512.New()
	for _, name := range names {
		h.Write(contents[name].Bytes())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
