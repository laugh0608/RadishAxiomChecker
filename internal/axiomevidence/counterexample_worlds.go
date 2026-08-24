package axiomevidence

import (
	"radishaxiom.dev/independent-checker-go/internal/axiomir"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// VerifyCounterexampleWorlds checks the declaration-level concrete-data
// boundary of every failed result in stable obligation-ID order. It checks
// world cardinality, IR input anchoring, closed records, scalar types and
// ranges, capacity, primary-key uniqueness, and canonical key order. Core and
// host/output failure witnesses must be WF; input-conformance witnesses may
// intentionally expose a WF or Pre failure. This slice does not evaluate assume
// expressions, execute the program, or confirm the target violation.
func (d Document) VerifyCounterexampleWorlds(ir axiomir.Document) error {
	if err := d.VerifyIRSubject(ir.ContentDigest, ir.DomainDigest); err != nil {
		return err
	}

	assumes := ir.AssumeContractIDs()
	obligationIDs := make([]protocol.Digest, 0, len(d.obligations))
	for id := range d.obligations {
		obligationIDs = append(obligationIDs, id)
	}
	sortDigests(obligationIDs)
	for _, id := range obligationIDs {
		definition := d.obligations[id]
		result := d.results[id]
		if result.kind != "failed" {
			continue
		}
		counterexample := result.counterexample
		if counterexample == nil {
			return invalidCounterexample("failed result has no retained counterexample")
		}
		if !validWorldCardinality(counterexample.kind, len(counterexample.worlds)) {
			return invalidCounterexample("counterexample kind has the wrong world cardinality")
		}
		for _, precondition := range counterexample.preconditions {
			if !containsDigest(assumes, precondition) {
				return invalidCounterexample("counterexample precondition is not an IR assume contract")
			}
		}
		if definition.Expectation == "prove" && !equalDigestSets(counterexample.preconditions, assumes) {
			return invalidCounterexample("proof counterexample does not declare the complete IR assume set")
		}

		for _, world := range counterexample.worlds {
			check := ir.CheckInputWorld(world)
			if !check.Anchored {
				return invalidCounterexample("counterexample world does not resolve to the IR input boundary")
			}
			if definition.Kind != "input-conformance" && !check.WellFormed {
				return invalidCounterexample("counterexample world is not WF for a non-input-conformance failure")
			}
		}
	}
	return nil
}

func validWorldCardinality(kind string, count int) bool {
	if kind == "paired-input" {
		return count == 2
	}
	return count == 1
}

func invalidCounterexample(detail string) error {
	return rejection.New(rejection.CounterexampleInvalid, detail)
}
