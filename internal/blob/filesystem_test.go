package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *FilesystemStore {
	t.Helper()

	store, err := OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}
	return store
}

func refFor(data []byte) string {
	sum := sha256.Sum256(data)
	return FormatRef(fmt.Sprintf("%x", sum[:]))
}

func TestParseRef(t *testing.T) {
	t.Parallel()

	validHex := strings.Repeat("a", sha256HexLen)
	validRef := FormatRef(validHex)

	tests := []struct {
		name    string
		ref     string
		wantHex string
		wantErr bool
	}{
		{name: "valid", ref: validRef, wantHex: validHex},
		{name: "missing prefix", ref: validHex, wantErr: true},
		{name: "short hex", ref: FormatRef("abc"), wantErr: true},
		{name: "uppercase hex", ref: FormatRef(strings.ToUpper(validHex)), wantErr: true},
		{name: "non-hex", ref: FormatRef(strings.Repeat("g", sha256HexLen)), wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRef() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRef() error = %v", err)
			}
			if got != tt.wantHex {
				t.Errorf("hex = %q, want %q", got, tt.wantHex)
			}
		})
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()
	data := []byte("hello blob store")

	ref, err := store.Put(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if ref != refFor(data) {
		t.Errorf("ref = %q, want %q", ref, refFor(data))
	}

	rc, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("data = %q, want %q", got, data)
	}
}

func TestPutDedup(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()
	data := []byte("dedup me")

	ref1, err := store.Put(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}
	ref2, err := store.Put(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}
	if ref1 != ref2 {
		t.Errorf("refs differ: %q vs %q", ref1, ref2)
	}

	path, err := refPath(store.root, ref1)
	if err != nil {
		t.Fatalf("refPath() error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(store.root, "*", "*", "*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("blob file count = %d, want 1: %v", len(matches), matches)
	}
	if matches[0] != path {
		t.Errorf("blob path = %q, want %q", matches[0], path)
	}
}

func TestRange(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()
	data := []byte("0123456789abcdef")

	ref, err := store.Put(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	tests := []struct {
		name    string
		start   int64
		end     int64
		want    string
		wantErr bool
	}{
		{name: "full", start: 0, end: int64(len(data)), want: string(data)},
		{name: "prefix", start: 0, end: 4, want: "0123"},
		{name: "middle", start: 4, end: 8, want: "4567"},
		{name: "empty", start: 4, end: 4, want: ""},
		{name: "inverted", start: 8, end: 4, wantErr: true},
		{name: "overflow", start: 0, end: int64(len(data)) + 1, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc, err := store.Range(ctx, ref, tt.start, tt.end)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Range() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Range() error = %v", err)
			}
			defer rc.Close()

			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("range data = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnumerate(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	contents := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
	}
	want := make(map[string]struct{}, len(contents))
	for _, data := range contents {
		ref, err := store.Put(ctx, bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		want[ref] = struct{}{}
	}

	ch, err := store.Enumerate(ctx)
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}

	got := make(map[string]struct{})
	for ref := range ch {
		got[ref] = struct{}{}
	}

	if len(got) != len(want) {
		t.Fatalf("enumerate count = %d, want %d (got %v)", len(got), len(want), got)
	}
	for ref := range want {
		if _, ok := got[ref]; !ok {
			t.Errorf("missing ref %q", ref)
		}
	}
}

func TestGetInvalidRef(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "not-a-ref")
	if err == nil {
		t.Fatal("Get() error = nil, want error")
	}

	_, err = store.Range(ctx, "not-a-ref", 0, 1)
	if err == nil {
		t.Fatal("Range() error = nil, want error")
	}
}

func TestGetMissingBlob(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()
	ref := FormatRef(strings.Repeat("b", sha256HexLen))

	_, err := store.Get(ctx, ref)
	if err == nil {
		t.Fatal("Get() error = nil, want error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Get() error = %v, want os.ErrNotExist", err)
	}
}

func TestEnumerateContextCancel(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Enumerate(ctx)
	if err == nil {
		t.Fatal("Enumerate() error = nil, want context error")
	}
}

func TestOpenFilesystemCreatesRoot(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "blobs")
	store, err := OpenFilesystem(dir)
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	info, err := os.Stat(store.root)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("root is not a directory")
	}
}
