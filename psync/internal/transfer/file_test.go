package transfer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/psync/psync/internal/transfer"
)

func TestChecksumFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("hello psync"), 0o644); err != nil {
		t.Fatal(err)
	}

	sum, err := transfer.ChecksumFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sum) != 64 {
		t.Errorf("expected 64-char hex digest, got %d chars", len(sum))
	}

	// Same content → same checksum.
	f2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(f2, []byte("hello psync"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum2, _ := transfer.ChecksumFile(f2)
	if sum != sum2 {
		t.Errorf("identical content produced different checksums: %s vs %s", sum, sum2)
	}

	// Different content → different checksum.
	f3 := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(f3, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum3, _ := transfer.ChecksumFile(f3)
	if sum == sum3 {
		t.Error("different content produced same checksum")
	}
}

func TestFilesIdentical(t *testing.T) {
	dir := t.TempDir()
	content := []byte("test content")

	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// dst doesn't exist yet.
	ok, err := transfer.FilesIdentical(src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected not identical when dst missing")
	}

	// dst has same content.
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = transfer.FilesIdentical(src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected identical files to be detected")
	}

	// dst has different content.
	if err := os.WriteFile(dst, []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = transfer.FilesIdentical(src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected different files to be detected")
	}
}
