package ids

import (
	"crypto/rand"
	"encoding/hex"
)

func NewSessionID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
