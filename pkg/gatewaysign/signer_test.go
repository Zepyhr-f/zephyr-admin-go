package gatewaysign

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestSignAndVerifyRoundtrip(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"hello":"world"}`)
	ts := int64(1750000000)
	nonce := "n-1"
	method := "POST"
	path := "/api/v1/ai/health"

	sig, err := SignWith(secret, SignParams{Method: method, Path: path, Ts: ts, Nonce: nonce, Body: body})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	NowSeconds = func() int64 { return ts }
	defer func() { NowSeconds = func() int64 { return time.Now().Unix() } }()

	v := Verifier{Secret: secret, TTL: 5 * time.Minute, Store: NewMemoryNonceStore()}
	if err := v.VerifyRequest(context.Background(), VerifyParams{
		Method: method, Path: path, Ts: strconv.FormatInt(ts, 10), Nonce: nonce, Body: body, Signature: sig,
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSignatureMismatch(t *testing.T) {
	secret := "real"
	ts := int64(1750000100)
	NowSeconds = func() int64 { return ts }
	defer func() { NowSeconds = func() int64 { return time.Now().Unix() } }()

	v := Verifier{Secret: secret, TTL: time.Minute, Store: NewMemoryNonceStore()}
	err := v.VerifyRequest(context.Background(), VerifyParams{
		Method: "GET", Path: "/x", Ts: strconv.FormatInt(ts, 10), Nonce: "n", Body: nil, Signature: "deadbeef",
	})
	if err == nil {
		t.Fatalf("expected error for invalid signature")
	}
}

func TestExpiredTimestamp(t *testing.T) {
	secret := "s"
	body := []byte("{}")
	ts := int64(1750000000)
	sig, _ := SignWith(secret, SignParams{Method: "GET", Path: "/x", Ts: ts, Nonce: "n", Body: body})
	NowSeconds = func() int64 { return ts + 600 }
	defer func() { NowSeconds = func() int64 { return time.Now().Unix() } }()

	v := Verifier{Secret: secret, TTL: 5 * time.Minute, Store: NewMemoryNonceStore()}
	err := v.VerifyRequest(context.Background(), VerifyParams{
		Method: "GET", Path: "/x", Ts: strconv.FormatInt(ts, 10), Nonce: "n", Body: body, Signature: sig,
	})
	if err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestReplayDetection(t *testing.T) {
	secret := "s"
	body := []byte("{}")
	ts := int64(1750000000)
	sig, _ := SignWith(secret, SignParams{Method: "GET", Path: "/x", Ts: ts, Nonce: "n", Body: body})
	NowSeconds = func() int64 { return ts }
	defer func() { NowSeconds = func() int64 { return time.Now().Unix() } }()

	v := Verifier{Secret: secret, TTL: time.Minute, Store: NewMemoryNonceStore()}
	p := VerifyParams{Method: "GET", Path: "/x", Ts: strconv.FormatInt(ts, 10), Nonce: "n", Body: body, Signature: sig}
	if err := v.VerifyRequest(context.Background(), p); err != nil {
		t.Fatalf("first call should pass: %v", err)
	}
	if err := v.VerifyRequest(context.Background(), p); err != ErrReplay {
		t.Fatalf("expected replay, got %v", err)
	}
}

func TestSignAndInjectMutatesHeaders(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://upstream/api/v1/ai/health", bytes.NewReader([]byte(`{"k":"v"}`)))
	NowSeconds = func() int64 { return 1750000000 }
	defer func() { NowSeconds = func() int64 { return time.Now().Unix() } }()

	if err := SignAndInject(req, "secret", UserContext{UserID: "u1", Username: "alice", Roles: []string{"admin"}}); err != nil {
		t.Fatalf("inject: %v", err)
	}
	for _, h := range []string{HeaderSign, HeaderTimestamp, HeaderNonce, HeaderRequestID, HeaderUserID, HeaderUsername, HeaderRoles} {
		if req.Header.Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
}
