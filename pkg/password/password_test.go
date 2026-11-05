package password_test

import (
	"context"
	"testing"

	"github.com/raulaguila/authenticity/pkg/password"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	ctx := context.Background()
	h, err := password.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := password.Verify(ctx, h, "correct horse battery")
	if err != nil || !ok {
		t.Fatalf("verify want true, got %v %v", ok, err)
	}
	ok, err = password.Verify(ctx, h, "wrong")
	if err != nil || ok {
		t.Fatalf("verify want false, got %v %v", ok, err)
	}
}
