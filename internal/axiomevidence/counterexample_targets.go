package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// CounterexampleTargetCheck reports only the locked failed-proof targets that
// were independently replayed and the host/output comparisons left for their
// separate slice. Input-conformance failures belong to VerifyConcreteInputs.
type CounterexampleTargetCheck struct {
	ReplayedProofs      int
	DeferredComparisons int
}

// VerifyCounterexampleTargets executes each retained prove+failed witness and
// confirms its exact obligation target, trace, observation, WF, Pre, and
// dynamic violation. It does not compare host/golden output, check minimality,
// proof support, conclusion, or produce an independent four-state result.
func (document Document) VerifyCounterexampleTargets(
	ir axiomir.Document,
	limits axiomir.ExecutionLimits,
) (CounterexampleTargetCheck, error) {
	if err := document.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest); err != nil {
		return CounterexampleTargetCheck{}, err
	}
	if err := document.VerifyCounterexampleWorlds(ir); err != nil {
		return CounterexampleTargetCheck{}, err
	}

	ids := make([]protocol.Digest, 0, len(document.obligations))
	for id := range document.obligations {
		ids = append(ids, id)
	}
	sortDigests(ids)
	result := CounterexampleTargetCheck{}
	for _, id := range ids {
		definition := document.obligations[id]
		obligationResult := document.results[id]
		if obligationResult.kind != "failed" {
			continue
		}
		if definition.Kind == "input-conformance" {
			continue
		}
		if definition.Kind == "host-conformance" || definition.Kind == "output-conformance" {
			result.DeferredComparisons++
			continue
		}
		counterexample := obligationResult.counterexample
		if counterexample == nil {
			return CounterexampleTargetCheck{}, invalidCounterexample("failed proof target has no retained counterexample")
		}
		if counterexample.observed.kind != "obligation-failure" || counterexample.observed.obligation != id {
			return CounterexampleTargetCheck{}, invalidCounterexample("counterexample observation does not bind its failed obligation")
		}
		if !validTargetTrace(counterexample.trace, ir.DomainDigest, id) {
			return CounterexampleTargetCheck{}, invalidCounterexample("counterexample trace does not bind document, obligation, and failed observation")
		}
		violated, err := ir.ReplayFailureTarget(
			definition,
			counterexample.worlds,
			counterexample.observed.requiredFields,
			counterexample.observed.requiredKeys,
			limits,
		)
		if err != nil {
			if code, ok := rejection.CodeOf(err); ok && code == rejection.ResourceLimit {
				return CounterexampleTargetCheck{}, err
			}
			return CounterexampleTargetCheck{}, invalidCounterexample("counterexample target replay could not confirm the declared violation")
		}
		if !violated {
			return CounterexampleTargetCheck{}, invalidCounterexample("counterexample target does not violate its declared obligation")
		}
		result.ReplayedProofs++
	}
	return result, nil
}

func validTargetTrace(trace []counterexampleTraceStep, document, obligation protocol.Digest) bool {
	return len(trace) == 3 &&
		trace[0].kind == "document" && trace[0].ref == document &&
		trace[1].kind == "obligation" && trace[1].ref == obligation &&
		trace[2].kind == "observation" && trace[2].value == "failed"
}
