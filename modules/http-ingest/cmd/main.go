package main

import (
	"github.com/joshmcarthur/trove/modules/httpingest"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

func main() {
	trovemodule.Serve(httpingest.New())
}
