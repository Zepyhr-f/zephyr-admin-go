// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"net/url"
	"time"

	"zephyr-go/app/auth/authservice"
	"zephyr-go/app/gateway/internal/config"
	"zephyr-go/app/identity/identityservice"
	"zephyr-go/pkg/gatewaysign"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config      config.Config
	AuthRpc     authservice.AuthService
	IdentityRpc identityservice.IdentityService

	BizRedis *redis.Redis

	GatewaySignSecret string
	GatewaySignTTL    time.Duration
	AiUpstreamURL     *url.URL
	AiPathPrefix      string
	AiUpstreamStrip   string
}

func NewServiceContext(c config.Config) *ServiceContext {
	var bizRedis *redis.Redis
	if c.BizRedis.Host != "" {
		bizRedis = redis.New(c.BizRedis.Host, func(r *redis.Redis) {
			r.Type = redis.NodeType
			r.Pass = c.BizRedis.Pass
		})
	}

	ttl := time.Duration(c.Gateway.SignTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	var aiURL *url.URL
	if c.Gateway.AiUpstream != "" {
		if u, err := url.Parse(c.Gateway.AiUpstream); err == nil {
			aiURL = u
		}
	}

	prefix := c.Gateway.AiPathPrefix
	if prefix == "" {
		prefix = "/api/v1/ai/"
	}

	return &ServiceContext{
		Config:      c,
		AuthRpc:     authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc)),
		IdentityRpc: identityservice.NewIdentityService(zrpc.MustNewClient(c.IdentityRpc)),

		BizRedis: bizRedis,

		GatewaySignSecret: c.Gateway.SignSecret,
		GatewaySignTTL:    ttl,
		AiUpstreamURL:     aiURL,
		AiPathPrefix:      prefix,
		AiUpstreamStrip:   c.Gateway.AiUpstreamStrip,
	}
}

// AnonymousUser exposes a tiny helper so handlers don't need to import gatewaysign just to grab the placeholder.
func (s *ServiceContext) AnonymousUser() gatewaysign.UserContext {
	return gatewaysign.AnonymousContext()
}
