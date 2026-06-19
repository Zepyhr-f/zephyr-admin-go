package adminextra

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/httpx"
	"zephyr-go/app/gateway/internal/logic/adminextra"
	"zephyr-go/app/gateway/internal/svc"
	"zephyr-go/app/gateway/internal/types"
	"zephyr-go/pkg/core/response"
)

func MenuListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := adminextra.MenuList(r.Context(), svcCtx.IdentityRpc)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, resp)
	}
}

func MenuDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CodeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		resp, err := adminextra.MenuDetail(r.Context(), svcCtx.IdentityRpc, req.Code)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, resp)
	}
}

func MenuSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MenuSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.MenuSave(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func MenuUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MenuUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.MenuUpdate(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func MenuRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MenuRemoveReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.MenuRemove(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func MenuStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MenuStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.MenuStatus(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func RoleDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CodeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		resp, err := adminextra.RoleDetail(r.Context(), svcCtx.IdentityRpc, req.Code)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, resp)
	}
}

func RoleMenuTreeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CodeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		resp, err := adminextra.RoleMenuTree(r.Context(), svcCtx.IdentityRpc, req.Code)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, resp)
	}
}

func RoleAssignMenusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RoleAssignMenusReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.AssignRoleMenus(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func RoleDataScopeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RoleDataScopeReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if err := adminextra.RoleDataScope(r.Context(), svcCtx.IdentityRpc, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func ServerMetricsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, adminextra.ServerSummary(svcCtx.Config))
	}
}
func CacheMetricsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.CacheSummary()) }
}
func DatasourceMetricsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.DatasourceSummary()) }
}

func LoginLogListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.LoginLogList(req) })
}
func LoginLogRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.LoginLogRemove)
}
func LoginLogClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, map[string]any{"removed": adminextra.LoginLogClear()})
	}
}
func OperationLogListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.OperationLogList(req) })
}
func OperationLogDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		item, ok := adminextra.OperationLogDetail(id)
		if !ok {
			response.Error(w, errors.New("操作日志不存在"))
			return
		}
		response.Success(w, item)
	}
}
func OperationLogRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.OperationLogRemove)
}
func OperationLogClearHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, map[string]any{"removed": adminextra.OperationLogClear()})
	}
}
func OnlineListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.OnlineList(req) })
}
func OnlineKickoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.OnlineKickout)
}

func CronListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.CronList(req) })
}
func CronHandlersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.CronHandlers()) }
}
func CronSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return jsonSaveHandler[adminextra.CronSaveReq](adminextra.CronSave)
}
func CronStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idStatusHandler(adminextra.CronStatus)
}
func CronRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.CronRemove)
}
func CronRunHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := parseID(r)
		out, err := adminextra.CronRunOnce(id)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}
func CronLogsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.CronLogs(req) })
}

func DictTypeListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.DictTypeList(req) })
}
func DictTypeSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return jsonSaveHandler[adminextra.DictTypeSaveReq](adminextra.DictTypeSave)
}
func DictTypeRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.DictTypeRemove)
}
func DictTypeStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idStatusHandler(adminextra.DictTypeStatus)
}
func DictDataListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.DictDataList(req) })
}
func DictDataSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return jsonSaveHandler[adminextra.DictDataSaveReq](adminextra.DictDataSave)
}
func DictDataRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.DictDataRemove)
}
func DictDataStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idStatusHandler(adminextra.DictDataStatus)
}

func ParamsListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.ParamList(req) })
}
func ParamsSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return jsonSaveHandler[adminextra.ParamSaveReq](adminextra.ParamSave)
}
func ParamsRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.ParamRemove)
}
func ParamsRefreshCacheHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.ParamRefreshCache()) }
}

func FilesListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.FileList(req) })
}
func FilesUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			response.Error(w, err)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			response.Error(w, err)
			return
		}
		_ = file.Close()
		fh := r.MultipartForm.File["file"][0]
		out, err := adminextra.FileUpload(fh, r.FormValue("category"))
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}
func FilesDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		data, meta, ok := adminextra.FileRead(id)
		if !ok {
			response.Error(w, errors.New("文件不存在"))
			return
		}
		w.Header().Set("Content-Type", meta.MimeType)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+meta.Name+"\"")
		_, _ = w.Write(data)
	}
}
func FilesRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.FileSoftDelete)
}

func NoticesListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.NoticeList(req) })
}
func NoticesSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return jsonSaveHandler[adminextra.NoticeSaveReq](adminextra.NoticeSave)
}
func NoticesRemoveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return idsActionHandler(adminextra.NoticeRemove)
}
func NoticesPublishHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := parseID(r)
		publish := r.URL.Query().Get("publish") != "false"
		var body struct {
			Id      string `json:"id"`
			Publish *bool  `json:"publish,optional"`
		}
		_ = httpx.Parse(r, &body)
		if body.Id != "" {
			id = body.Id
		}
		if body.Publish != nil {
			publish = *body.Publish
		}
		if err := adminextra.NoticePublish(id, publish); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func CodegenListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return listHandler(func(req adminextra.AdminPageReq) any { return adminextra.CodegenList(req) })
}
func CodegenPreviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.URL.Query().Get("tableName")
		var body struct {
			TableName string `json:"tableName"`
		}
		_ = httpx.Parse(r, &body)
		if body.TableName != "" {
			table = body.TableName
		}
		out, err := adminextra.CodegenPreview(table)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}
func CodegenDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.URL.Query().Get("tableName")
		var body struct {
			TableName string `json:"tableName"`
		}
		_ = httpx.Parse(r, &body)
		if body.TableName != "" {
			table = body.TableName
		}
		out, err := adminextra.CodegenDownload(table)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}
func ApiDocInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.ApiDocInfo()) }
}
func SQLStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { response.Success(w, adminextra.SQLStatus()) }
}
func SQLExecuteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminextra.SQLExecuteReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		out, err := adminextra.SQLExecute(req.SQL)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}

func listHandler(fn func(adminextra.AdminPageReq) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminextra.AdminPageReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, fn(req))
	}
}

func idsActionHandler(fn func([]string) int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminextra.IdsReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, map[string]any{"affected": fn(req.Ids)})
	}
}

func idStatusHandler(fn func(string, int32) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Id     string `json:"id" form:"id,optional"`
			Status int32  `json:"status" form:"status,optional"`
		}
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		if req.Id == "" {
			req.Id = r.URL.Query().Get("id")
		}
		if err := fn(req.Id, req.Status); err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, nil)
	}
}

func jsonSaveHandler[T any](fn func(T) (map[string]any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req T
		if err := httpx.Parse(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		out, err := fn(req)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Success(w, out)
	}
}

func parseID(r *http.Request) string {
	id := r.URL.Query().Get("id")
	if id != "" {
		return id
	}
	var body struct {
		Id string `json:"id"`
	}
	_ = httpx.Parse(r, &body)
	if body.Id != "" {
		return body.Id
	}
	if n, err := strconv.ParseInt(r.URL.Query().Get("jobId"), 10, 64); err == nil && n > 0 {
		return strconv.FormatInt(n, 10)
	}
	return ""
}
