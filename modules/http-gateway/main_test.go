package main

import (
	"context"
	"testing"

	troverpc "github.com/joshmcarthur/trove/internal/modules/rpc/trove/v1"
)

func TestValidateAuthBearer(t *testing.T) {
	t.Parallel()

	mod := &httpGatewayModule{cfg: config{Token: "secret"}}
	mod.ready.Store(true)

	resp, err := mod.ValidateAuth(context.Background(), &troverpc.AuthRequest{
		ValidatorId: bearerValidatorID,
		Headers:     map[string]string{"Authorization": "Bearer secret"},
	})
	if err != nil {
		t.Fatalf("ValidateAuth() error = %v", err)
	}
	if resp == nil || !resp.Allowed {
		t.Fatalf("resp = %#v, want allowed", resp)
	}
}

func TestValidateAuthRejectsMissingToken(t *testing.T) {
	t.Parallel()

	mod := &httpGatewayModule{cfg: config{Token: "secret"}}
	mod.ready.Store(true)

	resp, err := mod.ValidateAuth(context.Background(), &troverpc.AuthRequest{
		ValidatorId: bearerValidatorID,
	})
	if err != nil {
		t.Fatalf("ValidateAuth() error = %v", err)
	}
	if resp == nil || resp.Allowed || resp.Status != 401 {
		t.Fatalf("resp = %#v, want denied 401", resp)
	}
}
