package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

func Digest(previous string, sequence uint64, canonicalPayload []byte) string {
	hash := sha256.New()
	hash.Write([]byte(previous))
	hash.Write([]byte("\n"))
	hash.Write([]byte(strconv.FormatUint(sequence, 10)))
	hash.Write([]byte("\n"))
	hash.Write(canonicalPayload)
	return hex.EncodeToString(hash.Sum(nil))
}

func Verify(previous string, sequence uint64, canonicalPayload []byte, expected string) bool {
	return expected != "" && Digest(previous, sequence, canonicalPayload) == expected
}
