package bundled

import (
	"fmt"
	"os"

	"github.com/joshmcarthur/trove/internal/modules"
	"github.com/joshmcarthur/trove/modules/httpingest"
	"github.com/joshmcarthur/trove/modules/mcpquery"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

type entry struct {
	name     string
	new      func() trovemodule.Module
	manifest func() (modules.Manifest, error)
}

var registry = []entry{
	{
		name:     "http-ingest",
		new:      httpingest.New,
		manifest: httpingest.Manifest,
	},
	{
		name:     "mcp-query",
		new:      mcpquery.New,
		manifest: mcpquery.Manifest,
	},
}

// Serve starts a built-in module subprocess entrypoint.
func Serve(name string) {
	for _, entry := range registry {
		if entry.name != name {
			continue
		}
		trovemodule.Serve(entry.new())
		return
	}
	fmt.Fprintf(os.Stderr, "trove: unknown bundled module %q\n", name)
	os.Exit(1)
}

// Modules returns built-in module descriptors for discovery fallback.
func Modules() ([]modules.Module, error) {
	out := make([]modules.Module, 0, len(registry))
	for _, entry := range registry {
		manifest, err := entry.manifest()
		if err != nil {
			return nil, fmt.Errorf("bundled: manifest %q: %w", entry.name, err)
		}
		out = append(out, modules.Module{
			Manifest: manifest,
			Bundled:  true,
		})
	}
	return out, nil
}
