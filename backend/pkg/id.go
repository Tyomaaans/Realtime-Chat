package pkg

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(data[:]), nil
}
