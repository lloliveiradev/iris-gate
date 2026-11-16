package acquirer_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/iris-gate/iris-gate/libs/payments/infra/acquirer"
	"github.com/iris-gate/iris-gate/libs/payments/testfixtures"
)

// A1: same key → same AcquirerRef; different keys → different refs; the
// counter grows only on a key's first use.
func TestFake_SameKeySameResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := acquirer.NewFake()
	in := authorizeInput(100, testfixtures.TokenApproved)

	first, err := fake.Authorize(ctx, in)
	if err != nil || !first.Approved {
		t.Fatalf("first = (%+v, %v), want approved", first, err)
	}
	again, err := fake.Authorize(ctx, in)
	if err != nil || again != first {
		t.Errorf("replay = (%+v, %v), want %+v", again, err, first)
	}
	if n := fake.Authorizations(); n != 1 {
		t.Errorf("authorizations after replay = %d, want 1", n)
	}

	other, err := fake.Authorize(ctx, authorizeInput(100, testfixtures.TokenApproved))
	if err != nil || other.AcquirerRef == first.AcquirerRef {
		t.Errorf("other key = (%+v, %v), want a new ref", other, err)
	}
	if n := fake.Authorizations(); n != 2 {
		t.Errorf("authorizations = %d, want 2", n)
	}
}

func TestFake_DeclinesConfiguredTokensOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := acquirer.NewFake(testfixtures.TokenDeclined)

	declined, err := fake.Authorize(ctx, authorizeInput(100, testfixtures.TokenDeclined))
	if err != nil || declined.Approved {
		t.Errorf("declined token = (%+v, %v), want a decline", declined, err)
	}
	approved, err := fake.Authorize(ctx, authorizeInput(100, testfixtures.TokenApproved))
	if err != nil || !approved.Approved {
		t.Errorf("other token = (%+v, %v), want approval", approved, err)
	}
}

func TestFake_DefaultApprovesEverything(t *testing.T) {
	t.Parallel()
	out, err := acquirer.NewFake().Authorize(context.Background(), authorizeInput(100, testfixtures.TokenDeclined))
	if err != nil || !out.Approved {
		t.Errorf("default fake = (%+v, %v), want approval", out, err)
	}
}

func TestFake_RequiresIdempotencyKey(t *testing.T) {
	t.Parallel()
	in := authorizeInput(100, testfixtures.TokenApproved)
	in.IdempotencyKey = ""
	if _, err := acquirer.NewFake().Authorize(context.Background(), in); !errors.Is(err, acquirer.ErrIdempotencyKeyRequired) {
		t.Errorf("err = %v, want ErrIdempotencyKeyRequired", err)
	}
}

func TestFake_ConcurrentSameKeyIsOneAuthorization(t *testing.T) {
	t.Parallel()
	const callers = 16
	fake := acquirer.NewFake()
	in := authorizeInput(100, testfixtures.TokenApproved)

	refs := make([]string, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			out, err := fake.Authorize(context.Background(), in)
			if err != nil {
				t.Errorf("authorize: %v", err)
			}
			refs[i] = out.AcquirerRef
		})
	}
	wg.Wait()
	for _, ref := range refs {
		if ref != refs[0] {
			t.Fatalf("refs differ: %v", refs)
		}
	}
	if n := fake.Authorizations(); n != 1 {
		t.Errorf("authorizations = %d, want 1", n)
	}
}
