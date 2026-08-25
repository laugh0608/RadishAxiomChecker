package bundle

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"

	"radishaxiom.dev/independent-checker-go/internal/protocol"
	"radishaxiom.dev/independent-checker-go/internal/rejection"
	"radishaxiom.dev/independent-checker-go/internal/strictjson"
)

const (
	hardArtifactBytes   uint64 = 1 << 20
	hardBundleBytes     uint64 = 4 << 20
	hardCollectionItems uint64 = 10_000
	hardJSONDepth       uint64 = 128
	hardSemanticSteps   uint64 = 1_000_000
)

var requestEnvelopeLimits = strictjson.Limits{
	MaxBytes: hardArtifactBytes,
	MaxDepth: hardJSONDepth,
	MaxItems: hardCollectionItems,
	MaxSteps: hardSemanticSteps,
}

// Verified is the identity-only result of the parser slice. It is not an
// Axiom independent-check result and makes no Evidence semantic claim.
type Verified struct {
	Request        protocol.Request
	Manifest       protocol.Manifest
	RequestDigest  protocol.Digest
	ManifestDigest protocol.Digest
	root           string
	artifactLimit  uint64
}

// Verify parses and verifies a read-only bundle without mutating it.
func Verify(root string) (Verified, error) {
	if err := verifyRoot(root); err != nil {
		return Verified{}, err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Verified{}, rejection.New(rejection.IsolationBoundaryViolation, "bundle root cannot be resolved to an absolute path")
	}
	requestBytes, _, err := readRegular(filepath.Join(root, "request.jcs"), hardArtifactBytes)
	if err != nil {
		return Verified{}, err
	}
	request, err := protocol.ParseRequest(requestBytes, requestEnvelopeLimits)
	if err != nil {
		return Verified{}, err
	}
	requestDigest := digestBytes(requestBytes)

	artifactLimit := minLimit(request, "artifact-bytes", hardArtifactBytes)
	bundleLimit := minLimit(request, "bundle-bytes", hardBundleBytes)
	manifestBytes, _, err := readRegular(filepath.Join(root, "manifest.jcs"), artifactLimit)
	if err != nil {
		return Verified{}, err
	}
	manifestDigest := digestBytes(manifestBytes)
	if manifestDigest != request.BundleManifest {
		return Verified{}, rejection.New(rejection.RequestBindingMismatch, "request does not bind the supplied manifest bytes")
	}
	manifestLimits := strictjson.Limits{
		MaxBytes: artifactLimit,
		MaxDepth: minLimit(request, "json-depth", hardJSONDepth),
		MaxItems: minLimit(request, "collection-items", hardCollectionItems),
		MaxSteps: minLimit(request, "semantic-steps", hardSemanticSteps),
	}
	manifest, err := protocol.ParseManifest(manifestBytes, manifestLimits)
	if err != nil {
		return Verified{}, err
	}
	if err := verifyEvidenceBinding(request, manifest); err != nil {
		return Verified{}, err
	}
	if err := verifyBlobs(root, manifest, artifactLimit, bundleLimit); err != nil {
		return Verified{}, err
	}
	return Verified{
		Request:        request,
		Manifest:       manifest,
		RequestDigest:  requestDigest,
		ManifestDigest: manifestDigest,
		root:           absoluteRoot,
		artifactLimit:  artifactLimit,
	}, nil
}

// ReadArtifact reopens one manifest-listed artifact from the verified bundle,
// rechecking ordinary-file identity, declared length, and raw SHA-256. The
// bundle may have changed since Verify, so a successful earlier verification
// is never used as a mutable byte cache.
func (verified Verified) ReadArtifact(digest protocol.Digest) ([]byte, error) {
	var artifact protocol.Artifact
	found := false
	for _, candidate := range verified.Manifest.Artifacts {
		if candidate.ContentDigest == digest {
			artifact = candidate
			found = true
			break
		}
	}
	if !found {
		return nil, rejection.New(rejection.ArtifactMissing, "artifact is not listed by the verified bundle manifest")
	}
	if artifact.ByteLength > verified.artifactLimit {
		return nil, rejection.New(rejection.ResourceLimit, "artifact exceeds the verified request byte limit")
	}
	data, length, err := readRegular(
		filepath.Join(verified.root, "blobs", "sha256", digest.BlobName()),
		artifact.ByteLength,
	)
	if err != nil {
		return nil, err
	}
	if length != artifact.ByteLength {
		return nil, rejection.New(rejection.LengthMismatch, "artifact length differs from the verified manifest")
	}
	if digestBytes(data) != digest {
		return nil, rejection.New(rejection.DigestMismatch, "artifact bytes changed after bundle verification")
	}
	return data, nil
}

func verifyRoot(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return rejection.New(rejection.ArtifactMissing, "bundle root is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return rejection.New(rejection.IsolationBoundaryViolation, "bundle root must be a non-symlink directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return rejection.New(rejection.ArtifactMissing, "bundle root cannot be enumerated")
	}
	required := map[string]bool{"request.jcs": false, "manifest.jcs": false, "blobs": false}
	for _, entry := range entries {
		if _, ok := required[entry.Name()]; !ok {
			return rejection.New(rejection.ManifestCoverage, "bundle root contains an unlisted entry")
		}
		required[entry.Name()] = true
	}
	for _, present := range required {
		if !present {
			return rejection.New(rejection.ArtifactMissing, "bundle root is missing a required entry")
		}
	}
	if err := requireDirectory(filepath.Join(root, "blobs")); err != nil {
		return err
	}
	blobNamespaces, err := os.ReadDir(filepath.Join(root, "blobs"))
	if err != nil {
		return rejection.New(rejection.ArtifactMissing, "blob namespace cannot be enumerated")
	}
	if len(blobNamespaces) != 1 || blobNamespaces[0].Name() != "sha256" {
		return rejection.New(rejection.ManifestCoverage, "blob namespace must contain only sha256")
	}
	return requireDirectory(filepath.Join(root, "blobs", "sha256"))
}

func requireDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return rejection.New(rejection.ArtifactMissing, "required bundle directory is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return rejection.New(rejection.IsolationBoundaryViolation, "bundle directory must not be a symlink or type alias")
	}
	return nil
}

func readRegular(path string, maxBytes uint64) ([]byte, uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, rejection.New(rejection.ArtifactMissing, "required bundle file is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, 0, rejection.New(rejection.IsolationBoundaryViolation, "bundle file must be an ordinary non-symlink file")
	}
	if info.Size() < 0 || uint64(info.Size()) > maxBytes {
		return nil, 0, rejection.New(rejection.ResourceLimit, "bundle file exceeds its byte limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, rejection.New(rejection.ArtifactMissing, "bundle file cannot be opened")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, 0, rejection.New(rejection.IsolationBoundaryViolation, "bundle file identity changed while opening")
	}
	limited := io.LimitReader(file, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, rejection.New(rejection.ArtifactMissing, "bundle file cannot be read")
	}
	if uint64(len(data)) != uint64(info.Size()) {
		return nil, 0, rejection.New(rejection.LengthMismatch, "bundle file length changed while reading")
	}
	return data, uint64(len(data)), nil
}

func verifyEvidenceBinding(request protocol.Request, manifest protocol.Manifest) error {
	for _, artifact := range manifest.Artifacts {
		if artifact.HasRole("evidence") {
			if artifact.ContentDigest != request.Evidence {
				return rejection.New(rejection.RequestBindingMismatch, "request evidence digest does not match manifest evidence")
			}
			return nil
		}
	}
	return rejection.New(rejection.EvidenceCardinality, "manifest has no evidence artifact")
}

func verifyBlobs(
	root string,
	manifest protocol.Manifest,
	artifactLimit uint64,
	bundleLimit uint64,
) error {
	dir := filepath.Join(root, "blobs", "sha256")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rejection.New(rejection.ArtifactMissing, "blob directory cannot be enumerated")
	}
	expected := make(map[string]protocol.Artifact, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		expected[artifact.ContentDigest.BlobName()] = artifact
	}
	observed := make(map[string]struct{}, len(entries))
	var total uint64
	for _, entry := range entries {
		name := entry.Name()
		if !validBlobName(name) {
			return rejection.New(rejection.IsolationBoundaryViolation, "blob name is not a full lowercase SHA-256 digest")
		}
		artifact, listed := expected[name]
		if !listed {
			return rejection.New(rejection.ManifestCoverage, "blob is not listed by the manifest")
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return rejection.New(rejection.ArtifactMissing, "listed blob is unavailable")
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return rejection.New(rejection.IsolationBoundaryViolation, "blob must be an ordinary non-symlink file")
		}
		if info.Size() < 0 || uint64(info.Size()) != artifact.ByteLength {
			return rejection.New(rejection.LengthMismatch, "blob length differs from the manifest")
		}
		if artifact.ByteLength > artifactLimit {
			return rejection.New(rejection.ResourceLimit, "blob exceeds artifact byte limit")
		}
		nextTotal, ok := addChecked(total, artifact.ByteLength)
		if !ok || nextTotal > bundleLimit {
			return rejection.New(rejection.ResourceLimit, "bundle exceeds bundle byte limit")
		}
		total = nextTotal
		observed[name] = struct{}{}
	}
	for name := range expected {
		if _, ok := observed[name]; !ok {
			return rejection.New(rejection.ArtifactMissing, "manifest-listed blob is missing")
		}
	}
	for _, artifact := range manifest.Artifacts {
		actual, err := digestRegular(filepath.Join(dir, artifact.ContentDigest.BlobName()), artifact.ByteLength)
		if err != nil {
			return err
		}
		if actual != artifact.ContentDigest {
			return rejection.New(rejection.DigestMismatch, "blob SHA-256 differs from the manifest")
		}
	}
	return nil
}

func digestRegular(path string, declared uint64) (protocol.Digest, error) {
	var zero protocol.Digest
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return zero, rejection.New(rejection.IsolationBoundaryViolation, "blob identity changed before hashing")
	}
	file, err := os.Open(path)
	if err != nil {
		return zero, rejection.New(rejection.ArtifactMissing, "blob cannot be opened")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return zero, rejection.New(rejection.IsolationBoundaryViolation, "blob identity changed while opening")
	}
	hash := sha256.New()
	read, err := io.Copy(hash, io.LimitReader(file, int64(declared)+1))
	if err != nil {
		return zero, rejection.New(rejection.ArtifactMissing, "blob cannot be hashed")
	}
	if uint64(read) != declared {
		return zero, rejection.New(rejection.LengthMismatch, "blob length changed while hashing")
	}
	var digest protocol.Digest
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func digestBytes(data []byte) protocol.Digest {
	sum := sha256.Sum256(data)
	return protocol.Digest(sum)
}

func validBlobName(name string) bool {
	if len(name) != 64 {
		return false
	}
	for i := range name {
		b := name[i]
		if !('0' <= b && b <= '9') && !('a' <= b && b <= 'f') {
			return false
		}
	}
	return true
}

func minLimit(request protocol.Request, name string, hard uint64) uint64 {
	value, ok := request.Limit(name)
	if !ok || value > hard {
		return hard
	}
	return value
}

func addChecked(a, b uint64) (uint64, bool) {
	result := a + b
	return result, result >= a
}
