package protocol

import (
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

var artifactRoles = map[string]struct{}{
	"evidence":          {},
	"evidence-artifact": {},
	"normative-spec":    {},
	"subject":           {},
}

func ParseManifest(data []byte, limits strictjson.Limits) (Manifest, error) {
	value, err := strictjson.ParseCanonical(data, limits)
	if err != nil {
		return Manifest{}, err
	}
	root, err := object(value, []string{"artifacts", "bundle_version"})
	if err != nil {
		return Manifest{}, err
	}
	version, err := text(root["bundle_version"])
	if err != nil {
		return Manifest{}, err
	}
	if version != "0.1" {
		return Manifest{}, rejection.New(rejection.UnsupportedVersion, "unsupported bundle manifest version")
	}
	items, err := array(root["artifacts"])
	if err != nil {
		return Manifest{}, err
	}
	if len(items) == 0 {
		return Manifest{}, rejection.New(rejection.MissingRequiredMember, "manifest must contain at least one artifact")
	}
	artifacts := make([]Artifact, 0, len(items))
	evidenceCount := 0
	var previous Digest
	for i, item := range items {
		artifact, err := parseArtifact(item)
		if err != nil {
			return Manifest{}, err
		}
		if i > 0 {
			cmp := compareDigest(previous, artifact.ContentDigest)
			if cmp == 0 {
				return Manifest{}, rejection.New(rejection.DuplicateArtifact, "artifact digest appears more than once")
			}
			if cmp > 0 {
				return Manifest{}, rejection.New(rejection.NoncanonicalOrder, "artifacts are not sorted by content digest")
			}
		}
		if artifact.HasRole("evidence") {
			evidenceCount++
		}
		previous = artifact.ContentDigest
		artifacts = append(artifacts, artifact)
	}
	if evidenceCount != 1 {
		return Manifest{}, rejection.New(rejection.EvidenceCardinality, "manifest must identify exactly one evidence artifact")
	}
	return Manifest{Artifacts: artifacts, Version: version}, nil
}

func parseArtifact(value strictjson.Value) (Artifact, error) {
	fields, err := object(value, []string{
		"byte_length", "content_digest", "format", "format_version", "roles",
	})
	if err != nil {
		return Artifact{}, err
	}
	byteLength, err := decimal(fields["byte_length"], true)
	if err != nil {
		return Artifact{}, err
	}
	digestText, err := text(fields["content_digest"])
	if err != nil {
		return Artifact{}, err
	}
	digest, err := ParseDigest(digestText)
	if err != nil {
		return Artifact{}, err
	}
	format, err := text(fields["format"])
	if err != nil {
		return Artifact{}, err
	}
	if !validFormat(format) {
		return Artifact{}, rejection.New(rejection.InvalidJSON, "artifact format is not canonical")
	}
	formatVersion, err := text(fields["format_version"])
	if err != nil {
		return Artifact{}, err
	}
	if formatVersion == "" {
		return Artifact{}, rejection.New(rejection.InvalidJSON, "artifact format version is empty")
	}
	roles, err := strings(fields["roles"])
	if err != nil {
		return Artifact{}, err
	}
	if len(roles) == 0 {
		return Artifact{}, rejection.New(rejection.MissingRequiredMember, "artifact roles must not be empty")
	}
	if !strictlySorted(roles) && len(roles) > 1 {
		return Artifact{}, rejection.New(rejection.NoncanonicalOrder, "artifact roles are not sorted and unique")
	}
	for _, role := range roles {
		if _, ok := artifactRoles[role]; !ok {
			return Artifact{}, rejection.New(rejection.UnknownTag, "unknown artifact role")
		}
	}
	return Artifact{
		ByteLength:    byteLength,
		ContentDigest: digest,
		Format:        format,
		FormatVersion: formatVersion,
		Roles:         roles,
	}, nil
}

func validFormat(value string) bool {
	if value == "" || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for i := 1; i < len(value); i++ {
		b := value[i]
		if ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') || b == '-' {
			continue
		}
		return false
	}
	return true
}

func compareDigest(a, b Digest) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
