package gatewaysign

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// UserContext describes the authenticated principal that the gateway wants
// to forward to downstream services.
type UserContext struct {
	UserID   string
	Username string
	TenantID string
	Roles    []string
}

// AnonymousContext returns the placeholder user used for unauthenticated calls.
func AnonymousContext() UserContext {
	return UserContext{UserID: AnonymousUser}
}

// SignAndInject mutates req to carry user and signature headers using the supplied secret.
// Body is buffered (replaced) so the upstream handler still observes the same payload.
// Caller must close req.Body if not nil; SignAndInject re-assigns Body when it drains it.
func SignAndInject(req *http.Request, secret string, user UserContext) error {
	body, err := drainBody(req)
	if err != nil {
		return err
	}
	ts := NowSeconds()
	nonce := uuid.NewString()
	requestID := uuid.NewString()

	if user.UserID == "" {
		user.UserID = AnonymousUser
	}

	signature, err := SignWith(secret, SignParams{
		Method: req.Method,
		Path:   req.URL.Path,
		Ts:     ts,
		Nonce:  nonce,
		Body:   body,
	})
	if err != nil {
		return err
	}

	req.Header.Set(HeaderSign, signature)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderRequestID, requestID)
	req.Header.Set(HeaderUserID, user.UserID)
	if user.Username != "" {
		req.Header.Set(HeaderUsername, user.Username)
	}
	if user.TenantID != "" {
		req.Header.Set(HeaderTenantID, user.TenantID)
	}
	if len(user.Roles) > 0 {
		req.Header.Set(HeaderRoles, joinRoles(user.Roles))
	}
	return nil
}

func drainBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(buf))
	req.ContentLength = int64(len(buf))
	return buf, nil
}

func joinRoles(roles []string) string {
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}

// userContextKey is the private context key type to avoid collisions.
type userContextKey struct{}

// WithUser stores the user context in ctx so downstream code can retrieve it.
func WithUser(ctx context.Context, u UserContext) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

// UserFromContext fetches the user, returning anonymous when absent.
func UserFromContext(ctx context.Context) UserContext {
	if v, ok := ctx.Value(userContextKey{}).(UserContext); ok {
		return v
	}
	return AnonymousContext()
}
