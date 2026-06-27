package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/qalibre/qalibre/internal/config"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

// Salt characters for generating Werkzeug-compatible salts.
const saltChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomSalt generates a random alphanumeric string of a given length.
func GenerateRandomSalt(length int) (string, error) {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(saltChars))))
		if err != nil {
			return "", err
		}
		result[i] = saltChars[num.Int64()]
	}
	return string(result), nil
}

// GeneratePasswordHash hashes a password using Werkzeug-compatible scrypt.
func GeneratePasswordHash(password string) (string, error) {
	salt, err := GenerateRandomSalt(16)
	if err != nil {
		return "", err
	}

	// Default parameters: N=32768, r=8, p=1, keyLen=64
	N := 32768
	r := 8
	p := 1
	keyLen := 64

	hashBytes, err := scrypt.Key([]byte(password), []byte(salt), N, r, p, keyLen)
	if err != nil {
		return "", err
	}

	hashHex := hex.EncodeToString(hashBytes)
	return fmt.Sprintf("scrypt:%d:%d:%d$%s$%s", N, r, p, salt, hashHex), nil
}

// CheckPasswordHash checks a password against a Werkzeug-compatible hash string.
func CheckPasswordHash(hashStr, password string) bool {
	parts := strings.Split(hashStr, "$")
	if len(parts) != 3 {
		return false
	}

	method := parts[0]
	salt := parts[1]
	hashHex := parts[2]

	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	var actualHash []byte

	if strings.HasPrefix(method, "scrypt:") {
		// Format: scrypt:N:r:p
		params := strings.Split(method, ":")
		if len(params) != 4 {
			return false
		}
		N, err1 := strconv.Atoi(params[1])
		r, err2 := strconv.Atoi(params[2])
		p, err3 := strconv.Atoi(params[3])
		if err1 != nil || err2 != nil || err3 != nil {
			return false
		}
		actualHash, err = scrypt.Key([]byte(password), []byte(salt), N, r, p, len(expectedHash))
		if err != nil {
			return false
		}
	} else if strings.HasPrefix(method, "pbkdf2:sha256") {
		// Format: pbkdf2:sha256:iterations or pbkdf2:sha256
		params := strings.Split(method, ":")
		iterations := 150000 // default fallback
		if len(params) == 3 {
			var err1 error
			iterations, err1 = strconv.Atoi(params[2])
			if err1 != nil {
				return false
			}
		}
		actualHash = pbkdf2.Key([]byte(password), []byte(salt), iterations, len(expectedHash), sha256.New)
	} else {
		// Unsupported hash method
		return false
	}

	return subtle.ConstantTimeCompare(expectedHash, actualHash) == 1
}

// Password Policy Checker (replaces valid_password in helper.py)
func VerifyPasswordPolicy(password string, cfg config.Settings) error {
	if !cfg.ConfigPasswordPolicy {
		return nil
	}
	if len(password) < cfg.ConfigPasswordMinLength {
		return fmt.Errorf("password must be at least %d characters long", cfg.ConfigPasswordMinLength)
	}

	var hasLower, hasUpper, hasNumber, hasSpecial bool
	for _, char := range password {
		switch {
		case 'a' <= char && char <= 'z':
			hasLower = true
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case '0' <= char && char <= '9':
			hasNumber = true
		default:
			hasSpecial = true
		}
	}

	if cfg.ConfigPasswordLower && !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if cfg.ConfigPasswordUpper && !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if cfg.ConfigPasswordNumber && !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if cfg.ConfigPasswordSpecial && !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}

// User Context key for net/http request context
type contextKey struct{ name string }

var UserKey = &contextKey{"user"}

// HasRole checks if the given user role bitmask has a specific role flag.
func HasRole(role, flag int) bool {
	return flag == (flag & role)
}

// HasSidebar checks if the user sidebar bitmask has a specific sidebar flag.
func HasSidebar(sidebarView, flag int) bool {
	return flag == (flag & sidebarView)
}

// GetUserFromContext gets the current authenticated user from request context.
func GetUserFromContext(r *http.Request) (interface{}, bool) {
	val := r.Context().Value(UserKey)
	if val == nil {
		return nil, false
	}
	return val, true
}
