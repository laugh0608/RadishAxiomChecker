package checkresult

import (
	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

// ParseCompanion strictly parses, recomputes, and self-checks one canonical
// axiom-independent-check-result v0.1 document.
func ParseCompanion(raw []byte, limits strictjson.Limits) (CompanionDocument, error) {
	value, err := strictjson.ParseCanonical(raw, limits)
	if err != nil {
		return CompanionDocument{}, err
	}
	root, err := companionObject(value, []string{
		"checker", "checks", "evidence", "missing_artifacts", "remaining_trust",
		"request", "result", "result_version", "tcb",
	})
	if err != nil {
		return CompanionDocument{}, err
	}
	version, err := companionText(root["result_version"])
	if err != nil {
		return CompanionDocument{}, err
	}
	if version != resultVersion {
		return CompanionDocument{}, rejection.New(rejection.UnsupportedVersion, "unsupported result version")
	}
	checker, err := parseCompanionChecker(root["checker"])
	if err != nil {
		return CompanionDocument{}, err
	}
	checks, err := parseCompanionChecks(root["checks"])
	if err != nil {
		return CompanionDocument{}, err
	}
	evidence, err := parseCompanionDocumentIdentity(root["evidence"])
	if err != nil {
		return CompanionDocument{}, err
	}
	missing, err := parseCompanionDigestArray(root["missing_artifacts"], false, "missing artifacts")
	if err != nil {
		return CompanionDocument{}, err
	}
	trust, err := parseCompanionDigestArray(root["remaining_trust"], false, "remaining trust")
	if err != nil {
		return CompanionDocument{}, err
	}
	request, err := parseCompanionDocumentIdentity(root["request"])
	if err != nil {
		return CompanionDocument{}, err
	}
	outcome, err := parseCompanionOutcome(root["result"])
	if err != nil {
		return CompanionDocument{}, err
	}
	tcb, err := parseCompanionTCB(root["tcb"])
	if err != nil {
		return CompanionDocument{}, err
	}
	document := CompanionDocument{
		Checker:          checker,
		Checks:           checks,
		Evidence:         evidence,
		MissingArtifacts: missing,
		RemainingTrust:   trust,
		Request:          request,
		Outcome:          outcome,
		TCB:              tcb,
		DomainDigest:     companionDomainDigest(raw),
		raw:              append([]byte(nil), raw...),
	}
	if err := validateCompanionDocument(document); err != nil {
		return CompanionDocument{}, err
	}
	return document, nil
}

func parseCompanionChecker(value strictjson.Value) (CompanionChecker, error) {
	fields, err := companionObject(value, []string{"artifact", "name", "source", "toolchain", "version"})
	if err != nil {
		return CompanionChecker{}, err
	}
	artifact, err := companionDigest(fields["artifact"])
	if err != nil {
		return CompanionChecker{}, err
	}
	name, err := companionText(fields["name"])
	if err != nil {
		return CompanionChecker{}, err
	}
	if name != checkerName {
		return CompanionChecker{}, rejection.New(rejection.CheckerIdentity, "result does not name the independent Go checker")
	}
	source, err := companionDigest(fields["source"])
	if err != nil {
		return CompanionChecker{}, err
	}
	toolchain, err := companionText(fields["toolchain"])
	if err != nil {
		return CompanionChecker{}, err
	}
	if toolchain != checkerToolchain {
		return CompanionChecker{}, rejection.New(rejection.CheckerIdentity, "result does not bind the exact checker toolchain")
	}
	version, err := companionText(fields["version"])
	if err != nil {
		return CompanionChecker{}, err
	}
	if !validVersion(version) {
		return CompanionChecker{}, rejection.New(rejection.CheckerIdentity, "result does not bind an exact checker version")
	}
	if artifact == source {
		return CompanionChecker{}, rejection.New(rejection.CheckerIdentity, "checker source cannot substitute for checker artifact")
	}
	return CompanionChecker{Artifact: artifact, Name: name, Source: source, Toolchain: toolchain, Version: version}, nil
}

func parseCompanionChecks(value strictjson.Value) ([]Check, error) {
	items, err := companionArray(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, rejection.New(rejection.MissingRequiredMember, "result must contain at least one check")
	}
	result := make([]Check, 0, len(items))
	seenKinds := make(map[CheckKind]struct{}, len(items))
	var previous protocol.Digest
	for index, item := range items {
		fields, err := companionObject(item, []string{"definition", "id"})
		if err != nil {
			return nil, err
		}
		definition, err := parseCompanionDefinition(fields["definition"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenKinds[definition.Kind]; duplicate {
			return nil, rejection.New(rejection.ResultAggregation, "result contains duplicate check kind")
		}
		seenKinds[definition.Kind] = struct{}{}
		id, err := companionDigest(fields["id"])
		if err != nil {
			return nil, err
		}
		if index > 0 && previous.String() >= id.String() {
			return nil, rejection.New(rejection.NoncanonicalOrder, "checks are not sorted and unique by ID")
		}
		check, err := NewCheck(definition.Kind, definition.Outcome, definition.Codes, definition.Refs)
		if err != nil {
			return nil, rejection.New(rejection.UnknownTag, "check definition contains an unsupported value")
		}
		if check.ID != id {
			return nil, rejection.New(rejection.CheckIDMismatch, "check ID does not match its canonical definition")
		}
		previous = id
		result = append(result, check)
	}
	return result, nil
}

func parseCompanionDefinition(value strictjson.Value) (CheckDefinition, error) {
	fields, err := companionObject(value, []string{"codes", "kind", "outcome", "refs"})
	if err != nil {
		return CheckDefinition{}, err
	}
	codesValues, err := companionArray(fields["codes"])
	if err != nil {
		return CheckDefinition{}, err
	}
	if len(codesValues) == 0 {
		return CheckDefinition{}, rejection.New(rejection.MissingRequiredMember, "check codes cannot be empty")
	}
	codes := make([]string, 0, len(codesValues))
	for index, item := range codesValues {
		code, err := companionText(item)
		if err != nil {
			return CheckDefinition{}, err
		}
		if index > 0 && codes[index-1] >= code {
			return CheckDefinition{}, rejection.New(rejection.NoncanonicalOrder, "check codes are not sorted and unique")
		}
		codes = append(codes, code)
	}
	kindText, err := companionText(fields["kind"])
	if err != nil {
		return CheckDefinition{}, err
	}
	kind := CheckKind(kindText)
	allowedCodes, ok := checkCodeRegistry[kind]
	if !ok {
		return CheckDefinition{}, rejection.New(rejection.UnknownTag, "unknown check kind")
	}
	for _, code := range codes {
		if _, ok := allowedCodes[code]; !ok {
			return CheckDefinition{}, rejection.New(rejection.UnknownTag, "unknown code for check kind")
		}
	}
	outcomeText, err := companionText(fields["outcome"])
	if err != nil {
		return CheckDefinition{}, err
	}
	outcome := CheckOutcome(outcomeText)
	switch outcome {
	case CheckIncomplete, CheckPassed, CheckRejected, CheckTrusted:
	default:
		return CheckDefinition{}, rejection.New(rejection.UnknownTag, "unknown check outcome")
	}
	refValues, err := companionArray(fields["refs"])
	if err != nil {
		return CheckDefinition{}, err
	}
	refs := make([]Ref, 0, len(refValues))
	for index, item := range refValues {
		refFields, err := companionObject(item, []string{"kind", "ref"})
		if err != nil {
			return CheckDefinition{}, err
		}
		refKindText, err := companionText(refFields["kind"])
		if err != nil {
			return CheckDefinition{}, err
		}
		refKind := RefKind(refKindText)
		if !validRefKind(refKind) {
			return CheckDefinition{}, rejection.New(rejection.UnknownTag, "unknown check reference kind")
		}
		refID, err := companionDigest(refFields["ref"])
		if err != nil {
			return CheckDefinition{}, err
		}
		ref := Ref{Kind: refKind, ID: refID}
		if index > 0 && compareRef(refs[index-1], ref) >= 0 {
			return CheckDefinition{}, rejection.New(rejection.NoncanonicalOrder, "check refs are not sorted and unique")
		}
		refs = append(refs, ref)
	}
	return CheckDefinition{Codes: codes, Kind: kind, Outcome: outcome, Refs: refs}, nil
}

func parseCompanionDocumentIdentity(value strictjson.Value) (DocumentIdentity, error) {
	fields, err := companionObject(value, []string{"content_digest", "document_digest"})
	if err != nil {
		return DocumentIdentity{}, err
	}
	content, err := companionDigest(fields["content_digest"])
	if err != nil {
		return DocumentIdentity{}, err
	}
	members, ok := fields["document_digest"].Members()
	if !ok {
		return DocumentIdentity{}, rejection.New(rejection.InvalidJSON, "document digest must be an object")
	}
	if len(members) == 0 {
		return DocumentIdentity{}, rejection.New(rejection.MissingRequiredMember, "document digest lacks kind")
	}
	kindValue := strictjson.Value{}
	for _, member := range members {
		if member.Name == "kind" {
			kindValue = member.Value
		}
	}
	kind, err := companionText(kindValue)
	if err != nil {
		return DocumentIdentity{}, err
	}
	switch kind {
	case "available":
		digestFields, err := companionObject(fields["document_digest"], []string{"kind", "value"})
		if err != nil {
			return DocumentIdentity{}, err
		}
		domain, err := companionDigest(digestFields["value"])
		if err != nil {
			return DocumentIdentity{}, err
		}
		return DocumentIdentity{ContentDigest: content, DomainDigest: domain, DomainAvailable: true}, nil
	case "unavailable":
		if _, err := companionObject(fields["document_digest"], []string{"kind"}); err != nil {
			return DocumentIdentity{}, err
		}
		return DocumentIdentity{ContentDigest: content}, nil
	default:
		return DocumentIdentity{}, rejection.New(rejection.UnknownTag, "unknown document digest availability")
	}
}

func parseCompanionOutcome(value strictjson.Value) (Outcome, error) {
	members, ok := value.Members()
	if !ok {
		return Outcome{}, rejection.New(rejection.InvalidJSON, "result outcome must be an object")
	}
	var kindValue strictjson.Value
	for _, member := range members {
		if member.Name == "kind" {
			kindValue = member.Value
		}
	}
	kindText, err := companionText(kindValue)
	if err != nil {
		return Outcome{}, err
	}
	kind := ResultKind(kindText)
	if kind == ResultAccepted {
		if _, err := companionObject(value, []string{"kind"}); err != nil {
			return Outcome{}, err
		}
		return Outcome{Kind: kind}, nil
	}
	switch kind {
	case ResultAcceptedWithTrust, ResultIncomplete, ResultRejected:
	default:
		return Outcome{}, rejection.New(rejection.UnknownTag, "unknown result kind")
	}
	fields, err := companionObject(value, []string{"kind", "refs"})
	if err != nil {
		return Outcome{}, err
	}
	refs, err := parseCompanionDigestArray(fields["refs"], true, "result refs")
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Kind: kind, Refs: refs}, nil
}

func parseCompanionTCB(value strictjson.Value) ([]RuntimeTCBComponent, error) {
	items, err := companionArray(value)
	if err != nil {
		return nil, err
	}
	if len(items) < len(requiredTCBCategories) {
		return nil, rejection.New(rejection.TCBIncomplete, "result omits required TCB components")
	}
	result := make([]RuntimeTCBComponent, 0, len(items))
	present := make(map[string]struct{}, len(items))
	for index, item := range items {
		fields, err := companionObject(item, []string{"artifact", "category", "version"})
		if err != nil {
			return nil, err
		}
		artifact, err := companionDigest(fields["artifact"])
		if err != nil {
			return nil, err
		}
		category, err := companionText(fields["category"])
		if err != nil {
			return nil, err
		}
		if !validTCBCategory(category) {
			return nil, rejection.New(rejection.UnknownTag, "unknown TCB category")
		}
		version, err := companionText(fields["version"])
		if err != nil {
			return nil, err
		}
		if !validVersion(version) {
			return nil, rejection.New(rejection.CheckerIdentity, "TCB component lacks an exact version")
		}
		component := RuntimeTCBComponent{Artifact: artifact, Category: category, Version: version}
		if index > 0 && compareRuntimeTCB(result[index-1], component) >= 0 {
			return nil, rejection.New(rejection.NoncanonicalOrder, "TCB entries are not sorted and unique")
		}
		present[category] = struct{}{}
		result = append(result, component)
	}
	for category := range requiredTCBCategories {
		if _, ok := present[category]; !ok {
			return nil, rejection.New(rejection.TCBIncomplete, "result omits a required TCB category")
		}
	}
	return result, nil
}

func validateCompanionDocument(document CompanionDocument) error {
	if document.Checker.Artifact == document.Checker.Source {
		return rejection.New(rejection.CheckerIdentity, "checker source cannot substitute for checker artifact")
	}
	byKind := make(map[CheckKind]Check, len(document.Checks))
	checkIDsPresent := make(map[protocol.Digest]struct{}, len(document.Checks))
	for _, check := range document.Checks {
		byKind[check.Definition.Kind] = check
		checkIDsPresent[check.ID] = struct{}{}
	}
	rejected := checkIDs(document.Checks, CheckRejected)
	incomplete := checkIDs(document.Checks, CheckIncomplete)
	trusted := checkIDs(document.Checks, CheckTrusted)
	var expectedKind ResultKind
	switch {
	case len(rejected) != 0:
		expectedKind = ResultRejected
	case len(incomplete) != 0 || len(document.MissingArtifacts) != 0:
		expectedKind = ResultIncomplete
	case len(document.RemainingTrust) != 0:
		expectedKind = ResultAcceptedWithTrust
	default:
		if len(trusted) != 0 {
			return rejection.New(rejection.ResultAggregation, "trusted check has no remaining trust")
		}
		expectedKind = ResultAccepted
	}
	if document.Outcome.Kind != expectedKind {
		return rejection.New(rejection.ResultAggregation, "top-level result does not match check outcomes")
	}
	for _, ref := range document.Outcome.Refs {
		if _, ok := checkIDsPresent[ref]; !ok {
			return rejection.New(rejection.ResultAggregation, "top-level result references an unknown check")
		}
	}
	if document.Outcome.Kind != ResultRejected && (!document.Evidence.DomainAvailable || !document.Request.DomainAvailable) {
		return rejection.New(rejection.ResultAggregation, "non-rejected result requires available document identities")
	}
	if document.Outcome.Kind == ResultAccepted || document.Outcome.Kind == ResultAcceptedWithTrust {
		if !hasCompleteCheckSet(byKind) {
			return rejection.New(rejection.ResultAggregation, "accepted result lacks the complete ten-check set")
		}
	}
	return nil
}

func companionObject(value strictjson.Value, expected []string) (map[string]strictjson.Value, error) {
	members, ok := value.Members()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected object")
	}
	allowed := make(map[string]struct{}, len(expected))
	for _, name := range expected {
		allowed[name] = struct{}{}
	}
	result := make(map[string]strictjson.Value, len(members))
	for _, member := range members {
		if _, ok := allowed[member.Name]; !ok {
			return nil, rejection.New(rejection.UnknownMember, "object contains unknown member")
		}
		result[member.Name] = member.Value
	}
	for _, name := range expected {
		if _, ok := result[name]; !ok {
			return nil, rejection.New(rejection.MissingRequiredMember, "object lacks required member")
		}
	}
	return result, nil
}

func companionArray(value strictjson.Value) ([]strictjson.Value, error) {
	items, ok := value.Items()
	if !ok {
		return nil, rejection.New(rejection.InvalidJSON, "expected array")
	}
	return items, nil
}

func companionText(value strictjson.Value) (string, error) {
	text, ok := value.Text()
	if !ok {
		return "", rejection.New(rejection.InvalidJSON, "expected string")
	}
	return text, nil
}

func companionDigest(value strictjson.Value) (protocol.Digest, error) {
	text, err := companionText(value)
	if err != nil {
		return protocol.Digest{}, err
	}
	return protocol.ParseDigest(text)
}

func parseCompanionDigestArray(value strictjson.Value, nonempty bool, label string) ([]protocol.Digest, error) {
	items, err := companionArray(value)
	if err != nil {
		return nil, err
	}
	if nonempty && len(items) == 0 {
		return nil, rejection.New(rejection.ResultAggregation, label+" cannot be empty")
	}
	result := make([]protocol.Digest, 0, len(items))
	for index, item := range items {
		digest, err := companionDigest(item)
		if err != nil {
			return nil, err
		}
		if index > 0 && result[index-1].String() >= digest.String() {
			return nil, rejection.New(rejection.NoncanonicalOrder, label+" are not sorted and unique")
		}
		result = append(result, digest)
	}
	return result, nil
}

func compareRef(left, right Ref) int {
	if left.Kind < right.Kind {
		return -1
	}
	if left.Kind > right.Kind {
		return 1
	}
	if left.ID.String() < right.ID.String() {
		return -1
	}
	if left.ID.String() > right.ID.String() {
		return 1
	}
	return 0
}

func compareRuntimeTCB(left, right RuntimeTCBComponent) int {
	if left.Category < right.Category {
		return -1
	}
	if left.Category > right.Category {
		return 1
	}
	if left.Artifact.String() < right.Artifact.String() {
		return -1
	}
	if left.Artifact.String() > right.Artifact.String() {
		return 1
	}
	return 0
}

func equalDigestSlices(left, right []protocol.Digest) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
