// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package identity

import (
	"context"

	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/app/identity/identityservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取当前登录用户信息
func NewGetUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserInfoLogic {
	return &GetUserInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserInfoLogic) GetUserInfo(req *types.UserInfoReq) (resp *types.UserInfoResp, err error) {
	// 从 ctx 拿到用户标识 (在 jwt 中配置了 Auth, 可以取出 user_code)
	// 这里假设 userCode 会放在 header 或者通过 context 传入。
	// 为了跑通，可以先写死一个 admin 或者从 ctx 拿。
	// 实际上由 JWT 中间件设置在 context 中的 user_code
	uid, _ := l.ctx.Value("user_code").(string)
	if uid == "" {
		uid = "admin" // fallback for local test
	}

	rpcResp, err := l.svcCtx.IdentityRpc.GetUserInfo(l.ctx, &identityservice.GetUserInfoReq{
		UserCode: uid,
	})
	if err != nil {
		return nil, err
	}

	return &types.UserInfoResp{
		User: types.UserEntity{
			UserCode: rpcResp.User.UserCode,
			Username: rpcResp.User.Username,
			Avatar:   rpcResp.User.Avatar,
		},
		Roles: rpcResp.Roles,
		Perms: rpcResp.Perms,
	}, nil
}
