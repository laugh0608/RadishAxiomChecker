package protocol

import (
	"encoding/hex"

	"radishaxiom.dev/independent-checker-go/internal/rejection"
)

// Digest is a closed SHA-256 identity, never an abbreviated or case-folded string.
type Digest [32]byte

func ParseDigest(text string) (Digest, error) {
	var digest Digest
	if len(text) != len("sha256:")+64 || text[:len("sha256:")] != "sha256:" {
		return digest, rejection.New(rejection.InvalidJSON, "digest must use sha256 and 64 lowercase hex characters")
	}
	for i := len("sha256:"); i < len(text); i++ {
		b := text[i]
		if !('0' <= b && b <= '9') && !('a' <= b && b <= 'f') {
			return digest, rejection.New(rejection.InvalidJSON, "digest must use lowercase hexadecimal")
		}
	}
	if _, err := hex.Decode(digest[:], []byte(text[len("sha256:"):])); err != nil {
		return Digest{}, rejection.New(rejection.InvalidJSON, "invalid SHA-256 digest")
	}
	return digest, nil
}

func (d Digest) String() string {
	return "sha256:" + hex.EncodeToString(d[:])
}

func (d Digest) BlobName() string {
	return hex.EncodeToString(d[:])
}
