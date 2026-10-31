package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
)

// Token prefixes. A leaked token can be recognised by its prefix, and secret scanners can
// match it (docs/reference/10 §1).
const (
	PrefixPAT = "anv_pat_" // personal access token
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// tokenBodyLen is 40 base62 characters, about 238 bits of randomness.
const tokenBodyLen = 40

// NewToken returns a new random token with the given prefix ("anv_pat_") followed by
// 40 base62 characters, and its SHA-256 hash (hex) for storage.
func NewToken(prefix string) (plain, hash string, err error) {
	b := make([]byte, tokenBodyLen)
	max := big.NewInt(int64(len(base62)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", "", err
		}
		b[i] = base62[n.Int64()]
	}
	plain = prefix + string(b)
	return plain, HashSecret(plain), nil
}

// HashSecret returns the hex SHA-256 of a token or session id. A fast hash is right here:
// tokens are long random strings that cannot be brute-forced, unlike passwords.
func HashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// randomString returns n random bytes encoded as unpadded base64url.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
