package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/joshmcarthur/trove/internal/blob"
)

// CanonicalBytes returns stable UTF-8 JSON for a TTD.
func CanonicalBytes(td TypeDefinition) ([]byte, error) {
	return json.Marshal(td)
}

// CanonicalHash parses raw TTD bytes, canonicalizes, and returns a blob ref.
func CanonicalHash(raw []byte) (string, error) {
	td, err := ParseTypeDefinition(raw)
	if err != nil {
		return "", err
	}
	canonical, err := CanonicalBytes(td)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return blob.FormatRef(hex.EncodeToString(sum[:])), nil
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
