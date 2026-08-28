package resourcebudget

import (
	"testing"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

func TestLedgerDefersRequestLimitUntilIdentityActivation(t *testing.T) {
	now := time.Unix(0, 0)
	ledger := New(func() time.Time { return now })
	if err := ledger.ChargeSemanticSteps(2); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Configure(Limits{
		BundleBytes: 100, CollectionItems: 100, JSONDepth: 10,
		SemanticSteps: 1, WallClockMillis: 1000, WorkingMemory: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ChargeSemanticSteps(1); err != nil {
		t.Fatal("identity formation exposed a request-level resource result too early")
	}
	assertResourceLimit(t, ledger.Activate())
}

func TestLedgerCountsUniqueArtifactsAndDigestBlocks(t *testing.T) {
	ledger := New(nil)
	if err := ledger.Configure(Limits{
		BundleBytes: 10, CollectionItems: 100, JSONDepth: 10,
		SemanticSteps: 100, WallClockMillis: 1000, WorkingMemory: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ChargeArtifact("a", 6); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ChargeArtifact("a", 6); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ChargeDigestBytes((64 << 10) + 1); err != nil {
		t.Fatal(err)
	}
	if got := ledger.Snapshot(); got.BundleBytes != 6 || got.DigestBlocks != 2 || got.SemanticSteps != 2 {
		t.Fatalf("unexpected cumulative snapshot: %+v", got)
	}
	if err := ledger.ChargeArtifact("b", 5); err != nil {
		t.Fatal("limit must remain deferred until activation")
	}
	assertResourceLimit(t, ledger.Activate())
}

func TestLedgerChecksMonotonicDeadlineAtSemanticBoundary(t *testing.T) {
	now := time.Unix(0, 0)
	ledger := New(func() time.Time { return now })
	if err := ledger.Configure(Limits{
		BundleBytes: 100, CollectionItems: 100, JSONDepth: 10,
		SemanticSteps: 5000, WallClockMillis: 5, WorkingMemory: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Activate(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(5 * time.Millisecond)
	assertResourceLimit(t, ledger.ChargeSemanticSteps(1024))
}

func TestLedgerLogicalOwnershipCanBeReleased(t *testing.T) {
	ledger := New(nil)
	if err := ledger.Configure(Limits{
		BundleBytes: 100, CollectionItems: 100, JSONDepth: 10,
		SemanticSteps: 100, WallClockMillis: 1000, WorkingMemory: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.AcquireLogical("document", 8); err != nil {
		t.Fatal(err)
	}
	ledger.ReleaseLogical("document")
	if got := ledger.Snapshot().WorkingMemory; got != 0 {
		t.Fatalf("logical bytes after release = %d, want 0", got)
	}
	if err := ledger.Activate(); err != nil {
		t.Fatal(err)
	}
}

func assertResourceLimit(t *testing.T, err error) {
	t.Helper()
	code, ok := rejection.CodeOf(err)
	if !ok || code != rejection.ResourceLimit {
		t.Fatalf("error = %v, want resource-limit", err)
	}
}
