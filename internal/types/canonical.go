package types

import (
	"bytes"
	"encoding/json"
)

// CanonicalBytes returns stable UTF-8 JSON for a TTD.
func CanonicalBytes(td TypeDefinition) ([]byte, error) {
	return json.Marshal(td)
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}
