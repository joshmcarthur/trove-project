package modules

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

// Kind identifies a module's role in the Trove pipeline.
type Kind string

const (
	KindSource    Kind = "source"
	KindProcessor Kind = "processor"
	KindSink      Kind = "sink"
)

// HTTPRoute declares an HTTP route served via the gateway.
type HTTPRoute struct {
	Method       string `toml:"method"`
	Path         string `toml:"path"`
	MaxBodyBytes int64  `toml:"max_body_bytes"`
}

type manifestHTTP struct {
	Routes []HTTPRoute `toml:"routes"`
}

type manifestMCP struct {
	Tools []MCPTool `toml:"tools"`
}

// MCPTool declares an MCP tool provided by a module.
type MCPTool struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// Manifest describes a Trove module from manifest.toml.
type Manifest struct {
	Name     string            `toml:"name"`
	Version  string            `toml:"version"`
	Kind     Kind              `toml:"kind"`
	Provides []string          `toml:"provides"`
	Consumes []string          `toml:"consumes"`
	Schemas  map[string]string `toml:"schemas"`
	HTTP     manifestHTTP      `toml:"http"`
	MCP      manifestMCP       `toml:"mcp"`
	Listen   string            `toml:"listen"`
}

// EventRoutes reports whether the module participates in journal event routing.
func (m Manifest) EventRoutes() bool {
	return len(m.Consumes) > 0
}

// HTTPRoutes returns declared HTTP routes from the manifest.
func (m Manifest) HTTPRoutes() []HTTPRoute {
	return m.HTTP.Routes
}

// MCPTools returns declared MCP tools from the manifest.
func (m Manifest) MCPTools() []MCPTool {
	return m.MCP.Tools
}

// ParseManifest parses and validates manifest TOML from data.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if _, err := toml.Decode(string(data), &m); err != nil {
		return Manifest{}, fmt.Errorf("modules: manifest: parse: %w", err)
	}
	if err := validateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ParseManifestFile reads and parses manifest.toml at path.
func ParseManifestFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("modules: manifest: read %q: %w", path, err)
	}
	return ParseManifest(data)
}

func validateManifest(m Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("modules: manifest: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("modules: manifest: version is required")
	}
	switch m.Kind {
	case KindSource, KindProcessor, KindSink:
	default:
		if m.Kind == "" {
			return fmt.Errorf("modules: manifest: kind is required")
		}
		return fmt.Errorf("modules: manifest: invalid kind %q", m.Kind)
	}

	if m.Kind == KindSource && len(m.Provides) == 0 {
		return fmt.Errorf("modules: manifest: provides is required for source modules")
	}

	if strings.TrimSpace(m.Listen) != "" {
		return fmt.Errorf("modules: manifest: listen must not be set on module %q (use gateway)", m.Name)
	}

	for _, pattern := range m.Provides {
		if err := validateTypePattern(pattern); err != nil {
			return err
		}
	}
	for _, pattern := range m.Consumes {
		if err := validateTypePattern(pattern); err != nil {
			return fmt.Errorf("modules: manifest: consumes: %w", err)
		}
	}
	if err := validateKindPatterns(m); err != nil {
		return err
	}
	for pattern := range m.Schemas {
		if err := validateProvidesPattern(pattern); err != nil {
			return fmt.Errorf("modules: manifest: schemas key: %w", err)
		}
	}

	for i, route := range m.HTTP.Routes {
		if err := validateHTTPRoute(route); err != nil {
			return fmt.Errorf("modules: manifest: http.routes[%d]: %w", i, err)
		}
	}

	for i, tool := range m.MCP.Tools {
		if err := validateMCPTool(tool); err != nil {
			return fmt.Errorf("modules: manifest: mcp.tools[%d]: %w", i, err)
		}
	}

	return nil
}

func validateMCPTool(tool MCPTool) error {
	if strings.TrimSpace(tool.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(tool.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

func validateHTTPRoute(route HTTPRoute) error {
	if route.Method == "" {
		return fmt.Errorf("method is required")
	}
	if route.Path == "" {
		return fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(route.Path, "/") {
		return fmt.Errorf("path must start with /")
	}
	return nil
}

func validateKindPatterns(m Manifest) error {
	hasConsumes := len(m.Consumes) > 0
	hasProvides := len(m.Provides) > 0
	hasHTTP := len(m.HTTP.Routes) > 0
	hasMCPTools := len(m.MCP.Tools) > 0

	switch m.Kind {
	case KindSource:
		if hasConsumes {
			return fmt.Errorf("modules: manifest: consumes is not allowed for source modules")
		}
	case KindSink:
		if hasProvides {
			return fmt.Errorf("modules: manifest: provides is not allowed for sink modules")
		}
		if !hasConsumes {
			return fmt.Errorf("modules: manifest: consumes is required for sink modules")
		}
	case KindProcessor:
		switch {
		case hasConsumes:
			// event-routing processor
		case hasHTTP, hasMCPTools:
			if hasProvides {
				return fmt.Errorf("modules: manifest: provides is not allowed for HTTP/MCP-only processor modules")
			}
		default:
			return fmt.Errorf("modules: manifest: processor %q must declare consumes, http.routes, and/or mcp.tools", m.Name)
		}
	}
	return nil
}

func validateTypePattern(pattern string) error {
	if pattern == "*" {
		return fmt.Errorf("modules: manifest: pattern %q is not allowed", pattern)
	}
	if _, err := path.Match(pattern, "x"); err != nil {
		return fmt.Errorf("modules: manifest: invalid pattern %q: %w", pattern, err)
	}
	return nil
}

func validateProvidesPattern(pattern string) error {
	return validateTypePattern(pattern)
}

// CollectHTTPRoutes gathers HTTP routes from discovered modules and rejects duplicates.
func CollectHTTPRoutes(mods []Module) ([]HTTPRouteEntry, error) {
	seen := make(map[string]struct{})
	var routes []HTTPRouteEntry

	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			return nil, err
		}
		for _, route := range manifest.HTTPRoutes() {
			key := route.Method + " " + route.Path
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("modules: duplicate http route %s for module %q", key, manifest.Name)
			}
			seen[key] = struct{}{}
			routes = append(routes, HTTPRouteEntry{
				Route:  route,
				Module: manifest.Name,
			})
		}
	}
	return routes, nil
}

// HTTPRouteEntry binds a manifest route to its module.
type HTTPRouteEntry struct {
	Route  HTTPRoute
	Module string
}

// MCPToolEntry binds a manifest MCP tool to its module.
type MCPToolEntry struct {
	Tool   MCPTool
	Module string
}

// CollectMCPTools gathers MCP tools from discovered modules and rejects duplicates.
func CollectMCPTools(mods []Module) ([]MCPToolEntry, error) {
	seen := make(map[string]struct{})
	var tools []MCPToolEntry

	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			return nil, err
		}
		for _, tool := range manifest.MCPTools() {
			if _, exists := seen[tool.Name]; exists {
				return nil, fmt.Errorf("modules: duplicate mcp tool %q for module %q", tool.Name, manifest.Name)
			}
			seen[tool.Name] = struct{}{}
			tools = append(tools, MCPToolEntry{
				Tool:   tool,
				Module: manifest.Name,
			})
		}
	}
	return tools, nil
}

// MCPToolModuleIndex maps MCP tool names to providing module names.
func MCPToolModuleIndex(entries []MCPToolEntry) map[string]string {
	index := make(map[string]string, len(entries))
	for _, entry := range entries {
		index[entry.Tool.Name] = entry.Module
	}
	return index
}
