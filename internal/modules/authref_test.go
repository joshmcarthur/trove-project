package modules

import "testing"

func TestParseModuleRef(t *testing.T) {
	t.Parallel()

	ref, err := ParseModuleRef("module.http-gateway.bearer")
	if err != nil {
		t.Fatalf("ParseModuleRef() error = %v", err)
	}
	if ref.Module != "http-gateway" || ref.Capability != "bearer" {
		t.Fatalf("got module=%q capability=%q", ref.Module, ref.Capability)
	}
}

func TestResolveRouteAuth(t *testing.T) {
	t.Parallel()

	ref, skip, err := ResolveRouteAuth("inherit", "module.http-gateway.bearer")
	if err != nil || skip || ref != "module.http-gateway.bearer" {
		t.Fatalf("inherit: ref=%q skip=%v err=%v", ref, skip, err)
	}

	ref, skip, err = ResolveRouteAuth("none", "module.http-gateway.bearer")
	if err != nil || !skip {
		t.Fatalf("none: ref=%q skip=%v err=%v", ref, skip, err)
	}

	ref, skip, err = ResolveRouteAuth("module.webhook-github.hmac", "")
	if err != nil || skip || ref != "module.webhook-github.hmac" {
		t.Fatalf("custom: ref=%q skip=%v err=%v", ref, skip, err)
	}
}
