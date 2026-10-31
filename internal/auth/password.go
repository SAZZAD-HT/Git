package auth

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"anvil/internal/errs"
)

// argon2id parameters from docs/reference/10-security.md §1: 64 MiB, 3 passes, 2 lanes.
const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// MinPasswordLength is the shortest password accepted.
const MinPasswordLength = 10

// argonSem limits concurrent hashes: each uses 64 MiB, so unlimited parallel sign-ins
// could exhaust memory.
var argonSem = make(chan struct{}, 4)

// HashPassword returns an argon2id encoded hash of password in the standard PHC format
// ($argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>). It takes roughly 50–150 ms and 64 MiB,
// and blocks while four other hashes are running.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	argonSem <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-argonSem
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches encoded, in constant time. It honours the
// parameters stored in encoded, so hashes made with older settings still verify.
// It returns an error only if encoded is malformed.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("password hash is not argon2id")
	}
	var version int
	var mem, iters uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &threads); err != nil {
		return false, fmt.Errorf("bad argon2 parameters: %w", err)
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	argonSem <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, iters, mem, threads, uint32(len(want)))
	<-argonSem
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is verified when a sign-in names an unknown user, so that the response takes
// as long as for a real user and timing does not reveal which usernames exist.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("not-a-real-password")
	return h
})

//go:embed common-passwords.txt
var commonPasswordsTxt string

// commonPasswords is the 10,000 most common passwords, lower-cased
// (source: SecLists Passwords/Common-Credentials/10k-most-common.txt, MIT licence).
var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	m := make(map[string]struct{}, 10_000)
	sc := bufio.NewScanner(strings.NewReader(commonPasswordsTxt))
	for sc.Scan() {
		m[sc.Text()] = struct{}{}
	}
	return m
})

// CheckPasswordPolicy enforces the password rules from docs/reference/10 §1: at least
// MinPasswordLength characters, not one of the 10,000 most common passwords, and not the
// username. It returns errs.FieldErrors for the "password" field, or nil.
func CheckPasswordPolicy(password, username string) error {
	lower := strings.ToLower(password)
	switch {
	case utf8.RuneCountInString(password) < MinPasswordLength:
		return errs.FieldErrors{"password": fmt.Sprintf("must be at least %d characters", MinPasswordLength)}
	case utf8.RuneCountInString(password) > 256:
		return errs.FieldErrors{"password": "must be at most 256 characters"}
	case username != "" && strings.Contains(lower, strings.ToLower(username)):
		return errs.FieldErrors{"password": "must not contain the username"}
	}
	if _, common := commonPasswords()[lower]; common {
		return errs.FieldErrors{"password": "is too common; choose another"}
	}
	return nil
}
