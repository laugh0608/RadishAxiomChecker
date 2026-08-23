package sourceidentity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	ManifestPath = "source-identity/checker-source-v0.1.jcs"

	expectedGoMod = "module radishaxiom.dev/independent-checker-go\n\ngo 1.26.0\n\ntoolchain go1.26.7\n"
)

var (
	ErrInvalidRoot       = errors.New("invalid source snapshot root")
	ErrInvalidPath       = errors.New("invalid source snapshot path")
	ErrInvalidMode       = errors.New("invalid source snapshot file mode")
	ErrUnsupportedType   = errors.New("unsupported source snapshot file type")
	ErrSourceChanged     = errors.New("source file changed while hashing")
	ErrModuleIdentity    = errors.New("Go module identity mismatch")
	ErrManifestMissing   = errors.New("source manifest is unavailable")
	ErrManifestDrift     = errors.New("source manifest drift")
	ErrSourceUnavailable = errors.New("source snapshot input is unavailable")
)

type fileRecord struct {
	byteLength    uint64
	contentDigest string
	mode          string
	path          string
}

// Snapshot is the replayed checker.source identity. Digest is the SHA-256
// content digest of Manifest, not a Git object identity.
type Snapshot struct {
	Digest       string
	FileCount    int
	Manifest     []byte
	ManifestSize int
}

// Generate enumerates the v0.1 source input set and returns its canonical
// manifest. It does not consult Git, the network, PATH, or a cache.
func Generate(root string) (Snapshot, error) {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Snapshot{}, ErrInvalidRoot
	}

	files, err := collectFiles(root)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateModuleIdentity(files); err != nil {
		return Snapshot{}, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})
	manifest := encodeManifest(files)
	return Snapshot{
		Digest:       DigestManifest(manifest),
		FileCount:    len(files),
		Manifest:     manifest,
		ManifestSize: len(manifest),
	}, nil
}

// Verify regenerates the manifest and requires byte-for-byte equality with
// the committed sidecar. Any extra input, removed input, byte/mode drift, or
// sidecar drift fails closed.
func Verify(root string) (Snapshot, error) {
	expectedPath := filepath.Join(root, filepath.FromSlash(ManifestPath))
	expected, err := readManifest(expectedPath)
	if err != nil {
		return Snapshot{}, err
	}
	actual, err := Generate(root)
	if err != nil {
		return Snapshot{}, err
	}
	if !bytes.Equal(expected, actual.Manifest) {
		return Snapshot{}, ErrManifestDrift
	}
	if DigestManifest(expected) != actual.Digest {
		return Snapshot{}, ErrManifestDrift
	}
	return actual, nil
}

// DigestManifest returns the protocol spelling used by checker.source.
func DigestManifest(manifest []byte) string {
	sum := sha256.Sum256(manifest)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func collectFiles(root string) ([]fileRecord, error) {
	files := make([]fileRecord, 0, 128)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%w: repository entry cannot be enumerated", ErrSourceUnavailable)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return ErrInvalidPath
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if !validPath(relative) {
			return fmt.Errorf("%w: %s", ErrInvalidPath, relative)
		}
		if relative == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrSourceUnavailable, relative)
		}
		if relative == ManifestPath {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("%w: %s", ErrUnsupportedType, relative)
			}
			if mode, err := normalizedMode(info.Mode()); err != nil || mode != "0644" {
				return fmt.Errorf("%w: %s", ErrInvalidMode, relative)
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrUnsupportedType, relative)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: %s", ErrUnsupportedType, relative)
		}
		mode, err := normalizedMode(info.Mode())
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidMode, relative)
		}
		record, err := hashRegular(path, relative, mode, info)
		if err != nil {
			return err
		}
		files = append(files, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func validPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
		for i := 0; i < len(component); i++ {
			b := component[i]
			if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
				(b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func normalizedMode(mode fs.FileMode) (string, error) {
	if !mode.IsRegular() || mode&^os.ModePerm != 0 {
		return "", ErrInvalidMode
	}
	switch mode.Perm() {
	case 0o644:
		return "0644", nil
	case 0o755:
		return "0755", nil
	default:
		return "", ErrInvalidMode
	}
}

func hashRegular(path string, relative string, mode string, before fs.FileInfo) (fileRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileRecord{}, fmt.Errorf("%w: %s", ErrSourceUnavailable, relative)
	}
	defer file.Close()

	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Mode() != before.Mode() {
		return fileRecord{}, fmt.Errorf("%w: %s", ErrSourceChanged, relative)
	}
	hash := sha256.New()
	length, err := io.Copy(hash, file)
	if err != nil {
		return fileRecord{}, fmt.Errorf("%w: %s", ErrSourceUnavailable, relative)
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) ||
		after.Mode() != opened.Mode() || after.Size() != opened.Size() || length != opened.Size() {
		return fileRecord{}, fmt.Errorf("%w: %s", ErrSourceChanged, relative)
	}
	return fileRecord{
		byteLength:    uint64(length),
		contentDigest: "sha256:" + hex.EncodeToString(hash.Sum(nil)),
		mode:          mode,
		path:          relative,
	}, nil
}

func validateModuleIdentity(files []fileRecord) error {
	expectedDigest := DigestManifest([]byte(expectedGoMod))
	for _, file := range files {
		if file.path != "go.mod" {
			continue
		}
		if file.byteLength == uint64(len(expectedGoMod)) && file.contentDigest == expectedDigest && file.mode == "0644" {
			return nil
		}
		return ErrModuleIdentity
	}
	return ErrModuleIdentity
}

func readManifest(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, ErrManifestMissing
	}
	mode, modeErr := normalizedMode(before.Mode())
	if modeErr != nil || mode != "0644" {
		return nil, ErrManifestMissing
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrManifestMissing
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() || opened.Mode() != before.Mode() {
		return nil, ErrManifestMissing
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, ErrManifestMissing
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || after.Size() != int64(len(data)) || after.Mode() != opened.Mode() {
		return nil, ErrManifestMissing
	}
	return data, nil
}

func encodeManifest(files []fileRecord) []byte {
	var out strings.Builder
	out.Grow(512 + len(files)*192)
	out.WriteString(`{"content_encoding":"raw-bytes","digest_algorithm":"sha256","exclusions":[{"path":".git","scope":"entry-and-descendants"},{"path":"source-identity/checker-source-v0.1.jcs","scope":"exact-file"}],"files":[`)
	for i, file := range files {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(`{"byte_length":"`)
		out.WriteString(strconv.FormatUint(file.byteLength, 10))
		out.WriteString(`","content_digest":"`)
		out.WriteString(file.contentDigest)
		out.WriteString(`","mode":"`)
		out.WriteString(file.mode)
		out.WriteString(`","path":"`)
		out.WriteString(file.path)
		out.WriteString(`","type":"regular"}`)
	}
	out.WriteString(`],"identity":"checker.source","manifest_encoding":"RFC8785-JCS-UTF-8","module":{"go_language":"1.26.0","go_toolchain":"go1.26.7","module_path":"radishaxiom.dev/independent-checker-go"},"path_encoding":"UTF-8","path_order":"UTF-8-byte-lexicographic","source_version":"0.1"}`)
	return []byte(out.String())
}
