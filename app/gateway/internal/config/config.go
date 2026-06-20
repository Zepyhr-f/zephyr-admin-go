// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	AuthRpc     zrpc.RpcClientConf
	IdentityRpc zrpc.RpcClientConf
	Auth        struct {
		AccessSecret string
		AccessExpire int64
	}
	BizRedis struct {
		Host string
		Pass string
		DB   int
	}
	Gateway struct {
		SignSecret      string
		SignTTLSeconds  int
		AiUpstream      string
		AiPathPrefix    string
		AiUpstreamStrip string
	}
}
