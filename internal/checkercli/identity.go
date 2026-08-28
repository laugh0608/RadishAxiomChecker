package checkercli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"radishaxiom.dev/independent-checker-go/internal/checkresult"
	"radishaxiom.dev/independent-checker-go/internal/protocol"
)

// RuntimeInputs are supplied only by the process entrypoint. Source and
// Version are linker-injected build facts; Toolchain is the actual Go runtime
// version; Executable is the path of the running checker binary.
type RuntimeInputs struct {
	Source     string
	Toolchain  string
	Version    string
	Executable string
}

type InvocationIdentity struct {
	Boundary checkresult.IdentityBoundary
	Runtime  checkresult.RuntimeIdentity
}

type IdentityResolver func() (InvocationIdentity, error)

// ResolveRuntimeIdentity binds one invocation to the exact source snapshot,
// toolchain, implementation version, and executable bytes that produced it.
func ResolveRuntimeIdentity(input RuntimeInputs) (InvocationIdentity, error) {
	source, err := protocol.ParseDigest(input.Source)
	if err != nil {
		return InvocationIdentity{}, fmt.Errorf("checker source identity is unavailable: %w", err)
	}
	artifact, err := digestExecutable(input.Executable)
	if err != nil {
		return InvocationIdentity{}, err
	}
	boundary := checkresult.NewSourceBoundary(source, input.Toolchain, input.Version)
	runtimeIdentity := checkresult.RuntimeIdentity{CheckerArtifact: artifact}
	for _, component := range boundary.TCB {
		runtimeIdentity.TCB = append(runtimeIdentity.TCB, checkresult.RuntimeTCBComponent{
			Artifact: artifact,
			Category: component.Category,
			Version:  component.Version,
		})
	}
	if err := checkresult.ValidateRuntimeIdentity(boundary, runtimeIdentity); err != nil {
		return InvocationIdentity{}, fmt.Errorf("checker runtime identity is unavailable: %w", err)
	}
	return InvocationIdentity{Boundary: boundary, Runtime: runtimeIdentity}, nil
}

func digestExecutable(path string) (protocol.Digest, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return protocol.Digest{}, fmt.Errorf("checker executable path is not absolute and canonical")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return protocol.Digest{}, fmt.Errorf("resolve checker executable path: %w", err)
	}
	if resolved != path {
		return protocol.Digest{}, fmt.Errorf("checker executable path is not a canonical realpath")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return protocol.Digest{}, fmt.Errorf("inspect checker executable: %w", err)
	}
	if !before.Mode().IsRegular() || (runtime.GOOS != "windows" && before.Mode().Perm()&0o111 == 0) ||
		before.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return protocol.Digest{}, fmt.Errorf("checker executable is not a plain executable file")
	}
	file, err := os.Open(path)
	if err != nil {
		return protocol.Digest{}, fmt.Errorf("open checker executable: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return protocol.Digest{}, fmt.Errorf("inspect opened checker executable: %w", err)
	}
	if !os.SameFile(before, opened) {
		return protocol.Digest{}, fmt.Errorf("checker executable changed before hashing")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return protocol.Digest{}, fmt.Errorf("hash checker executable: %w", err)
	}
	after, err := os.Lstat(path)
	if err != nil {
		return protocol.Digest{}, fmt.Errorf("reinspect checker executable: %w", err)
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return protocol.Digest{}, fmt.Errorf("checker executable changed while hashing")
	}
	var digest protocol.Digest
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
