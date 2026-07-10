package blob

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var _ Store = (*FilesystemStore)(nil)

// FilesystemStore stores blobs on the local filesystem using hash-prefix paths.
type FilesystemStore struct {
	root string
}

// OpenFilesystem opens or creates a filesystem blob store at path.
func OpenFilesystem(path string) (*FilesystemStore, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("blob: filesystem path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("blob: create root %q: %w", abs, err)
	}
	return &FilesystemStore{root: abs}, nil
}

func (s *FilesystemStore) Put(ctx context.Context, data io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	hasher := sha256.New()
	tee := io.TeeReader(data, hasher)

	tmpDir, err := os.MkdirTemp(s.root, ".blob-put-*")
	if err != nil {
		return "", fmt.Errorf("blob: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "data")
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("blob: create temp file: %w", err)
	}

	if _, err := io.Copy(f, tee); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("blob: write: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("blob: close temp file: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	hex := fmt.Sprintf("%x", hasher.Sum(nil))
	ref := FormatRef(hex)

	dest, err := refPath(s.root, ref)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err == nil {
		return ref, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("blob: stat %q: %w", dest, err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("blob: create parent dir: %w", err)
	}

	finalTmp, err := os.CreateTemp(filepath.Dir(dest), ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("blob: create commit temp: %w", err)
	}
	finalTmpPath := finalTmp.Name()
	_ = finalTmp.Close()
	if err := os.Rename(tmpFile, finalTmpPath); err != nil {
		_ = os.Remove(finalTmpPath)
		return "", fmt.Errorf("blob: stage write: %w", err)
	}
	if err := os.Rename(finalTmpPath, dest); err != nil {
		_ = os.Remove(finalTmpPath)
		return "", fmt.Errorf("blob: commit write: %w", err)
	}

	return ref, nil
}

func (s *FilesystemStore) Get(ctx context.Context, ref string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path, err := refPath(s.root, ref)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("blob: get %q: %w", ref, err)
		}
		return nil, fmt.Errorf("blob: open %q: %w", ref, err)
	}
	return f, nil
}

func (s *FilesystemStore) Range(ctx context.Context, ref string, start, end int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if start < 0 || end < 0 || start > end {
		return nil, fmt.Errorf("blob: invalid range [%d, %d)", start, end)
	}

	path, err := refPath(s.root, ref)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("blob: range %q: %w", ref, err)
		}
		return nil, fmt.Errorf("blob: open %q: %w", ref, err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("blob: stat %q: %w", ref, err)
	}

	size := info.Size()
	if end > size {
		_ = f.Close()
		return nil, fmt.Errorf("blob: range [%d, %d) exceeds size %d", start, end, size)
	}

	section := io.NewSectionReader(f, start, end-start)
	return &sectionReadCloser{SectionReader: section, closer: f}, nil
}

func (s *FilesystemStore) Enumerate(ctx context.Context) (<-chan string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		_ = filepath.WalkDir(s.root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".blob-put-") || strings.HasPrefix(name, ".tmp-") {
					return filepath.SkipDir
				}
				return nil
			}
			ref, ok := refFromPath(s.root, path)
			if !ok {
				return nil
			}
			select {
			case ch <- ref:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	return ch, nil
}

type sectionReadCloser struct {
	*io.SectionReader
	closer io.Closer
}

func (s *sectionReadCloser) Close() error {
	return s.closer.Close()
}
