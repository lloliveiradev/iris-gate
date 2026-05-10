package hasher_test

import (
	"strings"
	"testing"

	"github.com/iris-gate/iris-gate/libs/identity/infra/hasher"
)

// Params reduzidos pra acelerar testes (não usar em prod).
func cheap() hasher.Params {
	return hasher.Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLen: 8, KeyLen: 16}
}

func TestArgon2_RoundTrip(t *testing.T) {
	t.Parallel()
	h := hasher.New(cheap())

	encoded, err := h.Hash("hunter2-very-secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Errorf("PHC string mal formada: %q", encoded)
	}

	ok, err := h.Verify("hunter2-very-secret", encoded)
	if err != nil || !ok {
		t.Fatalf("Verify(correct) ok=%v err=%v", ok, err)
	}
	ok, err = h.Verify("WRONG", encoded)
	if err != nil || ok {
		t.Fatalf("Verify(wrong) ok=%v err=%v (expected ok=false)", ok, err)
	}
}

func TestArgon2_DifferentSaltsProduceDifferentHashes(t *testing.T) {
	t.Parallel()
	h := hasher.New(cheap())

	a, _ := h.Hash("same-secret")
	b, _ := h.Hash("same-secret")
	if a == b {
		t.Fatalf("hashes iguais — salt não é random")
	}
}

func TestSecretGenerator_FormatAndUniqueness(t *testing.T) {
	t.Parallel()
	g := hasher.SecretGenerator{}
	seen := make(map[string]struct{})
	for range 50 {
		prefix, secret, err := g.NewToken()
		if err != nil {
			t.Fatalf("NewToken err = %v", err)
		}
		if len(prefix) != 16 {
			t.Errorf("prefix len = %d, want 16", len(prefix))
		}
		if len(secret) < 24 {
			t.Errorf("secret too short: %q", secret)
		}
		if _, dup := seen[prefix]; dup {
			t.Fatalf("collision em prefix %s", prefix)
		}
		seen[prefix] = struct{}{}
	}
}
