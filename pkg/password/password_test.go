package password_test

import (
	"testing"

	"github.com/raulaguila/authenticity/pkg/password"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := password.Verify(h, "correct horse battery")
	if err != nil || !ok {
		t.Fatalf("verify want true, got %v %v", ok, err)
	}
	ok, err = password.Verify(h, "wrong")
	if err != nil || ok {
		t.Fatalf("verify want false, got %v %v", ok, err)
	}
}
