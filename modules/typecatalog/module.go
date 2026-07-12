package typecatalog

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/joshmcarthur/trove/internal/modules"
	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
	"github.com/joshmcarthur/trove/pkg/trovemodule"
)

//go:embed manifest.toml
var manifestBytes []byte

// Module provides type catalog introspection via CLI and MCP.
type Module struct {
	ready chan struct{}
	core  trovemodule.Core
}

// New constructs a type-catalog module instance.
func New() trovemodule.Module {
	return &Module{ready: make(chan struct{})}
}

func (m *Module) Run(ctx context.Context, core trovemodule.Core) error {
	if core == nil {
		return fmt.Errorf("type-catalog: core connection is required")
	}
	m.core = core
	close(m.ready)
	<-ctx.Done()
	return nil
}

func (m *Module) waitReady(ctx context.Context) error {
	select {
	case <-m.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Module) RunCommand(ctx context.Context, command string, args []string) ([]byte, []byte, int, error) {
	if err := m.waitReady(ctx); err != nil {
		return nil, []byte("type-catalog: not ready\n"), 1, nil
	}
	if command != "types" {
		return nil, []byte(fmt.Sprintf("type-catalog: unknown command %q\n", command)), 1, nil
	}
	return runCLISubcommand(ctx, m.core, args)
}

func (m *Module) CallTool(ctx context.Context, name string, arguments json.RawMessage) (json.RawMessage, error) {
	if err := m.waitReady(ctx); err != nil {
		return nil, fmt.Errorf("type-catalog: not ready")
	}
	return callMCPTool(ctx, m.core, name, arguments)
}

func (m *Module) Healthcheck(ctx context.Context) (*troverpc.HealthcheckResponse, error) {
	select {
	case <-m.ready:
		return &troverpc.HealthcheckResponse{Ok: true, Message: "type-catalog ready"}, nil
	default:
		return &troverpc.HealthcheckResponse{Ok: false, Message: "type-catalog not ready"}, nil
	}
}

// Manifest returns the embedded module manifest.
func Manifest() (modules.Manifest, error) {
	return modules.ParseManifest(manifestBytes)
}

func runCLISubcommand(ctx context.Context, core trovemodule.Core, args []string) ([]byte, []byte, int, error) {
	if len(args) == 0 {
		return nil, []byte("usage: trove -config <path> types <list|describe|export|validate> ...\n"), 1, nil
	}
	switch args[0] {
	case "list":
		return runList(ctx, core, args[1:])
	case "describe":
		return runDescribe(ctx, core, args[1:])
	case "export":
		return runExport(ctx, core, args[1:])
	case "validate":
		return runValidate(ctx, core, args[1:])
	default:
		return nil, []byte(fmt.Sprintf("type-catalog: unknown subcommand %q\n", args[0])), 1, nil
	}
}

func runList(ctx context.Context, core trovemodule.Core, args []string) ([]byte, []byte, int, error) {
	fs := flag.NewFlagSet("types list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "output JSON")
	source := fs.String("source", "", "filter by source (builtin, user, or module name)")
	if err := fs.Parse(args); err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}

	types, err := core.ListTypes(ctx, *source)
	if err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	if *asJSON {
		out, err := json.MarshalIndent(types, "", "  ")
		if err != nil {
			return nil, []byte(err.Error() + "\n"), 1, nil
		}
		out = append(out, '\n')
		return out, nil, 0, nil
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "URI\tTITLE\tSOURCE\tSTATUS")
	for _, item := range types {
		title := item.Title
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.URI, title, item.Source, item.Status)
	}
	if err := w.Flush(); err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	return buf.Bytes(), nil, 0, nil
}

func runDescribe(ctx context.Context, core trovemodule.Core, args []string) ([]byte, []byte, int, error) {
	fs := flag.NewFlagSet("types describe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	uri := strings.TrimSpace(fs.Arg(0))
	if uri == "" {
		return nil, []byte("type-catalog: uri is required\n"), 1, nil
	}

	summary, definition, err := core.GetType(ctx, uri)
	if err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	if *asJSON {
		out, err := json.MarshalIndent(map[string]any{
			"summary":    summary,
			"definition": json.RawMessage(definition),
		}, "", "  ")
		if err != nil {
			return nil, []byte(err.Error() + "\n"), 1, nil
		}
		out = append(out, '\n')
		return out, nil, 0, nil
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "URI:         %s\n", summary.URI)
	if summary.Title != "" {
		fmt.Fprintf(&buf, "Title:       %s\n", summary.Title)
	}
	if summary.Description != "" {
		fmt.Fprintf(&buf, "Description: %s\n", summary.Description)
	}
	fmt.Fprintf(&buf, "Source:      %s\n", summary.Source)
	if summary.SourcePath != "" {
		fmt.Fprintf(&buf, "Source path: %s\n", summary.SourcePath)
	}
	fmt.Fprintf(&buf, "Schema ref:  %s\n", summary.SchemaRef)
	fmt.Fprintf(&buf, "Status:      %s\n", summary.Status)
	if len(definition) > 0 {
		fmt.Fprintf(&buf, "Definition:\n%s\n", prettyJSON(definition))
	}
	return buf.Bytes(), nil, 0, nil
}

func runExport(ctx context.Context, core trovemodule.Core, args []string) ([]byte, []byte, int, error) {
	fs := flag.NewFlagSet("types export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	outPath := fs.String("o", "", "write TTD to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	uri := strings.TrimSpace(fs.Arg(0))
	if uri == "" {
		return nil, []byte("type-catalog: uri is required\n"), 1, nil
	}

	data, _, err := core.ExportType(ctx, uri)
	if err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	pretty := prettyJSON(data)
	if *outPath != "" {
		if err := os.WriteFile(*outPath, pretty, 0o600); err != nil {
			return nil, []byte(err.Error() + "\n"), 1, nil
		}
		return []byte(fmt.Sprintf("exported %s to %s\n", uri, *outPath)), nil, 0, nil
	}
	if len(pretty) > 0 && pretty[len(pretty)-1] != '\n' {
		pretty = append(pretty, '\n')
	}
	return pretty, nil, 0, nil
}

func runValidate(ctx context.Context, core trovemodule.Core, args []string) ([]byte, []byte, int, error) {
	fs := flag.NewFlagSet("types validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	filePath := fs.String("file", "", "read TTD from file instead of stdin")
	if err := fs.Parse(args); err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}

	var data []byte
	var err error
	switch {
	case *filePath != "":
		data, err = os.ReadFile(*filePath)
	case fs.NArg() > 0:
		data = []byte(fs.Arg(0))
	default:
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, []byte("type-catalog: TTD input is required\n"), 1, nil
	}

	valid, uri, errMsg, err := core.ValidateTypeDefinition(ctx, data)
	if err != nil {
		return nil, []byte(err.Error() + "\n"), 1, nil
	}
	if !valid {
		return nil, []byte(errMsg + "\n"), 1, nil
	}
	return []byte(fmt.Sprintf("valid: %s\n", uri)), nil, 0, nil
}

func callMCPTool(ctx context.Context, core trovemodule.Core, name string, arguments json.RawMessage) (json.RawMessage, error) {
	switch name {
	case "list_types":
		var params struct {
			Source string `json:"source"`
		}
		if len(arguments) > 0 {
			if err := json.Unmarshal(arguments, &params); err != nil {
				return nil, fmt.Errorf("type-catalog: invalid arguments: %w", err)
			}
		}
		types, err := core.ListTypes(ctx, params.Source)
		if err != nil {
			return nil, err
		}
		return json.Marshal(types)
	case "describe_type":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(arguments, &params); err != nil {
			return nil, fmt.Errorf("type-catalog: invalid arguments: %w", err)
		}
		if strings.TrimSpace(params.URI) == "" {
			return nil, fmt.Errorf("type-catalog: uri is required")
		}
		summary, definition, err := core.GetType(ctx, params.URI)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"summary":    summary,
			"definition": json.RawMessage(definition),
		})
	case "export_type":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(arguments, &params); err != nil {
			return nil, fmt.Errorf("type-catalog: invalid arguments: %w", err)
		}
		if strings.TrimSpace(params.URI) == "" {
			return nil, fmt.Errorf("type-catalog: uri is required")
		}
		data, schemaRef, err := core.ExportType(ctx, params.URI)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"ttd":        json.RawMessage(data),
			"schema_ref": schemaRef,
		})
	case "validate_type_schema":
		var params struct {
			Schema json.RawMessage `json:"schema"`
		}
		if err := json.Unmarshal(arguments, &params); err != nil {
			return nil, fmt.Errorf("type-catalog: invalid arguments: %w", err)
		}
		if len(params.Schema) == 0 {
			return nil, fmt.Errorf("type-catalog: schema is required")
		}
		valid, uri, errMsg, err := core.ValidateTypeDefinition(ctx, params.Schema)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"valid": valid,
			"uri":   uri,
			"error": errMsg,
		})
	default:
		return nil, fmt.Errorf("type-catalog: unknown tool %q", name)
	}
}

func prettyJSON(data []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return data
	}
	return buf.Bytes()
}
