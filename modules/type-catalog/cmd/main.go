package main

import (
	"github.com/joshmcarthur/trove/modules/typecatalog"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(typecatalog.New())
}
