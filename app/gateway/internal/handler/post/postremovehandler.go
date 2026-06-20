// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package post

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"zephyr-go/app/gateway/internal/logic/post"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/pkg/core/response"
)

func PostRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PostRemoveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		l := post.NewPostRemoveLogic(r.Context(), svcCtx)
		err := l.PostRemove(&req)
		if err != nil {
			response.Error(w, err)
		} else {
			response.Success(w, nil)
		}
	}
}
