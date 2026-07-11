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
			Provides: []string{"classify.*"},
			MCP: manifestMCP{Tools: []MCPTool{{
				Name:        "classify_event",
				Description: "classify",
			}}},
		},
	}, {
		Manifest: Manifest{
			Name:     "two",
			Version:  "1.0",
			Kind:     KindSource,
			Provides: []string{"classify.*"},
			MCP: manifestMCP{Tools: []MCPTool{{
				Name:        "classify_event",
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
		Tool:   MCPTool{Name: "classify_event"},
		Module: "capture-classifier",
	}})
	if index["classify_event"] != "capture-classifier" {
		t.Fatalf("index = %#v", index)
	}
}
