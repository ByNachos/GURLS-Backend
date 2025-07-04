package random

import (
	"crypto/rand"
	"math/big"
)

const (
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// NewRandomString generates a random string of a given length from the alphabet.
func NewRandomString(length int) (string, error) {
	b := make([]byte, length)
	max := big.NewInt(int64(len(alphabet)))

	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}

	return string(b), nil
}
