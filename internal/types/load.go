package types

import (
	"fmt"
	"os"
	"strings"
)

// TypeDecl describes a type contributed by config or a module manifest.
type TypeDecl struct {
	Name    string
	Version int
	Schema  string // file path
	Source  string
}

// LoadTypeFile reads a TTD JSON file and parses it.
func LoadTypeFile(path string, source string) (TypeDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("types: load %s (source %q): %w", path, source, err)
	}
	return ParseTypeDefinition(data)
}

// DeclURI returns the trove:// type URI for a declaration.
func DeclURI(decl TypeDecl) (string, error) {
	if strings.TrimSpace(decl.Name) == "" {
		return "", fmt.Errorf("types: type name is required")
	}
	if decl.Version < 1 {
		return "", fmt.Errorf("types: type version must be positive")
	}
	return FormatTypeURI(NameToPath(decl.Name), decl.Version), nil
}
