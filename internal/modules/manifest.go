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
	Auth         string `toml:"auth"`
}

type manifestHTTP struct {
	Routes []HTTPRoute `toml:"routes"`
}

type manifestMCP struct {
	Tools []MCPTool `toml:"tools"`
}

type manifestCLI struct {
	Commands []CLICommand `toml:"commands"`
}

type manifestAuth struct {
	Validators []AuthValidatorDecl `toml:"validators"`
}

// AuthValidatorDecl declares an auth validator provided by a module.
type AuthValidatorDecl struct {
	ID          string `toml:"id"`
	Description string `toml:"description"`
}

// MCPTool declares an MCP tool provided by a module.
type MCPTool struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// CLICommand declares a top-level CLI command provided by a module.
type CLICommand struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// ManifestTypeDecl declares a type contributed by a module manifest.
type ManifestTypeDecl struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
	Schema  string `toml:"schema"`
}

// Manifest describes a Trove module from manifest.toml.
type Manifest struct {
	Name     string             `toml:"name"`
	Version  string             `toml:"version"`
	Kind     Kind               `toml:"kind"`
	Provides []string           `toml:"provides"`
	Consumes []string           `toml:"consumes"`
	Types    []ManifestTypeDecl `toml:"types"`
	HTTP     manifestHTTP       `toml:"http"`
	MCP      manifestMCP        `toml:"mcp"`
	CLI      manifestCLI        `toml:"cli"`
	Auth     manifestAuth       `toml:"auth"`
	Listen   string             `toml:"listen"`
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

// CLICommands returns declared CLI commands from the manifest.
func (m Manifest) CLICommands() []CLICommand {
	return m.CLI.Commands
}

// AuthValidators returns declared auth validators from the manifest.
func (m Manifest) AuthValidators() []AuthValidatorDecl {
	return m.Auth.Validators
}

// ParseManifest parses and validates manifest TOML from data.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	meta, err := toml.Decode(string(data), &m)
	if err != nil {
		return Manifest{}, fmt.Errorf("modules: manifest: parse: %w", err)
	}
	if meta.IsDefined("schemas") {
		return Manifest{}, fmt.Errorf("modules: manifest: [schemas] is removed; use [[types]] with TTD files")
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

	for i, td := range m.Types {
		if err := validateManifestTypeDecl(td); err != nil {
			return fmt.Errorf("modules: manifest: types[%d]: %w", i, err)
		}
	}

	for i, route := range m.HTTP.Routes {
		if err := validateHTTPRoute(route); err != nil {
			return fmt.Errorf("modules: manifest: http.routes[%d]: %w", i, err)
		}
	}

	for i, validator := range m.Auth.Validators {
		if err := validateAuthValidator(validator); err != nil {
			return fmt.Errorf("modules: manifest: auth.validators[%d]: %w", i, err)
		}
	}

	for i, tool := range m.MCP.Tools {
		if err := validateMCPTool(tool); err != nil {
			return fmt.Errorf("modules: manifest: mcp.tools[%d]: %w", i, err)
		}
	}

	for i, command := range m.CLI.Commands {
		if err := validateCLICommand(command); err != nil {
			return fmt.Errorf("modules: manifest: cli.commands[%d]: %w", i, err)
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

func validateCLICommand(command CLICommand) error {
	if strings.TrimSpace(command.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(command.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if isReservedCLICommand(command.Name) {
		return fmt.Errorf("name %q is reserved", command.Name)
	}
	return nil
}

func isReservedCLICommand(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "version", "config", "help":
		return true
	default:
		return false
	}
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
	if route.Auth != "" && route.Auth != AuthInherit && route.Auth != AuthNone {
		if _, err := ParseModuleRef(route.Auth); err != nil {
			return err
		}
	}
	return nil
}

func validateAuthValidator(validator AuthValidatorDecl) error {
	if strings.TrimSpace(validator.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

func validateManifestTypeDecl(decl ManifestTypeDecl) error {
	if strings.TrimSpace(decl.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(decl.Schema) == "" {
		return fmt.Errorf("schema is required")
	}
	if decl.Version < 1 {
		return fmt.Errorf("version must be >= 1")
	}
	return nil
}

func validateKindPatterns(m Manifest) error {
	hasConsumes := len(m.Consumes) > 0
	hasProvides := len(m.Provides) > 0
	hasHTTP := len(m.HTTP.Routes) > 0
	hasMCPTools := len(m.MCP.Tools) > 0
	hasCLI := len(m.CLI.Commands) > 0
	hasAuth := len(m.Auth.Validators) > 0

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
		case hasHTTP, hasMCPTools, hasCLI, hasAuth:
			if hasProvides {
				return fmt.Errorf("modules: manifest: provides is not allowed for HTTP/MCP/CLI/auth-only processor modules")
			}
		default:
			return fmt.Errorf("modules: manifest: processor %q must declare consumes, http.routes, auth.validators, cli.commands, and/or mcp.tools", m.Name)
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

// CLICommandEntry binds a manifest CLI command to its module.
type CLICommandEntry struct {
	Command CLICommand
	Module  string
}

// CollectCLICommands gathers CLI commands from discovered modules and rejects duplicates.
func CollectCLICommands(mods []Module) ([]CLICommandEntry, error) {
	seen := make(map[string]struct{})
	var commands []CLICommandEntry

	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			return nil, err
		}
		for _, command := range manifest.CLICommands() {
			if isReservedCLICommand(command.Name) {
				return nil, fmt.Errorf("modules: cli command %q is reserved", command.Name)
			}
			if _, exists := seen[command.Name]; exists {
				return nil, fmt.Errorf("modules: duplicate cli command %q for module %q", command.Name, manifest.Name)
			}
			seen[command.Name] = struct{}{}
			commands = append(commands, CLICommandEntry{
				Command: command,
				Module:  manifest.Name,
			})
		}
	}
	return commands, nil
}

// CLICommandModuleIndex maps CLI command names to providing module names.
func CLICommandModuleIndex(entries []CLICommandEntry) map[string]string {
	index := make(map[string]string, len(entries))
	for _, entry := range entries {
		index[entry.Command.Name] = entry.Module
	}
	return index
}

// CollectAuthValidatorRefs gathers declared auth validator refs from discovered modules.
func CollectAuthValidatorRefs(mods []Module) (map[string]struct{}, error) {
	refs := make(map[string]struct{})
	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			return nil, err
		}
		for _, validator := range manifest.AuthValidators() {
			ref := FormatModuleRef(manifest.Name, validator.ID)
			if _, exists := refs[ref]; exists {
				return nil, fmt.Errorf("modules: duplicate auth validator %q", ref)
			}
			refs[ref] = struct{}{}
		}
	}
	return refs, nil
}

// ValidateAuthConfig ensures configured validator refs are declared by discovered modules.
func ValidateAuthConfig(defaultValidator string, routes []HTTPRouteEntry, declared map[string]struct{}) error {
	validateRef := func(ref string) error {
		if ref == "" {
			return nil
		}
		if _, err := ParseModuleRef(ref); err != nil {
			return err
		}
		if _, ok := declared[ref]; !ok {
			return fmt.Errorf("modules: auth validator %q is not declared by any discovered module", ref)
		}
		return nil
	}

	if err := validateRef(defaultValidator); err != nil {
		return fmt.Errorf("config: http.auth.validator: %w", err)
	}

	for _, route := range routes {
		auth := NormalizeRouteAuth(route.Route.Auth)
		if auth == AuthInherit || auth == AuthNone {
			continue
		}
		if err := validateRef(auth); err != nil {
			return fmt.Errorf("modules: http route %s %s auth: %w", route.Route.Method, route.Route.Path, err)
		}
	}
	return nil
}
