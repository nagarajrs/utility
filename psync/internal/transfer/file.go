package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// ChecksumFile computes the SHA-256 hex digest of the file at path.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FilesIdentical returns true when both files exist, have the same size,
// and the same SHA-256 checksum.
func FilesIdentical(src, dst string) (bool, error) {
	si, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	di, err := os.Stat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if si.Size() != di.Size() {
		return false, nil
	}
	srcSum, err := ChecksumFile(src)
	if err != nil {
		return false, err
	}
	dstSum, err := ChecksumFile(dst)
	if err != nil {
		return false, err
	}
	return srcSum == dstSum, nil
}

// CopyFile copies src to dst, preserving permissions and timestamps.
func CopyFile(src, dst string, dryRun bool) error {
	if dryRun {
		return nil
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Ensure destination directory exists.
	if err := os.MkdirAll(parentDir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}

	// Preserve timestamps.
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
