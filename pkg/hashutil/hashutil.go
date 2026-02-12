package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"lukechampine.com/blake3"
)

type HashAlgo string

const (
	HashAlgoSHA256 = "sha256"
	HashAlgoBLAKE3 = "blake3"
)

// HashBytes returns the hash of bytes as a hex string using the specified algorithm.
// Supported algorithms: "sha256" and "blake3".
func HashBytes(data []byte, algo HashAlgo) (string, error) {
	switch algo {
	case HashAlgoSHA256:
		return hashBytesSha256(data), nil
	case HashAlgoBLAKE3:
		return hashBytesBlake3(data), nil
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algo)
	}
}

func hashBytesSha256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hashBytesBlake3(data []byte) string {
	hash := blake3.Sum256(data)
	return hex.EncodeToString(hash[:])
}
