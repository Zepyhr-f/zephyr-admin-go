// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"net/http"
	"time"

	"zephyr-go/app/gateway/internal/logic/auth"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/pkg/core/response"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 用户登录
func LoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}

		l := auth.NewLoginLogic(r.Context(), svcCtx)
		resp, err := l.Login(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			// 将 RefreshToken 植入 HttpOnly Cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "refresh_token",
				Value:    resp.RefreshToken,
				Path:     "/",
				HttpOnly: true,
				Secure:   true,                                                    // 生产环境应为 true (HTTPS)
				Expires:  time.Unix(resp.AccessExpire, 0).Add(7 * 24 * time.Hour), // 也可以从配置读取
			})

			response.Success(w, resp)
		}
	}
}
