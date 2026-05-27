package ids

import (
	"crypto/rand"
	"encoding/hex"
)

const sessionIDLength = 8

func NewSessionID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func ValidSessionID(id string) bool {
	if len(id) != sessionIDLength {
		return false
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' && ch < 'a' || ch > 'f' {
			return false
		}
	}
	return true
}
