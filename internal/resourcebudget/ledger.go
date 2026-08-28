package resourcebudget

import (
	"fmt"
	"time"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

const digestBlockBytes uint64 = 64 << 10

// Limits is the request-scoped limit set consumed by one checker invocation.
// ArtifactBytes remains a per-artifact preflight limit and is therefore
// enforced by the bundle reader rather than the cumulative ledger.
type Limits struct {
	BundleBytes     uint64
	CollectionItems uint64
	JSONDepth       uint64
	SemanticSteps   uint64
	WallClockMillis uint64
	WorkingMemory   uint64
}

// Snapshot is a deterministic observation of counters that are independent of
// host heap layout and wall-clock scheduling.
type Snapshot struct {
	BundleBytes     uint64
	CollectionItems uint64
	DigestBlocks    uint64
	JSONDepth       uint64
	SemanticSteps   uint64
	WorkingMemory   uint64
}

// Ledger accumulates every request-internal resource counter for one
// invocation. Before Activate, counters are recorded under implementation hard
// caps while request, Evidence, and checker identities are being established;
// a request-limit exhaustion becomes observable at Activate and is never reset.
type Ledger struct {
	now        func() time.Time
	started    time.Time
	limits     Limits
	configured bool
	active     bool
	exhausted  error

	bundleBytes     uint64
	collectionItems uint64
	digestBlocks    uint64
	jsonDepth       uint64
	semanticSteps   uint64
	workingMemory   uint64
	nextTimeStep    uint64

	artifacts map[string]uint64
	owners    map[string]uint64
}

func New(now func() time.Time) *Ledger {
	if now == nil {
		now = time.Now
	}
	return &Ledger{
		now: now, started: now(), nextTimeStep: 1024,
		artifacts: make(map[string]uint64), owners: make(map[string]uint64),
	}
}

func (ledger *Ledger) Configure(limits Limits) error {
	if ledger == nil {
		return fmt.Errorf("resource ledger is nil")
	}
	if ledger.configured {
		return fmt.Errorf("resource ledger was configured more than once")
	}
	if limits.BundleBytes == 0 || limits.CollectionItems == 0 || limits.JSONDepth == 0 ||
		limits.SemanticSteps == 0 || limits.WallClockMillis == 0 || limits.WorkingMemory == 0 {
		return fmt.Errorf("resource ledger limits must all be positive")
	}
	ledger.limits = limits
	ledger.configured = true
	ledger.observeLimits()
	return nil
}

// Activate begins immediate enforcement after the identities required for a
// canonical result are available. Counters accumulated during identity
// formation are retained and can make this call fail with resource-limit.
func (ledger *Ledger) Activate() error {
	if ledger == nil || !ledger.configured {
		return fmt.Errorf("resource ledger is not configured")
	}
	ledger.active = true
	ledger.observeLimits()
	if ledger.exhausted != nil {
		return ledger.exhausted
	}
	return ledger.Checkpoint()
}

func (ledger *Ledger) ChargeArtifact(key string, bytes uint64) error {
	if ledger == nil {
		return nil
	}
	if key == "" {
		return fmt.Errorf("resource artifact key is empty")
	}
	if previous, ok := ledger.artifacts[key]; ok {
		if previous != bytes {
			return fmt.Errorf("resource artifact length changed for one identity")
		}
		return ledger.currentError()
	}
	ledger.artifacts[key] = bytes
	ledger.bundleBytes = saturatingAdd(ledger.bundleBytes, bytes)
	ledger.observeLimits()
	return ledger.currentError()
}

func (ledger *Ledger) ChargeDigestBytes(bytes uint64) error {
	if ledger == nil || bytes == 0 {
		return nil
	}
	blocks := 1 + (bytes-1)/digestBlockBytes
	ledger.digestBlocks = saturatingAdd(ledger.digestBlocks, blocks)
	return ledger.ChargeSemanticSteps(blocks)
}

func (ledger *Ledger) ChargeCollectionItems(items uint64) error {
	if ledger == nil || items == 0 {
		return nil
	}
	ledger.collectionItems = saturatingAdd(ledger.collectionItems, items)
	ledger.observeLimits()
	return ledger.currentError()
}

func (ledger *Ledger) ObserveJSONDepth(depth uint64) error {
	if ledger == nil {
		return nil
	}
	if depth > ledger.jsonDepth {
		ledger.jsonDepth = depth
	}
	ledger.observeLimits()
	return ledger.currentError()
}

func (ledger *Ledger) ChargeSemanticSteps(steps uint64) error {
	if ledger == nil || steps == 0 {
		return nil
	}
	ledger.semanticSteps = saturatingAdd(ledger.semanticSteps, steps)
	ledger.observeLimits()
	if ledger.active && ledger.semanticSteps >= ledger.nextTimeStep {
		ledger.nextTimeStep = nextCheckpoint(ledger.semanticSteps)
		if err := ledger.Checkpoint(); err != nil {
			return err
		}
	}
	return ledger.currentError()
}

// AcquireLogical records deterministic logical ownership. Reusing an owner
// replaces its previous charge, while ReleaseLogical removes ownership only
// when the corresponding budget object has been discarded.
func (ledger *Ledger) AcquireLogical(owner string, bytes uint64) error {
	if ledger == nil {
		return nil
	}
	if owner == "" {
		return fmt.Errorf("logical-memory owner is empty")
	}
	previous := ledger.owners[owner]
	if previous > ledger.workingMemory {
		return fmt.Errorf("logical-memory ownership is inconsistent")
	}
	ledger.workingMemory -= previous
	ledger.workingMemory = saturatingAdd(ledger.workingMemory, bytes)
	ledger.owners[owner] = bytes
	ledger.observeLimits()
	return ledger.currentError()
}

func (ledger *Ledger) ReleaseLogical(owner string) {
	if ledger == nil {
		return
	}
	if previous, ok := ledger.owners[owner]; ok {
		ledger.workingMemory -= previous
		delete(ledger.owners, owner)
	}
}

func (ledger *Ledger) Checkpoint() error {
	if ledger == nil || !ledger.active {
		return nil
	}
	if ledger.exhausted != nil {
		return ledger.exhausted
	}
	elapsed := ledger.now().Sub(ledger.started)
	if elapsed < 0 || elapsed >= time.Duration(ledger.limits.WallClockMillis)*time.Millisecond {
		ledger.exhaust("checker invocation exceeded its internal wall-clock limit")
	}
	return ledger.exhausted
}

func (ledger *Ledger) Snapshot() Snapshot {
	if ledger == nil {
		return Snapshot{}
	}
	return Snapshot{
		BundleBytes: ledger.bundleBytes, CollectionItems: ledger.collectionItems,
		DigestBlocks: ledger.digestBlocks, JSONDepth: ledger.jsonDepth,
		SemanticSteps: ledger.semanticSteps, WorkingMemory: ledger.workingMemory,
	}
}

func (ledger *Ledger) observeLimits() {
	if !ledger.configured || ledger.exhausted != nil {
		return
	}
	switch {
	case ledger.bundleBytes > ledger.limits.BundleBytes:
		ledger.exhaust("checker invocation exceeded its cumulative bundle byte limit")
	case ledger.collectionItems > ledger.limits.CollectionItems:
		ledger.exhaust("checker invocation exceeded its cumulative collection item limit")
	case ledger.jsonDepth > ledger.limits.JSONDepth:
		ledger.exhaust("checker invocation exceeded its JSON depth limit")
	case ledger.semanticSteps > ledger.limits.SemanticSteps:
		ledger.exhaust("checker invocation exceeded its cumulative semantic step limit")
	case ledger.workingMemory > ledger.limits.WorkingMemory:
		ledger.exhaust("checker invocation exceeded its logical working-memory limit")
	}
}

func (ledger *Ledger) currentError() error {
	if ledger == nil || !ledger.active {
		return nil
	}
	return ledger.exhausted
}

func (ledger *Ledger) exhaust(detail string) {
	if ledger.exhausted == nil {
		ledger.exhausted = rejection.New(rejection.ResourceLimit, detail)
	}
}

func nextCheckpoint(steps uint64) uint64 {
	if steps > ^uint64(0)-1024 {
		return ^uint64(0)
	}
	return (steps/1024 + 1) * 1024
}

func saturatingAdd(left, right uint64) uint64 {
	result := left + right
	if result < left {
		return ^uint64(0)
	}
	return result
}
