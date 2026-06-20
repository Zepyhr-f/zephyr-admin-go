package aiproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/logx"

	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/pkg/gatewaysign"
)

// ForwardHandler returns an http.Handler that forwards /api/v1/ai/* to the
// configured upstream while injecting the authenticated user headers and
// gateway HMAC signature.
func ForwardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svcCtx.AiUpstreamURL == nil {
			httpError(w, http.StatusServiceUnavailable, "ai_upstream_unconfigured")
			return
		}
		if svcCtx.GatewaySignSecret == "" {
			httpError(w, http.StatusInternalServerError, "gateway_sign_secret_missing")
			return
		}

		user := resolveUser(r, svcCtx)

		proxy := httputil.NewSingleHostReverseProxy(svcCtx.AiUpstreamURL)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			rewriteAiPath(req.URL, svcCtx)
			req.Host = svcCtx.AiUpstreamURL.Host

			// Strip Authorization to avoid leaking caller token to AI service.
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")

			if err := gatewaysign.SignAndInject(req, svcCtx.GatewaySignSecret, user); err != nil {
				logx.WithContext(req.Context()).Errorf("gateway sign failed: %v", err)
			}
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
			logx.WithContext(r.Context()).Errorf("ai upstream error: %v", err)
			if errors.Is(err, context.Canceled) {
				return
			}
			httpError(rw, http.StatusBadGateway, "ai_upstream_unreachable")
		}
		proxy.ServeHTTP(w, r)
	}
}

func rewriteAiPath(u *url.URL, svcCtx *svc.ServiceContext) {
	if svcCtx.AiUpstreamStrip == "" {
		return
	}
	if strings.HasPrefix(u.Path, svcCtx.AiUpstreamStrip) {
		u.Path = strings.TrimPrefix(u.Path, svcCtx.AiUpstreamStrip)
		if u.Path == "" {
			u.Path = "/"
		} else if !strings.HasPrefix(u.Path, "/") {
			u.Path = "/" + u.Path
		}
	}
}

// resolveUser turns whatever the gateway already knows about the request
// into a UserContext suitable for forwarding. We trust the JWT middleware to
// have already validated the token; if no token is present (e.g. /api/v1/ai/health
// is exempted), we forward as anonymous.
func resolveUser(r *http.Request, svcCtx *svc.ServiceContext) gatewaysign.UserContext {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return gatewaysign.AnonymousContext()
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if tokenString == "" {
		return gatewaysign.AnonymousContext()
	}

	parsed, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(svcCtx.Config.Auth.AccessSecret), nil
	})
	if parsed == nil {
		return gatewaysign.AnonymousContext()
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return gatewaysign.AnonymousContext()
	}

	uc := gatewaysign.UserContext{}
	if v, ok := claims["userId"].(string); ok {
		uc.UserID = v
	}
	if v, ok := claims["userCode"].(string); ok && uc.UserID == "" {
		uc.UserID = v
	}
	if v, ok := claims["username"].(string); ok {
		uc.Username = v
	}
	if v, ok := claims["tenantId"].(string); ok {
		uc.TenantID = v
	}
	if rolesRaw, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rolesRaw {
			if s, ok := r.(string); ok && s != "" {
				uc.Roles = append(uc.Roles, s)
			}
		}
	}
	if uc.UserID == "" {
		uc.UserID = gatewaysign.AnonymousUser
	}
	return uc
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"code":` + statusCodeBody(code) + `,"msg":"` + msg + `"}`))
}

func statusCodeBody(code int) string {
	switch code {
	case http.StatusServiceUnavailable:
		return "503"
	case http.StatusBadGateway:
		return "502"
	default:
		return "500"
	}
}
