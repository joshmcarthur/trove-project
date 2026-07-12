package modules_test

import (
	"strings"
	"testing"

	"github.com/joshmcarthur/trove/internal/modules"
)

func TestCollectCLICommandsRejectsDuplicates(t *testing.T) {
	parse := func(name string) modules.Module {
		manifest, err := modules.ParseManifest([]byte(`
name = "` + name + `"
version = "1.0"
kind = "processor"

[[cli.commands]]
name = "types"
description = "types"
`))
		if err != nil {
			t.Fatalf("ParseManifest(%q) error = %v", name, err)
		}
		return modules.Module{Manifest: manifest}
	}

	_, err := modules.CollectCLICommands([]modules.Module{parse("one"), parse("two")})
	if err == nil {
		t.Fatal("CollectCLICommands() error = nil, want duplicate error")
	}
}

func TestCollectCLICommandsRejectsReservedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
	}{
		{name: "version", command: "version"},
		{name: "init", command: "init"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := modules.ParseManifest([]byte(`
name = "bad"
version = "1.0"
kind = "processor"

[[cli.commands]]
name = "` + tt.command + `"
description = "shadow core"
`))
			if err == nil {
				t.Fatal("ParseManifest() error = nil, want reserved name error")
			}
			if !strings.Contains(err.Error(), "reserved") {
				t.Fatalf("ParseManifest() error = %v, want reserved", err)
			}
		})
	}
}

func TestParseManifestCLIOnlyProcessor(t *testing.T) {
	_, err := modules.ParseManifest([]byte(`
name = "type-catalog"
version = "1.0"
kind = "processor"

[[cli.commands]]
name = "types"
description = "List types"
`))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
}
