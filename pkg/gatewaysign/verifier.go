package gatewaysign

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// NonceStore is an abstraction over the storage backing replay protection.
// Implementations must atomically reserve the nonce for the given TTL and
// return whether the nonce was previously seen.
type NonceStore interface {
	// Reserve returns (firstSeen=true) when this nonce was not present yet.
	// firstSeen=false means a replay attempt.
	Reserve(ctx context.Context, nonce string, ttl time.Duration) (firstSeen bool, err error)
}

// ErrReplay is returned by Verifier when the nonce was already reserved.
var ErrReplay = errors.New("gatewaysign: nonce replay")

// ErrExpired is returned when the request timestamp drifts beyond the allowed window.
var ErrExpired = errors.New("gatewaysign: timestamp expired")

// VerifyParams carries everything required to validate an inbound signed request.
type VerifyParams struct {
	Method    string
	Path      string
	Ts        string
	Nonce     string
	Body      []byte
	Signature string
}

// Verifier ties together the secret, the replay store and the TTL window.
type Verifier struct {
	Secret string
	Store  NonceStore
	TTL    time.Duration
}

// VerifyRequest validates timestamp window, replay store and HMAC.
func (v Verifier) VerifyRequest(ctx context.Context, p VerifyParams) error {
	if v.Secret == "" {
		return ErrSecretMissing
	}
	if p.Signature == "" || p.Ts == "" || p.Nonce == "" {
		return errors.New("gatewaysign: missing sign headers")
	}
	tsInt, err := strconv.ParseInt(p.Ts, 10, 64)
	if err != nil {
		return errors.New("gatewaysign: invalid ts header")
	}
	now := NowSeconds()
	skew := now - tsInt
	if skew < 0 {
		skew = -skew
	}
	if v.TTL > 0 && time.Duration(skew)*time.Second > v.TTL {
		return ErrExpired
	}
	if err := Verify(v.Secret, SignParams{
		Method: p.Method,
		Path:   p.Path,
		Ts:     tsInt,
		Nonce:  p.Nonce,
		Body:   p.Body,
	}, p.Signature); err != nil {
		return err
	}
	if v.Store != nil {
		first, err := v.Store.Reserve(ctx, p.Nonce, ttlOrDefault(v.TTL))
		if err != nil {
			return err
		}
		if !first {
			return ErrReplay
		}
	}
	return nil
}

func ttlOrDefault(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 5 * time.Minute
	}
	return ttl
}
