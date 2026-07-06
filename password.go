package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	// pbkdf2Iterations follows OWASP's current PBKDF2-HMAC-SHA256
	// recommendation (~56ms/hash on a modern desktop core; noticeably
	// slower, though still sub-second, on the weakest officially supported
	// target, a Raspberry Pi 2 - acceptable since this only runs on
	// login/poll-unlock/creation, not on every request). It only governs
	// newly hashed passwords - the encoded hash is self-describing (format:
	// "pbkdf2-sha256$<iterations>$..."), so existing hashes keep verifying
	// correctly against whatever iteration count they were created with even
	// after this constant changes.
	pbkdf2Iterations = 600_000
	pbkdf2SaltBytes  = 16
	pbkdf2KeyBytes   = 32
)

// pbkdf2 derives a key from password and salt using HMAC-SHA256, following
// RFC 8018. It is implemented here directly (rather than pulled in as a
// dependency) to keep the project dependency-free.
func pbkdf2(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	dk := make([]byte, 0, numBlocks*hashLen)
	buf := make([]byte, 4)

	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(buf, uint32(block))
		prf.Write(buf)
		u := prf.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)

		for i := 1; i < iterations; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}

		dk = append(dk, t...)
	}

	return dk[:keyLen]
}

// HashPassword returns a self-describing, salted hash suitable for storage,
// in the form "pbkdf2-sha256$<iterations>$<saltHex>$<hashHex>".
func HashPassword(password string) (string, error) {
	salt, err := randomHex(pbkdf2SaltBytes)
	if err != nil {
		return "", err
	}
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return "", err
	}

	derived := pbkdf2([]byte(password), saltBytes, pbkdf2Iterations, pbkdf2KeyBytes)

	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iterations, salt, hex.EncodeToString(derived)), nil
}

// VerifyPassword checks password against a hash produced by HashPassword,
// using a constant-time comparison.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	saltBytes, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	wantHash, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	gotHash := pbkdf2([]byte(password), saltBytes, iterations, len(wantHash))

	return subtle.ConstantTimeCompare(gotHash, wantHash) == 1
}
