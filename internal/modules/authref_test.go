package modules

import "testing"

func TestParseAuthValidatorRef(t *testing.T) {
	t.Parallel()

	moduleName, validatorID, err := ParseAuthValidatorRef("module.http-gateway.bearer")
	if err != nil {
		t.Fatalf("ParseAuthValidatorRef() error = %v", err)
	}
	if moduleName != "http-gateway" || validatorID != "bearer" {
		t.Fatalf("got module=%q validator=%q", moduleName, validatorID)
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
