// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package menu

import (
	"context"

	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/app/identity/identityservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type MenuTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenuTreeLogic {
	return &MenuTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MenuTreeLogic) MenuTree() (resp *types.MenuTreeResp, err error) {
	rpcResp, err := l.svcCtx.IdentityRpc.MenuTree(l.ctx, &identityservice.EmptyReq{})
	if err != nil {
		return nil, err
	}

	list := make([]types.MenuDetail, 0)
	for _, item := range rpcResp.List {
		list = append(list, mapMenuToTypes(item))
	}

	return &types.MenuTreeResp{
		List: list,
	}, nil
}

func mapMenuToTypes(v *identityservice.MenuDetail) types.MenuDetail {
	m := types.MenuDetail{
		MenuCode:   v.MenuCode,
		ParentCode: v.ParentCode,
		MenuName:   v.MenuName,
		Icon:       v.Icon,
		MenuType:   v.MenuType,
		Perms:      v.Perms,
		Path:       v.Path,
		Component:  v.Component,
		OrderNum:   v.OrderNum,
	}
	if len(v.Children) > 0 {
		m.Children = make([]types.MenuDetail, 0)
		for _, child := range v.Children {
			m.Children = append(m.Children, mapMenuToTypes(child))
		}
	}
	return m
}
