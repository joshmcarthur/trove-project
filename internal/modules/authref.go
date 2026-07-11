package modules

import "fmt"

const (
	// AuthInherit uses the gateway default validator from config.
	AuthInherit = "inherit"
	// AuthNone skips gateway auth for a route.
	AuthNone = "none"
	// HTTPCapability is the implicit capability suffix for HTTP route dispatch.
	HTTPCapability = "http"
)

// ModuleRef is a module capability reference: module.<name>.<capability>.
type ModuleRef struct {
	Module     string
	Capability string
}

// ModuleRef formats a module capability reference.
func FormatModuleRef(moduleName, capability string) string {
	return "module." + moduleName + "." + capability
}

// ParseModuleRef splits module.<name>.<capability> into its parts.
func ParseModuleRef(ref string) (ModuleRef, error) {
	if ref == AuthInherit || ref == AuthNone || ref == "" {
		return ModuleRef{}, fmt.Errorf("modules: not a module ref %q", ref)
	}
	const prefix = "module."
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return ModuleRef{}, fmt.Errorf("modules: invalid module ref %q (want module.<name>.<capability>)", ref)
	}
	rest := ref[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] != '.' {
			continue
		}
		moduleName := rest[:i]
		capability := rest[i+1:]
		if moduleName == "" || capability == "" {
			break
		}
		return ModuleRef{Module: moduleName, Capability: capability}, nil
	}
	return ModuleRef{}, fmt.Errorf("modules: invalid module ref %q (want module.<name>.<capability>)", ref)
}

// NormalizeRouteAuth returns inherit when auth is unset.
func NormalizeRouteAuth(auth string) string {
	if auth == "" {
		return AuthInherit
	}
	return auth
}

// ResolveRouteAuth maps a route auth policy to a module ref or skip.
func ResolveRouteAuth(routeAuth, defaultValidator string) (validatorRef string, skip bool, err error) {
	auth := NormalizeRouteAuth(routeAuth)
	switch auth {
	case AuthInherit:
		if defaultValidator == "" {
			return "", true, nil
		}
		return defaultValidator, false, nil
	case AuthNone:
		return "", true, nil
	default:
		if _, err := ParseModuleRef(auth); err != nil {
			return "", false, err
		}
		return auth, false, nil
	}
}
