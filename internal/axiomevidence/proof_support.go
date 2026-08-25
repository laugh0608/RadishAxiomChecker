package axiomevidence

import (
	"bytes"
	"crypto/sha256"
	"sort"
	"unicode/utf8"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	ProofCoverageIndependent = "independently-verified"
	ProofCoverageAttestation = "attestation-only"
	ProofCoverageMissing     = "missing-proof-material"
)

// ProofSupportLimits binds proof artifact parsing and inspection to the same
// explicit request budgets used by the concrete-data verification slices.
type ProofSupportLimits = ConcreteDataLimits

// ProofSupportCapabilities states only proof material the independent checker
// can itself replay. Empty sets are meaningful: a producer's kernel-replay tag
// cannot expand this compiled capability boundary.
type ProofSupportCapabilities struct {
	CertificateProfiles []string
	KernelRuleProfiles  []string
}

// ProofSupportFinding keeps the producer claim, inspected execution material,
// and independent coverage distinct. QueryTheoremVerified remains false until
// a future checker profile can reconstruct and compare the encoded theorem.
type ProofSupportFinding struct {
	Obligation               protocol.Digest
	Execution                protocol.Digest
	ObligationSet            protocol.Digest
	Query                    protocol.Digest
	Response                 protocol.Digest
	Tool                     protocol.Digest
	ToolArtifact             protocol.Digest
	Trust                    protocol.Digest
	SupportKind              string
	Coverage                 string
	MissingReason            string
	QueryLogic               string
	ResponseStatus           string
	ToolName                 string
	ToolVersion              string
	ProofPolicySatisfied     bool
	TargetInObligationSet    bool
	QueryTheoremVerified     bool
	BackendTrustScopeMatched bool
}

// ProofSupportCheck is an audit summary, not an independent four-state result.
// MissingProofMaterial is deliberately explicit so unsupported producer claims
// cannot become successful proof checks through a zero value or completed
// execution status.
type ProofSupportCheck struct {
	Capabilities          ProofSupportCapabilities
	Findings              []ProofSupportFinding
	Artifacts             []protocol.Digest
	RemainingTrust        []protocol.Digest
	Claims                int
	IndependentlyVerified int
	AttestationsConfirmed int
	UnsupportedClaims     int
	ProofPolicySatisfied  int
	MissingProofMaterial  int
}

// CurrentProofSupportCapabilities returns a defensive copy of the proof
// capabilities compiled into this checker source snapshot. The locked v0.1
// implementation has accepted neither a kernel-rule nor certificate profile.
func CurrentProofSupportCapabilities() ProofSupportCapabilities {
	return ProofSupportCapabilities{
		CertificateProfiles: []string{},
		KernelRuleProfiles:  []string{},
	}
}

type inspectedProofExecution struct {
	obligationSet protocol.Digest
	query         protocol.Digest
	response      protocol.Digest
	tool          protocol.Digest
	toolArtifact  protocol.Digest
	toolName      string
	toolVersion   string
	logic         string
	status        string
}

// InspectProofSupports reopens and authenticates the complete artifact boundary
// of every proved result, compares the production obligation-set with the
// already checked Evidence obligations, validates the closed SMT query envelope
// and response status, and checks attestation trust scope against the exact
// prover tool. It never treats an unsat status, a completed execution, or the
// producer's kernel-replay tag as an independent proof.
func (document Document) InspectProofSupports(
	policy protocol.AssurancePolicy,
	read ArtifactReader,
	limits ProofSupportLimits,
) (ProofSupportCheck, error) {
	if policy.ProofSupport != "attestation-allowed" && policy.ProofSupport != "certificate-required" {
		return ProofSupportCheck{}, proofSupportMismatch("proof support policy is outside the closed request profile")
	}

	obligationIDs := make([]protocol.Digest, 0)
	for id, result := range document.results {
		if result.kind == "proved" {
			obligationIDs = append(obligationIDs, id)
		}
	}
	sortDigests(obligationIDs)
	check := ProofSupportCheck{
		Capabilities: CurrentProofSupportCapabilities(),
		Claims:       len(obligationIDs),
	}
	if len(obligationIDs) == 0 {
		return check, nil
	}
	if read == nil {
		return ProofSupportCheck{}, rejection.New(rejection.ArtifactMissing, "proof support artifact reader is unavailable")
	}

	loader := proofArtifactLoader{
		document: document,
		read:     read,
		limits:   limits,
		cache:    make(map[protocol.Digest][]byte),
	}
	executions := make(map[protocol.Digest]inspectedProofExecution)
	remainingTrust := make(map[protocol.Digest]struct{})
	for _, obligationID := range obligationIDs {
		result, ok := document.results[obligationID]
		if !ok {
			return ProofSupportCheck{}, proofSupportMismatch("proved obligation result is unavailable")
		}
		if err := document.verifyProofSupport(result); err != nil {
			return ProofSupportCheck{}, err
		}
		inspection, ok := executions[result.support.execution]
		if !ok {
			var err error
			inspection, err = document.inspectProofExecution(result.support.execution, &loader, limits.JSON)
			if err != nil {
				return ProofSupportCheck{}, err
			}
			executions[result.support.execution] = inspection
		}

		finding := ProofSupportFinding{
			Obligation:            obligationID,
			Execution:             result.support.execution,
			ObligationSet:         inspection.obligationSet,
			Query:                 inspection.query,
			Response:              inspection.response,
			Tool:                  inspection.tool,
			ToolArtifact:          inspection.toolArtifact,
			SupportKind:           result.support.kind,
			QueryLogic:            inspection.logic,
			ResponseStatus:        inspection.status,
			ToolName:              inspection.toolName,
			ToolVersion:           inspection.toolVersion,
			TargetInObligationSet: true,
			QueryTheoremVerified:  false,
		}

		switch result.support.kind {
		case "kernel-replay":
			finding.Coverage = ProofCoverageMissing
			finding.MissingReason = "kernel-replay-material-unavailable"
			check.UnsupportedClaims++
			check.MissingProofMaterial++
		case "backend-attestation":
			if inspection.status != "unsat" {
				return ProofSupportCheck{}, proofSupportMismatch("proved backend attestation response status is not unsat")
			}
			trust := document.trust[result.support.trust]
			if trust.scopeKind != "tool" || trust.scopeTool != inspection.tool {
				return ProofSupportCheck{}, proofSupportMismatch("backend attestation trust does not scope the exact prover tool")
			}
			finding.Trust = result.support.trust
			finding.Coverage = ProofCoverageAttestation
			finding.BackendTrustScopeMatched = true
			check.AttestationsConfirmed++
			remainingTrust[result.support.trust] = struct{}{}
			if policy.ProofSupport == "attestation-allowed" {
				finding.ProofPolicySatisfied = true
				check.ProofPolicySatisfied++
			} else {
				finding.MissingReason = "certificate-required-attestation-only"
				check.MissingProofMaterial++
			}
		default:
			return ProofSupportCheck{}, proofSupportMismatch("proved result has an unsupported proof support kind")
		}
		check.Findings = append(check.Findings, finding)
	}

	for artifact := range loader.cache {
		check.Artifacts = append(check.Artifacts, artifact)
	}
	sortDigests(check.Artifacts)
	for trust := range remainingTrust {
		check.RemainingTrust = append(check.RemainingTrust, trust)
	}
	sortDigests(check.RemainingTrust)
	sortProofFindings(check.Findings)
	return check, nil
}

func (document Document) inspectProofExecution(
	executionID protocol.Digest,
	loader *proofArtifactLoader,
	jsonLimits strictjson.Limits,
) (inspectedProofExecution, error) {
	execution, ok := document.executions[executionID]
	if !ok || execution.kind != "prove" || execution.result.kind != "completed" {
		return inspectedProofExecution{}, proofSupportMismatch("proof support does not bind a completed prove execution")
	}
	obligationSet, setOK := singleExecutionArtifact(execution.inputs, "obligation-set")
	query, queryOK := singleExecutionArtifact(execution.inputs, "query")
	response, responseOK := singleExecutionArtifact(execution.outputs, "response")
	if !setOK || !queryOK || !responseOK || len(execution.inputs) != 2 || len(execution.outputs) != 1 {
		return inspectedProofExecution{}, proofSupportMismatch("prove execution has an invalid artifact boundary")
	}
	if err := document.requireProofArtifact(obligationSet, "axiom-obligation-set", "0.1"); err != nil {
		return inspectedProofExecution{}, err
	}
	if err := document.requireProofArtifact(query, "axiom-smtlib2-qf-uflia-query", "0.1"); err != nil {
		return inspectedProofExecution{}, err
	}
	if err := document.requireProofArtifact(response, "cvc5-response", "1.3.4"); err != nil {
		return inspectedProofExecution{}, err
	}
	tool, ok := document.tools[execution.tool]
	if !ok {
		return inspectedProofExecution{}, proofSupportMismatch("prove execution tool is unavailable")
	}
	if _, ok := tool.roles["prover"]; !ok {
		return inspectedProofExecution{}, proofSupportMismatch("prove execution tool lacks the prover role")
	}

	setBytes, err := loader.load(obligationSet)
	if err != nil {
		return inspectedProofExecution{}, err
	}
	if err := document.verifyProofObligationSet(setBytes, jsonLimits); err != nil {
		return inspectedProofExecution{}, err
	}
	queryBytes, err := loader.load(query)
	if err != nil {
		return inspectedProofExecution{}, err
	}
	logic, err := inspectProofQuery(queryBytes)
	if err != nil {
		return inspectedProofExecution{}, err
	}
	responseBytes, err := loader.load(response)
	if err != nil {
		return inspectedProofExecution{}, err
	}
	status, err := inspectProofResponse(responseBytes)
	if err != nil {
		return inspectedProofExecution{}, err
	}
	if _, err := loader.load(tool.artifact); err != nil {
		return inspectedProofExecution{}, err
	}
	return inspectedProofExecution{
		obligationSet: obligationSet,
		query:         query,
		response:      response,
		tool:          execution.tool,
		toolArtifact:  tool.artifact,
		toolName:      tool.name,
		toolVersion:   tool.version,
		logic:         logic,
		status:        status,
	}, nil
}

func (document Document) requireProofArtifact(id protocol.Digest, format, version string) error {
	definition, ok := document.artifacts[id]
	if !ok || definition.format != format || definition.formatVersion != version {
		return proofSupportMismatch("proof execution artifact format or version does not match its role")
	}
	return nil
}

func (document Document) verifyProofObligationSet(data []byte, limits strictjson.Limits) error {
	value, err := strictjson.ParseCanonical(data, limits)
	if err != nil {
		return err
	}
	root, err := object(value,
		"format", "format_version", "ir_artifact", "ir_document_digest",
		"obligation_profile", "obligations", "semantics",
	)
	if err != nil {
		return err
	}
	if err := requireText(root["format"], "axiom-obligation-set", rejection.UnknownTag, "unknown proof obligation-set format"); err != nil {
		return err
	}
	if err := requireText(root["format_version"], "0.1", rejection.UnsupportedVersion, "unsupported proof obligation-set version"); err != nil {
		return err
	}
	irArtifact, err := digest(root["ir_artifact"])
	if err != nil {
		return err
	}
	irDocument, err := digest(root["ir_document_digest"])
	if err != nil {
		return err
	}
	if irArtifact != document.irArtifact || irDocument != document.irDocumentDigest {
		return proofSupportMismatch("proof obligation-set does not bind the Evidence IR subject")
	}
	profile, err := object(root["obligation_profile"], "name", "version")
	if err != nil {
		return err
	}
	if err := requireText(profile["name"], document.obligationProfile, rejection.UnknownTag, "proof obligation-set profile differs from Evidence"); err != nil {
		return err
	}
	if err := requireText(profile["version"], "0.1", rejection.UnsupportedVersion, "unsupported proof obligation-set profile version"); err != nil {
		return err
	}
	semantics, err := object(root["semantics"], "name", "sha256")
	if err != nil {
		return err
	}
	if err := requireText(semantics["name"], "keyed-finite-table-semantics", rejection.UnknownTag, "proof obligation-set semantics differs from Evidence"); err != nil {
		return err
	}
	if err := requireText(semantics["sha256"], semanticsSHA256, rejection.DigestMismatch, "proof obligation-set semantics digest mismatch"); err != nil {
		return err
	}

	items, err := array(root["obligations"])
	if err != nil {
		return err
	}
	if len(items) != len(document.obligations) {
		return proofSupportMismatch("proof obligation-set cardinality differs from checked Evidence obligations")
	}
	artifactParser := parser{
		artifactRefs: make(map[protocol.Digest]struct{}),
		documentRefs: make(map[protocol.Digest]struct{}),
		trustRefs:    make(map[protocol.Digest]struct{}),
	}
	var previous string
	for index, item := range items {
		fields, err := object(item, "definition", "id")
		if err != nil {
			return err
		}
		spelling, err := text(fields["id"])
		if err != nil {
			return err
		}
		if err := requireStrictOrder(previous, spelling, index != 0, "proof obligation-set is not sorted and unique by ID"); err != nil {
			return err
		}
		id, err := protocol.ParseDigest(spelling)
		if err != nil {
			return err
		}
		definition, err := artifactParser.parseObligationDefinition(fields["definition"])
		if err != nil {
			return err
		}
		if err := verifyDefinitionID(domainObligation, fields["definition"], id); err != nil {
			return err
		}
		expected, ok := document.obligations[id]
		if !ok || !equalObligationDefinition(expected, definition) {
			return proofSupportMismatch("proof obligation-set differs from checked Evidence obligations")
		}
		previous = spelling
	}
	return nil
}

func inspectProofQuery(data []byte) (string, error) {
	if len(data) == 0 || !utf8.Valid(data) || data[len(data)-1] != '\n' {
		return "", proofSupportMismatch("proof query is empty, non-UTF-8, or lacks its final newline")
	}
	commands := make([][]byte, 0, 8)
	for position := 0; position < len(data); {
		for position < len(data) && (data[position] == ' ' || data[position] == '\n' || data[position] == '\t') {
			position++
		}
		if position == len(data) {
			break
		}
		if data[position] != '(' {
			return "", proofSupportMismatch("proof query contains bytes outside top-level SMT commands")
		}
		start := position
		depth := 0
		for ; position < len(data); position++ {
			current := data[position]
			if current == '\r' || current < 0x20 && current != '\n' && current != '\t' || current > 0x7e {
				return "", proofSupportMismatch("proof query is outside the closed ASCII SMT envelope")
			}
			if current == ';' || current == '"' || current == '|' {
				return "", proofSupportMismatch("proof query uses an unsupported SMT lexical form")
			}
			switch current {
			case '(':
				depth++
			case ')':
				depth--
				if depth < 0 {
					return "", proofSupportMismatch("proof query parentheses are unbalanced")
				}
				if depth == 0 {
					position++
					commands = append(commands, data[start:position])
					goto commandComplete
				}
			}
		}
		return "", proofSupportMismatch("proof query parentheses are unbalanced")
	commandComplete:
	}
	if len(commands) < 3 || !bytes.Equal(commands[0], []byte("(set-logic QF_UFLIA)")) ||
		!bytes.Equal(commands[len(commands)-1], []byte("(check-sat)")) {
		return "", proofSupportMismatch("proof query does not bind the closed QF_UFLIA single-check envelope")
	}
	allowed := map[string]struct{}{
		"assert": {}, "check-sat": {}, "declare-const": {}, "declare-fun": {}, "set-logic": {},
	}
	setLogic := 0
	checkSat := 0
	assertions := 0
	for _, command := range commands {
		head := smtCommandHead(command)
		if _, ok := allowed[head]; !ok {
			return "", proofSupportMismatch("proof query contains an unsupported top-level SMT command")
		}
		switch head {
		case "set-logic":
			setLogic++
		case "check-sat":
			checkSat++
		case "assert":
			assertions++
		}
	}
	if setLogic != 1 || checkSat != 1 || assertions == 0 {
		return "", proofSupportMismatch("proof query has an invalid logic, assertion, or status-command cardinality")
	}
	return "QF_UFLIA", nil
}

func smtCommandHead(command []byte) string {
	start := 1
	for start < len(command) && (command[start] == ' ' || command[start] == '\n' || command[start] == '\t') {
		start++
	}
	end := start
	for end < len(command) && command[end] != ')' && command[end] != ' ' && command[end] != '\n' && command[end] != '\t' {
		end++
	}
	return string(command[start:end])
}

func inspectProofResponse(data []byte) (string, error) {
	switch string(data) {
	case "unsat\n":
		return "unsat", nil
	case "sat\n":
		return "sat", nil
	case "unknown\n":
		return "unknown", nil
	default:
		return "", proofSupportMismatch("proof response is not one closed cvc5 status frame")
	}
}

type proofArtifactLoader struct {
	document     Document
	read         ArtifactReader
	limits       ProofSupportLimits
	cache        map[protocol.Digest][]byte
	logicalBytes uint64
	steps        uint64
}

func (loader *proofArtifactLoader) load(id protocol.Digest) ([]byte, error) {
	if data, ok := loader.cache[id]; ok {
		return data, nil
	}
	if _, ok := loader.document.artifacts[id]; !ok {
		return nil, proofSupportMismatch("proof support references an unknown artifact")
	}
	data, err := loader.read(id)
	if err != nil {
		return nil, err
	}
	if uint64(len(data)) > loader.limits.JSON.MaxBytes {
		return nil, rejection.New(rejection.ResourceLimit, "proof support artifact exceeds its byte limit")
	}
	if protocol.Digest(sha256.Sum256(data)) != id {
		return nil, rejection.New(rejection.DigestMismatch, "proof support bytes do not match their artifact digest")
	}
	charge := uint64(len(data)) + 1
	if loader.limits.MaxLogicalBytes == 0 || charge > loader.limits.MaxLogicalBytes-loader.logicalBytes {
		return nil, rejection.New(rejection.ResourceLimit, "proof support artifacts exceed their logical-memory limit")
	}
	if loader.limits.MaxSemanticSteps == 0 || charge > loader.limits.MaxSemanticSteps-loader.steps {
		return nil, rejection.New(rejection.ResourceLimit, "proof support inspection exceeds its semantic-step limit")
	}
	loader.logicalBytes += charge
	loader.steps += charge
	loader.cache[id] = data
	return data, nil
}

func proofSupportMismatch(detail string) error {
	return rejection.New(rejection.InvalidStateSupport, detail)
}

func sortProofFindings(findings []ProofSupportFinding) {
	sort.Slice(findings, func(left, right int) bool {
		return findings[left].Obligation.String() < findings[right].Obligation.String()
	})
}
