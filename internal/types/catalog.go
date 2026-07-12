package types

import "fmt"

// Entry is a registered type in the catalog.
type Entry struct {
	URI        string
	SchemaRef  string
	Compiled   *CompiledType
	Source     string // module name or "user"
	SourcePath string
}

// Catalog holds all active types keyed by URI.
type Catalog struct {
	entries map[string]Entry
}

// NewCatalog returns an empty type catalog.
func NewCatalog() *Catalog {
	return &Catalog{entries: make(map[string]Entry)}
}

// Register adds or replaces an entry. Returns a warning when a user override
// replaces an existing definition with a different schema_ref.
func (c *Catalog) Register(e Entry) (warning string, err error) {
	if prev, ok := c.entries[e.URI]; ok && prev.SchemaRef != e.SchemaRef {
		if prev.Source != "user" && e.Source != "user" && prev.Source != e.Source {
			return "", fmt.Errorf("types: conflicting definitions for %s from %q and %q", e.URI, prev.Source, e.Source)
		}
		if e.Source == "user" {
			warning = fmt.Sprintf("types: user override replaces %s (was %s, now %s)", e.URI, prev.Source, e.Source)
		}
	}
	c.entries[e.URI] = e
	return warning, nil
}

// RegisterPermissive adds a type with an empty JTD schema that accepts any payload.
func (c *Catalog) RegisterPermissive(uri string) error {
	td := TypeDefinition{
		ID:         uri,
		Definition: []byte(`{}`),
	}
	ct, err := Compile(td)
	if err != nil {
		return err
	}
	_, err = c.Register(Entry{
		URI:       uri,
		SchemaRef: "blob:" + uri,
		Compiled:  ct,
		Source:    "test",
	})
	return err
}

func (c *Catalog) Lookup(uri string) (Entry, bool) {
	e, ok := c.entries[uri]
	return e, ok
}
