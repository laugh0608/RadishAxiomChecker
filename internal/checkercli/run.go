package checkercli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"radishaxiom.dev/independent-checker-go/internal/checkresult"
)

const (
	exitSuccess     = 0
	exitFailure     = 1
	exitUsage       = 2
	diagnosticLimit = 64 << 10
)

// Run executes the single public checker command. A nonzero return always
// means no complete canonical result was produced by this process.
func Run(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	resolve IdentityResolver,
) int {
	root, err := parseInvocation(args)
	if err != nil {
		writeDiagnostic(stderr, err)
		return exitUsage
	}
	if err := requireEmptyStdin(stdin); err != nil {
		writeDiagnostic(stderr, err)
		return exitUsage
	}
	if resolve == nil {
		writeDiagnostic(stderr, fmt.Errorf("checker runtime identity resolver is unavailable"))
		return exitFailure
	}
	identity, err := resolve()
	if err != nil {
		writeDiagnostic(stderr, err)
		return exitFailure
	}
	if err := checkresult.ValidateRuntimeIdentity(identity.Boundary, identity.Runtime); err != nil {
		writeDiagnostic(stderr, fmt.Errorf("checker runtime identity is invalid: %w", err))
		return exitFailure
	}
	invocation, err := checkresult.InvokeBundle(root, identity.Boundary, identity.Runtime, time.Now)
	if err != nil {
		writeDiagnostic(stderr, fmt.Errorf("checker invocation failed: %w", err))
		return exitFailure
	}
	if invocation.Kind != checkresult.InvocationResult {
		writeDiagnostic(stderr, fmt.Errorf("checker invocation produced no canonical result"))
		return exitFailure
	}
	if err := writeAll(stdout, invocation.Companion.Bytes()); err != nil {
		writeDiagnostic(stderr, fmt.Errorf("write canonical result: %w", err))
		return exitFailure
	}
	return exitSuccess
}

func parseInvocation(args []string) (string, error) {
	if len(args) != 2 || args[0] != "check" {
		return "", fmt.Errorf("usage: radishaxiom-independent-checker-go check --bundle-root=<canonical-realpath>")
	}
	root, ok := strings.CutPrefix(args[1], "--bundle-root=")
	if !ok || root == "" {
		return "", fmt.Errorf("usage: radishaxiom-independent-checker-go check --bundle-root=<canonical-realpath>")
	}
	if !utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", fmt.Errorf("bundle root is not an absolute canonical path")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve bundle root: %w", err)
	}
	if resolved != root {
		return "", fmt.Errorf("bundle root is not a canonical realpath")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect bundle root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("bundle root is not a directory")
	}
	return root, nil
}

func requireEmptyStdin(stdin io.Reader) error {
	if stdin == nil {
		return fmt.Errorf("stdin is unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1))
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(data) != 0 {
		return fmt.Errorf("stdin must be empty")
	}
	return nil
}

func writeDiagnostic(stderr io.Writer, err error) {
	if stderr == nil || err == nil {
		return
	}
	message := "radishaxiom-independent-checker-go: " + sanitizeDiagnostic(err.Error())
	message = truncateUTF8(message, diagnosticLimit-1) + "\n"
	_ = writeAll(stderr, []byte(message))
}

func sanitizeDiagnostic(value string) string {
	value = strings.ToValidUTF8(value, "?")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, value)
}

func truncateUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func writeAll(writer io.Writer, data []byte) error {
	if writer == nil {
		return fmt.Errorf("output is unavailable")
	}
	for len(data) > 0 {
		written, err := writer.Write(data)
		if written > len(data) || written < 0 {
			return fmt.Errorf("invalid write count")
		}
		data = data[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}
