package adminextra

import (
	"net/http"
	"strings"
	"time"

	"zephyr-go/app/gateway/internal/types"
)

// 登录日志：使用运行时记录仓储。
// Append 由认证逻辑在登录成功/失败处调用，本模块只暴露查询/清理。

func AppendLoginLog(rec LoginRecord) {
	if rec.Id == "" {
		rec.Id = newId()
	}
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowStr()
	}
	rec.UserAgent = truncate(rec.UserAgent, 256)
	rec.Reason = truncate(rec.Reason, 200)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.loginLogs = append(store.loginLogs, rec)
	if n := len(store.loginLogs); n > maxLoginRecords {
		store.loginLogs = store.loginLogs[n-maxLoginRecords:]
	}
}

func LoginLogList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]LoginRecord, len(store.loginLogs))
	copy(src, store.loginLogs)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- { // 倒序，最新在前
		r := src[i]
		if !matchKeyword(req.Keyword, r.Username, r.Ip) {
			continue
		}
		if !matchStatusStr(req.Status, r.Status) {
			continue
		}
		if !withinTimeRange(r.CreatedAt, req.StartTime, req.EndTime) {
			continue
		}
		filtered = append(filtered, map[string]any{
			"id":        r.Id,
			"username":  r.Username,
			"tenant":    r.Tenant,
			"ip":        r.Ip,
			"userAgent": r.UserAgent,
			"status":    r.Status,
			"reason":    r.Reason,
			"createdAt": r.CreatedAt,
		})
	}
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func LoginLogRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.loginLogs[:0]
	removed := 0
	for _, r := range store.loginLogs {
		if _, ok := idSet[r.Id]; ok {
			removed++
			continue
		}
		out = append(out, r)
	}
	store.loginLogs = out
	return removed
}

func LoginLogClear() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	n := len(store.loginLogs)
	store.loginLogs = nil
	return n
}

// 操作日志：由 AppendOperationLog 主动写入，路由级别审计中间件可使用。

func AppendOperationLog(rec OperationRecord) {
	if rec.Id == "" {
		rec.Id = newId()
	}
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowStr()
	}
	rec.Summary = truncate(rec.Summary, 1024)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.opLogs = append(store.opLogs, rec)
	if n := len(store.opLogs); n > maxAuditRecords {
		store.opLogs = store.opLogs[n-maxAuditRecords:]
	}
}

func OperationLogList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]OperationRecord, len(store.opLogs))
	copy(src, store.opLogs)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		r := src[i]
		if !matchKeyword(req.Keyword, r.Module, r.Action, r.Path, r.Operator) {
			continue
		}
		if req.Module != "" && !strings.EqualFold(req.Module, r.Module) {
			continue
		}
		if !matchStatusStr(req.Status, r.Status) {
			continue
		}
		if !withinTimeRange(r.CreatedAt, req.StartTime, req.EndTime) {
			continue
		}
		filtered = append(filtered, opRecordToMap(r))
	}
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func OperationLogDetail(id string) (map[string]any, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, r := range store.opLogs {
		if r.Id == id {
			return opRecordToMap(r), true
		}
	}
	return nil, false
}

func OperationLogRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.opLogs[:0]
	removed := 0
	for _, r := range store.opLogs {
		if _, ok := idSet[r.Id]; ok {
			removed++
			continue
		}
		out = append(out, r)
	}
	store.opLogs = out
	return removed
}

func OperationLogClear() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	n := len(store.opLogs)
	store.opLogs = nil
	return n
}

func opRecordToMap(r OperationRecord) map[string]any {
	return map[string]any{
		"id":         r.Id,
		"module":     r.Module,
		"action":     r.Action,
		"method":     r.Method,
		"path":       r.Path,
		"summary":    r.Summary,
		"operator":   r.Operator,
		"ip":         r.Ip,
		"status":     r.Status,
		"durationMs": r.DurationMs,
		"traceId":    r.TraceId,
		"createdAt":  r.CreatedAt,
	}
}

// 在线用户：由会话登记/心跳维护；超时未活跃自动剔除。
const onlineTimeout = 15 * time.Minute

// TouchOnlineSession 由认证/网关在登录或心跳时调用，可做最小化的运行时会话索引。
func TouchOnlineSession(rec OnlineRecord) {
	if rec.Username == "" {
		return
	}
	if rec.Id == "" {
		rec.Id = newId()
	}
	now := nowStr()
	if rec.LoginTime == "" {
		rec.LoginTime = now
	}
	rec.LastActiveAt = now
	rec.Status = "online"
	rec.UserAgent = truncate(rec.UserAgent, 256)

	store.mu.Lock()
	defer store.mu.Unlock()
	for i, o := range store.online {
		if o.Username == rec.Username && o.Ip == rec.Ip {
			store.online[i].LastActiveAt = now
			store.online[i].UserAgent = rec.UserAgent
			store.online[i].Status = "online"
			return
		}
	}
	store.online = append(store.online, rec)
	if n := len(store.online); n > maxOnlineRecords {
		store.online = store.online[n-maxOnlineRecords:]
	}
}

func OnlineList(req AdminPageReq) *types.AdminListResp {
	cutoff := time.Now().Add(-onlineTimeout).UTC().Format(time.RFC3339)
	store.mu.Lock()
	// 过期清理：状态降级为 expired，但保留近似在线视图
	for i := range store.online {
		if store.online[i].Status == "online" && store.online[i].LastActiveAt < cutoff {
			store.online[i].Status = "expired"
		}
	}
	src := make([]OnlineRecord, len(store.online))
	copy(src, store.online)
	store.mu.Unlock()

	filtered := make([]map[string]any, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		o := src[i]
		if !matchKeyword(req.Keyword, o.Username, o.Ip) {
			continue
		}
		if !matchStatusStr(req.Status, o.Status) {
			continue
		}
		filtered = append(filtered, map[string]any{
			"id":           o.Id,
			"username":     o.Username,
			"ip":           o.Ip,
			"userAgent":    o.UserAgent,
			"loginTime":    o.LoginTime,
			"lastActiveAt": o.LastActiveAt,
			"status":       o.Status,
		})
	}
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func OnlineKickout(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	kicked := 0
	for i := range store.online {
		if _, ok := idSet[store.online[i].Id]; ok {
			store.online[i].Status = "kicked"
			store.online[i].LastActiveAt = nowStr()
			kicked++
		}
	}
	return kicked
}

// ----- 工具 -----

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, i := range items {
		out[i] = struct{}{}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// HTTPClientHint 给上游记录用，避免过度依赖具体上下文。
func HTTPClientHint(r *http.Request) (ip, ua string) {
	if r == nil {
		return "", ""
	}
	ip = r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	if comma := strings.Index(ip, ","); comma > 0 {
		ip = ip[:comma]
	}
	ip = strings.TrimSpace(ip)
	ua = truncate(r.Header.Get("User-Agent"), 256)
	return
}
