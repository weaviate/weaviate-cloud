package config_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/config"
)

func TestWriteFileAtomicSetsPerm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		preexisting bool
		preMode     os.FileMode
	}{
		{name: "new file"},
		{name: "pre-existing looser mode retightened", preexisting: true, preMode: 0o644},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertWriteFileAtomicSetsPerm(t, tc.preexisting, tc.preMode)
		})
	}
}

func assertWriteFileAtomicSetsPerm(t *testing.T, preexisting bool, preMode os.FileMode) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")
	if preexisting {
		if err := os.WriteFile(path, []byte("old"), preMode); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if err := config.WriteFileAtomic(path, []byte("new"), config.FilePerm); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != config.FilePerm {
		t.Fatalf("perm = %o, want %o", info.Mode().Perm(), config.FilePerm)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}
}

func TestWriteFileAtomicNoTempFileLeftBehind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")

	if err := config.WriteFileAtomic(path, []byte("hello"), config.FilePerm); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("dir entries = %v, want exactly [%s]", entries, filepath.Base(path))
	}
}

//nolint:paralleltest // timing-sensitive concurrency probe; must not race scheduling against unrelated parallel tests
func TestWriteFileAtomicConcurrentReadersNeverObservePartialWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")

	const payloadSize = 5 * 1024 * 1024
	oldData := bytes.Repeat([]byte("a"), payloadSize)
	newData := bytes.Repeat([]byte("b"), payloadSize)

	if err := config.WriteFileAtomic(path, oldData, config.FilePerm); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	var readErr error
	var badRead bool
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			buf, err := os.ReadFile(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					readErr = err
					return
				}
				continue
			}
			if len(buf) != len(oldData) && len(buf) != len(newData) {
				badRead = true
				return
			}
		}
	}()

	writeErr := config.WriteFileAtomic(path, newData, config.FilePerm)
	close(stop)
	<-done

	if writeErr != nil {
		t.Fatalf("write under test: %v", writeErr)
	}
	if readErr != nil {
		t.Fatalf("concurrent reader error: %v", readErr)
	}
	if badRead {
		t.Fatal("concurrent reader observed a partial/truncated file -- write is not atomic")
	}
}

func TestEnsureDirSetsPerm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		preexisting bool
		preMode     os.FileMode
	}{
		{name: "new dir"},
		{name: "pre-existing looser mode retightened", preexisting: true, preMode: 0o755},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			dir := filepath.Join(parent, "sub")
			if tc.preexisting {
				if err := os.Mkdir(dir, tc.preMode); err != nil {
					t.Fatalf("seed: %v", err)
				}
			}

			if err := config.EnsureDir(dir, config.DirPerm); err != nil {
				t.Fatalf("EnsureDir: %v", err)
			}

			info, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Mode().Perm() != config.DirPerm {
				t.Fatalf("perm = %o, want %o", info.Mode().Perm(), config.DirPerm)
			}
		})
	}
}
