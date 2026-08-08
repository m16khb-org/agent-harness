package toolconformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CanonicalSchemaSHA256은 스키마의 정규 다이제스트를 계산한다. 순수 계산이므로
// domain 계층이 소유한다.
func CanonicalSchemaSHA256(schema map[string]any) (string, error) {
	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(schema); err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded.Bytes())
	return hex.EncodeToString(sum[:]), nil
}
