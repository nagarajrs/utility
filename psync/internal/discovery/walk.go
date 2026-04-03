package discovery

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// FileEntry is a discovered file ready to be synced.
type FileEntry struct {
	RelPath  string      // relative to source root
	AbsPath  string      // absolute path on the source
	Size     int64
	ModTime  time.Time
	FileMode fs.FileMode
}

// Options controls which files are included in a walk.
type Options struct {
	Include   []string  // glob patterns; empty = include all
	Exclude   []string  // glob patterns; match = skip
	MinSize   int64     // 0 = no minimum
	MaxSize   int64     // 0 = no maximum
	NewerThan time.Time // zero = no filter
}

// Walk recursively walks root and returns all files that pass the filters.
func Walk(root string, opts Options) ([]FileEntry, error) {
	var entries []FileEntry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Normalise to forward slashes for cross-platform checkpoint keys.
		rel = filepath.ToSlash(rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if !passesFilters(rel, info, opts) {
			return nil
		}

		entries = append(entries, FileEntry{
			RelPath:  rel,
			AbsPath:  path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			FileMode: info.Mode(),
		})
		return nil
	})
	return entries, err
}

// passesFilters returns true if the file should be included in the sync.
func passesFilters(rel string, info fs.FileInfo, opts Options) bool {
	base := filepath.Base(rel)

	// Exclude patterns take priority.
	for _, pat := range opts.Exclude {
		if matchGlob(pat, base) || matchGlob(pat, rel) {
			return false
		}
	}

	// Include patterns: if any are specified the file must match at least one.
	if len(opts.Include) > 0 {
		matched := false
		for _, pat := range opts.Include {
			if matchGlob(pat, base) || matchGlob(pat, rel) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Size filters.
	if opts.MinSize > 0 && info.Size() < opts.MinSize {
		return false
	}
	if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
		return false
	}

	// Time filter.
	if !opts.NewerThan.IsZero() && !info.ModTime().After(opts.NewerThan) {
		return false
	}

	return true
}

// matchGlob is a thin wrapper around filepath.Match that also handles
// path separators so patterns like ".git" match ".git/config" rel paths.
func matchGlob(pattern, name string) bool {
	// Try direct match first.
	ok, _ := filepath.Match(pattern, name)
	if ok {
		return true
	}
	// Try matching against each path component.
	for _, part := range strings.Split(name, "/") {
		if ok, _ = filepath.Match(pattern, part); ok {
			return true
		}
	}
	return false
}
