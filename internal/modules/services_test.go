package modules

import (
	"context"
	"testing"

	"github.com/joshmcarthur/trove/internal/blob"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

func TestCoreServicesBlobPut(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	srv := &coreServicesServer{blobs: store}
	resp, err := srv.BlobPut(context.Background(), &troverpc.BlobPutRequest{Data: []byte("hello")})
	if err != nil {
		t.Fatalf("BlobPut() error = %v", err)
	}
	if resp.BlobRef == "" {
		t.Fatal("BlobRef is empty")
	}

	rc, err := store.Get(context.Background(), resp.BlobRef)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer rc.Close()

	buf := make([]byte, 5)
	if _, err := rc.Read(buf); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("stored data = %q, want hello", buf)
	}
}

func TestCoreServicesBlobPutEmpty(t *testing.T) {
	t.Parallel()

	store, err := blob.OpenFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystem() error = %v", err)
	}

	srv := &coreServicesServer{blobs: store}
	_, err = srv.BlobPut(context.Background(), &troverpc.BlobPutRequest{})
	if err == nil {
		t.Fatal("BlobPut() error = nil, want error")
	}
}
