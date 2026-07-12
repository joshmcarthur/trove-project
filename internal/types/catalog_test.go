package types_test

import (
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/types"
)

const noteCreatedURI = "trove://type/note/created/1"

func TestCatalogUserOverridesModule(t *testing.T) {
	t.Parallel()
	c := types.NewCatalog()

	moduleEntry := types.Entry{
		URI:        noteCreatedURI,
		SchemaRef:  "blob:aaa",
		Source:     "notes-module",
		SourcePath: "/modules/notes/note-created.json",
	}
	if _, err := c.Register(moduleEntry); err != nil {
		t.Fatalf("Register(module) error = %v", err)
	}

	userEntry := types.Entry{
		URI:        noteCreatedURI,
		SchemaRef:  "blob:bbb",
		Source:     "user",
		SourcePath: "/home/user/types/note-created.json",
	}
	warning, err := c.Register(userEntry)
	if err != nil {
		t.Fatalf("Register(user) error = %v", err)
	}
	if warning == "" {
		t.Fatal("Register(user) warning = empty, want user override warning")
	}
	if !strings.Contains(warning, "user override replaces") {
		t.Fatalf("Register(user) warning = %q, want substring %q", warning, "user override replaces")
	}

	got, ok := c.Lookup(noteCreatedURI)
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if got.SchemaRef != "blob:bbb" {
		t.Fatalf("Lookup() SchemaRef = %q, want %q", got.SchemaRef, "blob:bbb")
	}
	if got.Source != "user" {
		t.Fatalf("Lookup() Source = %q, want %q", got.Source, "user")
	}
}

func TestCatalogRejectsConflictingModules(t *testing.T) {
	t.Parallel()
	c := types.NewCatalog()

	first := types.Entry{
		URI:       noteCreatedURI,
		SchemaRef: "blob:aaa",
		Source:    "module-a",
	}
	if _, err := c.Register(first); err != nil {
		t.Fatalf("Register(module-a) error = %v", err)
	}

	second := types.Entry{
		URI:       noteCreatedURI,
		SchemaRef: "blob:bbb",
		Source:    "module-b",
	}
	_, err := c.Register(second)
	if err == nil {
		t.Fatal("Register(module-b) error = nil, want conflicting definitions error")
	}
	if !strings.Contains(err.Error(), "conflicting definitions") {
		t.Fatalf("Register(module-b) error = %v, want conflicting definitions", err)
	}

	got, ok := c.Lookup(noteCreatedURI)
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if got.Source != "module-a" {
		t.Fatalf("Lookup() Source = %q, want original module-a entry preserved", got.Source)
	}
}

func TestCatalogSameModuleReregisterNoWarning(t *testing.T) {
	t.Parallel()
	c := types.NewCatalog()
	entry := types.Entry{
		URI:       noteCreatedURI,
		SchemaRef: "blob:aaa",
		Source:    "notes-module",
	}
	if _, err := c.Register(entry); err != nil {
		t.Fatalf("Register() first error = %v", err)
	}
	warning, err := c.Register(entry)
	if err != nil {
		t.Fatalf("Register() second error = %v", err)
	}
	if warning != "" {
		t.Fatalf("Register() second warning = %q, want empty", warning)
	}
}
