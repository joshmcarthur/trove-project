package bundled

import "github.com/joshmcarthur/trove/internal/modules"

func init() {
	modules.SetBundledDiscover(Modules)
}
