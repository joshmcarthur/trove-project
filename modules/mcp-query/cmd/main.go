package main

import (
	"github.com/joshmcarthur/trove/modules/mcpquery"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(mcpquery.New())
}
