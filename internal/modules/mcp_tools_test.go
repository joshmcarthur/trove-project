package modules

import (
	"testing"
)

func TestCollectMCPToolsRejectsDuplicates(t *testing.T) {
	mods := []Module{{
		Manifest: Manifest{
			Name:     "one",
			Version:  "1.0",
			Kind:     KindSource,
			Provides: []string{"trove://type/example/*"},
			MCP: manifestMCP{Tools: []MCPTool{{
				Name:        "example_tool",
				Description: "first",
			}}},
		},
	}, {
		Manifest: Manifest{
			Name:     "two",
			Version:  "1.0",
			Kind:     KindSource,
			Provides: []string{"trove://type/example/*"},
			MCP: manifestMCP{Tools: []MCPTool{{
				Name:        "example_tool",
				Description: "duplicate",
			}}},
		},
	}}

	_, err := CollectMCPTools(mods)
	if err == nil {
		t.Fatal("CollectMCPTools() error = nil, want duplicate error")
	}
}

func TestMCPToolModuleIndex(t *testing.T) {
	index := MCPToolModuleIndex([]MCPToolEntry{{
		Tool:   MCPTool{Name: "example_tool"},
		Module: "example-module",
	}})
	if index["example_tool"] != "example-module" {
		t.Fatalf("index = %#v", index)
	}
}
