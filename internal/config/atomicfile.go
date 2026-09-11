package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	FilePerm = 0o600
	DirPerm  = 0o700
)

// EnsureDir creates dir (and any missing parents) if needed and guarantees
// perm on the result even when dir already existed with a looser mode --
// [os.MkdirAll] only applies perm when it creates a new directory, leaving a
// pre-existing directory's mode untouched.
func EnsureDir(dir string, perm os.FileMode) error {
	if mkErr := os.MkdirAll(dir, perm); mkErr != nil {
		return fmt.Errorf("create dir: %w", mkErr)
	}
	if chmodErr := os.Chmod(dir, perm); chmodErr != nil {
		return fmt.Errorf("chmod dir: %w", chmodErr)
	}
	return nil
}

// WriteFileAtomic writes data to path with perm via a temp-file-then-rename
// swap in the same directory, so a concurrent reader never observes a
// truncated or partially written file, and perm is re-applied even when
// path already exists -- [os.WriteFile] only applies perm when it creates
// the file, leaving a pre-existing file's mode untouched on rewrite.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, createErr := os.CreateTemp(dir, ".tmp-*")
	if createErr != nil {
		return fmt.Errorf("create temp file: %w", createErr)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, writeErr := tmp.Write(data); writeErr != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", writeErr)
	}
	if chmodErr := tmp.Chmod(perm); chmodErr != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", chmodErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return fmt.Errorf("close temp file: %w", closeErr)
	}
	if renameErr := os.Rename(tmpPath, path); renameErr != nil {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}
	return nil
}
