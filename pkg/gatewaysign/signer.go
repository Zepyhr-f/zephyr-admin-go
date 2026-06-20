package gatewaysign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrSecretMissing is returned when the gateway sign secret is empty.
var ErrSecretMissing = errors.New("gatewaysign: secret missing")

// CanonicalMessage builds the deterministic message used for HMAC signing.
//
// Layout (all separated by '\n'):
//
//	UPPER(method) + "\n" +
//	path          + "\n" +
//	ts            + "\n" +
//	nonce         + "\n" +
//	hex(sha256(body))
//
// `path` MUST NOT contain the query string.
func CanonicalMessage(method, path string, ts int64, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	return strings.ToUpper(method) + "\n" +
		path + "\n" +
		strconv.FormatInt(ts, 10) + "\n" +
		nonce + "\n" +
		hex.EncodeToString(bodyHash[:])
}

// Sign returns the lowercase hex HMAC-SHA256 signature for the given canonical message.
func Sign(secret, message string) (string, error) {
	if secret == "" {
		return "", ErrSecretMissing
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(message)); err != nil {
		return "", fmt.Errorf("gatewaysign: hmac write: %w", err)
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// SignParams describes the per-request material used to compute a signature.
type SignParams struct {
	Method string
	Path   string
	Ts     int64
	Nonce  string
	Body   []byte
}

// SignWith computes the signature for the given request material.
func SignWith(secret string, p SignParams) (string, error) {
	return Sign(secret, CanonicalMessage(p.Method, p.Path, p.Ts, p.Nonce, p.Body))
}

// Verify performs constant-time comparison.
func Verify(secret string, p SignParams, signature string) error {
	if secret == "" {
		return ErrSecretMissing
	}
	expected, err := SignWith(secret, p)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("gatewaysign: signature mismatch")
	}
	return nil
}

// NowSeconds returns the current unix timestamp as an int64.
// Wrapped so tests can override via package variable.
var NowSeconds = func() int64 { return time.Now().Unix() }
