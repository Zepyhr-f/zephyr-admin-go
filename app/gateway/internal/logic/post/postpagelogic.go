// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package post

import (
	"context"

	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/app/identity/identityservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type PostPageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPostPageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PostPageLogic {
	return &PostPageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PostPageLogic) PostPage(req *types.PostPageReq) (resp *types.PostPageResp, err error) {
	rpcResp, err := l.svcCtx.IdentityRpc.PostPage(l.ctx, &identityservice.PostPageReq{
		Current:  int32(req.Current),
		Size:     int32(req.Size),
		PostName: req.PostName,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}

	var records []types.PostDetail
	for _, v := range rpcResp.Records {
		records = append(records, types.PostDetail{
			Id:         v.Id,
			Code:       v.Code,
			PostName:   v.PostName,
			OrderNum:   v.OrderNum,
			Status:     v.Status,
			CreateTime: v.CreateTime,
		})
	}

	return &types.PostPageResp{
		Total:   rpcResp.Total,
		Records: records,
		Size:    int(rpcResp.Size),
		Current: int(rpcResp.Current),
		Pages:   int(rpcResp.Pages),
	}, nil
}
