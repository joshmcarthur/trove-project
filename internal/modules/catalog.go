package modules

import (
	"fmt"

	"github.com/joshmcarthur/trove/internal/types"
)

// CollectModuleTypesInputs builds type catalog inputs from discovered modules.
func CollectModuleTypesInputs(mods []Module) ([]types.ModuleTypesInput, error) {
	var out []types.ModuleTypesInput
	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			return nil, fmt.Errorf("modules: collect types for %q: %w", mod.Manifest.Name, err)
		}
		if len(manifest.Types) == 0 {
			continue
		}
		decls := make([]types.TypeDecl, len(manifest.Types))
		for i, td := range manifest.Types {
			decls[i] = types.TypeDecl{
				Name:    td.Name,
				Version: td.Version,
				Schema:  td.Schema,
			}
		}
		out = append(out, types.ModuleTypesInput{
			ModuleName: manifest.Name,
			ModuleDir:  mod.Dir,
			Types:      decls,
		})
	}
	return out, nil
}
