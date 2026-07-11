package modules

import "fmt"

const (
	// AuthInherit uses the gateway default validator from config.
	AuthInherit = "inherit"
	// AuthNone skips gateway auth for a route.
	AuthNone = "none"
)

// AuthValidatorRef formats a validator reference for config and manifests.
func AuthValidatorRef(moduleName, validatorID string) string {
	return "module." + moduleName + "." + validatorID
}

// ParseAuthValidatorRef splits module.<name>.<validator> into module name and validator id.
func ParseAuthValidatorRef(ref string) (moduleName, validatorID string, err error) {
	if ref == AuthInherit || ref == AuthNone || ref == "" {
		return "", "", fmt.Errorf("modules: not a module validator ref %q", ref)
	}
	const prefix = "module."
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return "", "", fmt.Errorf("modules: invalid auth validator ref %q (want module.<name>.<validator>)", ref)
	}
	rest := ref[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] != '.' {
			continue
		}
		moduleName = rest[:i]
		validatorID = rest[i+1:]
		break
	}
	if moduleName == "" || validatorID == "" {
		return "", "", fmt.Errorf("modules: invalid auth validator ref %q (want module.<name>.<validator>)", ref)
	}
	return moduleName, validatorID, nil
}

// NormalizeRouteAuth returns inherit when auth is unset.
func NormalizeRouteAuth(auth string) string {
	if auth == "" {
		return AuthInherit
	}
	return auth
}

// ResolveRouteAuth maps a route auth policy to a validator ref or skip.
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
		if _, _, err := ParseAuthValidatorRef(auth); err != nil {
			return "", false, err
		}
		return auth, false, nil
	}
}
