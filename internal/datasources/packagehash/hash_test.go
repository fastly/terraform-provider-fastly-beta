package packagehash

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestPackage builds a gzipped tar archive from files (name -> content) and
// writes it to a temp file, returning its path.
func writeTestPackage(t *testing.T, files map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	path := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func TestHashPackage(t *testing.T) {
	path := writeTestPackage(t, map[string]string{
		"main.wasm": "binary-content",
		"README.md": "docs",
	})

	got, err := hashPackage(path)
	require.NoError(t, err)

	want := fmt.Sprintf("%x", sha512.Sum512([]byte("docsbinary-content")))
	assert.Equal(t, want, got)
}

func TestHashPackage_orderIndependent(t *testing.T) {
	pathA := writeTestPackage(t, map[string]string{"a.txt": "1", "b.txt": "2"})
	pathB := writeTestPackage(t, map[string]string{"b.txt": "2", "a.txt": "1"})

	hashA, err := hashPackage(pathA)
	require.NoError(t, err)
	hashB, err := hashPackage(pathB)
	require.NoError(t, err)

	assert.Equal(t, hashA, hashB)
}

func TestHashPackage_nonRegularFilesSkipped(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "adir",
		Typeflag: tar.TypeDir,
	}))
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "file.txt",
		Mode: 0o600,
		Size: int64(len("content")),
	}))
	_, err := tw.Write([]byte("content"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())

	path := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	got, err := hashPackage(path)
	require.NoError(t, err)

	want := fmt.Sprintf("%x", sha512.Sum512([]byte("content")))
	assert.Equal(t, want, got)
}

func TestReadPackageFiles_sizeLimitExceeded(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "file.bin",
		Mode: 0o600,
		Size: int64(len("more than ten bytes")),
	}))
	_, err := tw.Write([]byte("more than ten bytes"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	_, err = readPackageFiles(tar.NewReader(&buf), 10)
	assert.ErrorContains(t, err, "100MB limit")
}

func TestHashPackage_invalidGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(path, []byte("not a gzip file"), 0o600))

	_, err := hashPackage(path)
	assert.ErrorContains(t, err, "gzip reader")
}

func TestHashPackage_missingFile(t *testing.T) {
	_, err := hashPackage(filepath.Join(t.TempDir(), "does-not-exist.tar.gz"))
	assert.ErrorContains(t, err, "failed to open package")
}
