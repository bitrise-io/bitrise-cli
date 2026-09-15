package rde

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// readTarHeaders gunzips + walks an archive and returns every header.
func readTarHeaders(t *testing.T, archive []byte) []*tar.Header {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	tr := tar.NewReader(gr)
	var headers []*tar.Header
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		headers = append(headers, h)
	}
	return headers
}

// TestCreateTarGz_StripsLocalOwnership pins that the archive never carries the
// local uid/gid/user/group: a root-side extraction on the VM would otherwise
// recreate the caller's numeric uid there (seen as files owned by 501:root).
func TestCreateTarGz_StripsLocalOwnership(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, src := range map[string]string{
		"directory":   dir,
		"single file": filepath.Join(dir, "a.txt"),
	} {
		t.Run(name, func(t *testing.T) {
			archive, err := createTarGz(src)
			if err != nil {
				t.Fatalf("createTarGz: %v", err)
			}
			headers := readTarHeaders(t, archive)
			if len(headers) == 0 {
				t.Fatal("archive is empty")
			}
			for _, h := range headers {
				if h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" {
					t.Errorf("%s: owner leaked into archive: uid=%d gid=%d uname=%q gname=%q", h.Name, h.Uid, h.Gid, h.Uname, h.Gname)
				}
			}
		})
	}
}

// TestCreateTarGz_SingleFileUsesBasename pins that a single file is archived
// under its basename (so it lands as REMOTE_FOLDER/<basename> on the VM) with
// its content intact.
func TestCreateTarGz_SingleFileUsesBasename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.xml")
	if err := os.WriteFile(path, []byte("<m/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive, err := createTarGz(path)
	if err != nil {
		t.Fatal(err)
	}
	headers := readTarHeaders(t, archive)
	if len(headers) != 1 || headers[0].Name != "manifest.xml" {
		t.Fatalf("headers = %+v, want exactly one entry named manifest.xml", headers)
	}
}
