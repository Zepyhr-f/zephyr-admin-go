// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	authpb "zephyr-go/app/auth/pb/pb"
	adminextra "zephyr-go/app/gateway/internal/logic/adminextra"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 用户登录
func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.LoginResp, err error) {
	adminextra.AppendOperationLog(adminextra.OperationRecord{Module: "auth", Action: "login", Method: "POST", Path: "/api/v1/auth/login", Summary: "login attempt", Operator: req.Username, Status: "started"})
	// 调用 Auth 微服务进行密码比对和 Token 签发
	rpcResp, err := l.svcCtx.AuthRpc.LoginVerify(l.ctx, &authpb.LoginVerifyReq{
		Username:   req.Username,
		Password:   req.Password,
		TenantCode: req.TenantCode,
	})
	if err != nil {
		adminextra.AppendLoginLog(adminextra.LoginRecord{Username: req.Username, Tenant: req.TenantCode, Status: "failed", Reason: err.Error()})
		adminextra.AppendOperationLog(adminextra.OperationRecord{Module: "auth", Action: "login", Method: "POST", Path: "/api/v1/auth/login", Summary: "login failed", Operator: req.Username, Status: "failed"})
		return nil, err
	}
	adminextra.AppendLoginLog(adminextra.LoginRecord{Username: req.Username, Tenant: req.TenantCode, Status: "success"})
	adminextra.TouchOnlineSession(adminextra.OnlineRecord{Username: req.Username})
	adminextra.AppendOperationLog(adminextra.OperationRecord{Module: "auth", Action: "login", Method: "POST", Path: "/api/v1/auth/login", Summary: "login success", Operator: req.Username, Status: "success"})

	return &types.LoginResp{
		Token:        rpcResp.AccessToken,
		RefreshToken: rpcResp.RefreshToken,
		AccessExpire: rpcResp.Expire,
	}, nil
}
