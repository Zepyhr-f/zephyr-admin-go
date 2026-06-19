package adminextra

import (
	"errors"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"zephyr-go/app/gateway/internal/config"
	"zephyr-go/app/gateway/internal/types"
)

// ServerSummary 返回 gateway 进程级别的运行时摘要。
// 不暴露宿主机绝对路径、密钥、容器配置。
func ServerSummary(cfg config.Config) map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	hostname, _ := os.Hostname()

	uptime := time.Since(bootTime)
	rpcStatus := map[string]any{
		"auth":     probeRpcTarget(cfg.AuthRpc.Target),
		"identity": probeRpcTarget(cfg.IdentityRpc.Target),
	}

	return map[string]any{
		"app":           "zephyr-gateway",
		"runtime":       "go",
		"goVersion":     runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"numCpu":        runtime.NumCPU(),
		"hostname":      maskHost(hostname),
		"uptimeSeconds": int64(uptime.Seconds()),
		"startedAt":     bootTime.UTC().Format(time.RFC3339),
		"now":           nowStr(),
		"memory": map[string]any{
			"allocBytes":      m.Alloc,
			"totalAllocBytes": m.TotalAlloc,
			"sysBytes":        m.Sys,
			"heapInUseBytes":  m.HeapInuse,
			"numGc":           m.NumGC,
		},
		"rpc":    rpcStatus,
		"status": "ok",
	}
}

// probeRpcTarget 探测 zrpc target 的 TCP 可达性。仅做连通性，不做实际 grpc 握手，避免阻塞。
func probeRpcTarget(target string) map[string]any {
	out := map[string]any{
		"target":    maskTarget(target),
		"reachable": "unknown",
		"rttMs":     int64(0),
	}
	if target == "" {
		return out
	}
	host, port := parseHostPort(target)
	if host == "" || port == "" {
		return out
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 600*time.Millisecond)
	if err == nil {
		out["reachable"] = "tcp-ok"
		out["rttMs"] = time.Since(start).Milliseconds()
		_ = conn.Close()
	} else {
		out["reachable"] = "tcp-failed"
	}
	return out
}

func parseHostPort(target string) (string, string) {
	// 支持 host:port 或 dns:///host:port / direct:host:port 等去前缀
	if idx := strings.LastIndex(target, "/"); idx >= 0 {
		target = target[idx+1:]
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", ""
	}
	return host, port
}

func maskTarget(t string) string {
	host, port := parseHostPort(t)
	if host == "" {
		return "***"
	}
	return maskHost(host) + ":" + port
}

// CacheSummary 在网关层未注入 Redis 客户端时，返回禁用状态而不是占位文本。
func CacheSummary() map[string]any {
	host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	if host == "" {
		host = strings.TrimSpace(os.Getenv("ZEPHYR_REDIS_HOST"))
	}
	enabled := false
	reachable := "unknown"
	var rtt int64

	if host != "" {
		port := strings.TrimSpace(os.Getenv("REDIS_PORT"))
		if port == "" {
			port = "6379"
		}
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 800*time.Millisecond)
		if err == nil {
			rtt = time.Since(start).Milliseconds()
			reachable = "tcp-ok"
			enabled = true
			_ = conn.Close()
		} else {
			reachable = "tcp-failed"
		}
	}

	return map[string]any{
		"enabled":          enabled,
		"reachable":        reachable,
		"rttMs":            rtt,
		"host":             maskHost(host),
		"protocol":         "redis",
		"checkedAt":        nowStr(),
		"managementPolicy": "网关层不直连 Redis，仅做 TCP 探测；详细键空间由专用监控服务采集",
	}
}

// DatasourceSummary 同样在网关层不暴露连接串、用户名密码。
// 仅探测 host:port 连通性，作为生产可用的安全摘要。
func DatasourceSummary() map[string]any {
	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	if host == "" {
		host = strings.TrimSpace(os.Getenv("ZEPHYR_PG_HOST"))
	}
	enabled := false
	reachable := "unknown"
	var rtt int64

	if host != "" {
		port := strings.TrimSpace(os.Getenv("POSTGRES_PORT"))
		if port == "" {
			port = "5432"
		}
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 800*time.Millisecond)
		if err == nil {
			rtt = time.Since(start).Milliseconds()
			reachable = "tcp-ok"
			enabled = true
			_ = conn.Close()
		} else {
			reachable = "tcp-failed"
		}
	}
	return map[string]any{
		"enabled":          enabled,
		"reachable":        reachable,
		"rttMs":            rtt,
		"host":             maskHost(host),
		"engine":           "postgresql",
		"checkedAt":        nowStr(),
		"managementPolicy": "网关层不直连数据库，仅做 TCP 探测；详细指标由 identity 服务上报",
	}
}

func maskHost(host string) string {
	if host == "" {
		return ""
	}
	if strings.Contains(host, ".") {
		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			parts[0] = "***"
			return strings.Join(parts, ".")
		}
	}
	if len(host) <= 4 {
		return "***"
	}
	return host[:2] + "***"
}

// ----- 任务调度 -----

// 内置只读白名单 handler，保证不允许任意 shell。
var allowedCronHandlers = map[string]string{
	"system.heartbeat":     "记录系统心跳到运行日志",
	"system.cleanup-cache": "清理过期缓存运行记录",
	"audit.rotate":         "轮转操作日志运行记录",
}

func CronHandlers() []map[string]any {
	keys := make([]string, 0, len(allowedCronHandlers))
	for k := range allowedCronHandlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{"handler": k, "description": allowedCronHandlers[k]})
	}
	return out
}

func CronList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]CronRecord, len(store.crons))
	copy(src, store.crons)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, c := range src {
		if !matchKeyword(req.Keyword, c.Name, c.Group, c.Handler) {
			continue
		}
		if !matchStatusInt(req.Status, c.Status) {
			continue
		}
		filtered = append(filtered, cronToMap(c))
	}
	sortByCreatedDesc(filtered)
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

type CronSaveReq struct {
	Id          string `json:"id,optional"`
	Name        string `json:"name"`
	Group       string `json:"group,optional"`
	Cron        string `json:"cron"`
	Handler     string `json:"handler"`
	Status      int32  `json:"status,optional"`
	Description string `json:"description,optional"`
}

func CronSave(req CronSaveReq) (map[string]any, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("任务名称不能为空")
	}
	if !validCronExpr(req.Cron) {
		return nil, errors.New("cron 表达式无效，需 5-7 段空格分隔")
	}
	if _, ok := allowedCronHandlers[req.Handler]; !ok {
		return nil, errors.New("handler 不在白名单内")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	now := nowStr()
	if req.Id != "" {
		for i := range store.crons {
			if store.crons[i].Id == req.Id {
				store.crons[i].Name = req.Name
				store.crons[i].Group = req.Group
				store.crons[i].Cron = req.Cron
				store.crons[i].Handler = req.Handler
				store.crons[i].Status = req.Status
				store.crons[i].Description = req.Description
				store.crons[i].UpdatedAt = now
				return cronToMap(store.crons[i]), nil
			}
		}
		return nil, errors.New("任务不存在")
	}

	// 名称唯一
	for _, c := range store.crons {
		if strings.EqualFold(c.Name, req.Name) {
			return nil, errors.New("任务名称已存在")
		}
	}

	rec := CronRecord{
		Id:          newId(),
		Name:        req.Name,
		Group:       req.Group,
		Cron:        req.Cron,
		Handler:     req.Handler,
		Status:      req.Status,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.crons = append(store.crons, rec)
	return cronToMap(rec), nil
}

func CronStatus(id string, status int32) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := range store.crons {
		if store.crons[i].Id == id {
			store.crons[i].Status = status
			store.crons[i].UpdatedAt = nowStr()
			return nil
		}
	}
	return errors.New("任务不存在")
}

func CronRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.crons[:0]
	removed := 0
	for _, c := range store.crons {
		if _, ok := idSet[c.Id]; ok {
			removed++
			continue
		}
		out = append(out, c)
	}
	store.crons = out
	return removed
}

// CronRunOnce 在白名单 handler 下执行一次安全动作并落运行日志。
func CronRunOnce(id string) (map[string]any, error) {
	store.mu.Lock()
	var rec CronRecord
	found := false
	for i := range store.crons {
		if store.crons[i].Id == id {
			rec = store.crons[i]
			found = true
			break
		}
	}
	store.mu.Unlock()
	if !found {
		return nil, errors.New("任务不存在")
	}

	start := time.Now()
	status := "success"
	message, err := runWhitelistedJob(rec.Handler)
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	end := time.Now()

	log := CronRunLog{
		Id:        newId(),
		JobId:     rec.Id,
		JobName:   rec.Name,
		Handler:   rec.Handler,
		Status:    status,
		Message:   truncate(message, 512),
		StartedAt: start.UTC().Format(time.RFC3339),
		EndedAt:   end.UTC().Format(time.RFC3339),
	}

	store.mu.Lock()
	for i := range store.crons {
		if store.crons[i].Id == rec.Id {
			store.crons[i].LastRunAt = log.EndedAt
			store.crons[i].LastRunStatus = status
			break
		}
	}
	store.cronLog = append(store.cronLog, log)
	if n := len(store.cronLog); n > maxAuditRecords {
		store.cronLog = store.cronLog[n-maxAuditRecords:]
	}
	store.mu.Unlock()

	return map[string]any{
		"jobId":      log.JobId,
		"status":     log.Status,
		"message":    log.Message,
		"startedAt":  log.StartedAt,
		"endedAt":    log.EndedAt,
		"durationMs": end.Sub(start).Milliseconds(),
	}, nil
}

func CronLogs(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]CronRunLog, len(store.cronLog))
	copy(src, store.cronLog)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		l := src[i]
		if !matchKeyword(req.Keyword, l.JobName, l.Handler) {
			continue
		}
		if !matchStatusStr(req.Status, l.Status) {
			continue
		}
		filtered = append(filtered, map[string]any{
			"id":        l.Id,
			"jobId":     l.JobId,
			"jobName":   l.JobName,
			"handler":   l.Handler,
			"status":    l.Status,
			"message":   l.Message,
			"startedAt": l.StartedAt,
			"endedAt":   l.EndedAt,
		})
	}
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func cronToMap(c CronRecord) map[string]any {
	return map[string]any{
		"id":            c.Id,
		"name":          c.Name,
		"group":         c.Group,
		"cron":          c.Cron,
		"handler":       c.Handler,
		"status":        c.Status,
		"description":   c.Description,
		"lastRunAt":     c.LastRunAt,
		"lastRunStatus": c.LastRunStatus,
		"createdAt":     c.CreatedAt,
		"updatedAt":     c.UpdatedAt,
	}
}

// validCronExpr 仅做基础结构校验：5-7 段，每段非空。
// 真正的调度执行由专门 worker 完成，这里负责持久化前的语法防护。
func validCronExpr(expr string) bool {
	parts := strings.Fields(expr)
	if len(parts) < 5 || len(parts) > 7 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}

func runWhitelistedJob(handler string) (string, error) {
	switch handler {
	case "system.heartbeat":
		return "heartbeat=" + strconv.FormatInt(time.Now().Unix(), 10), nil
	case "system.cleanup-cache":
		store.mu.Lock()
		defer store.mu.Unlock()
		// 仅清理 cron 运行日志的过期部分；不触碰业务数据。
		cutoff := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
		kept := store.cronLog[:0]
		for _, l := range store.cronLog {
			if l.EndedAt >= cutoff {
				kept = append(kept, l)
			}
		}
		store.cronLog = kept
		return "cleanup-ok", nil
	case "audit.rotate":
		store.mu.Lock()
		defer store.mu.Unlock()
		// 轮转：保留最近 1000 条审计/登录记录。
		if len(store.opLogs) > 1000 {
			store.opLogs = store.opLogs[len(store.opLogs)-1000:]
		}
		if len(store.loginLogs) > 1000 {
			store.loginLogs = store.loginLogs[len(store.loginLogs)-1000:]
		}
		return "rotate-ok", nil
	}
	return "", errors.New("handler 未在白名单内")
}

var _ = sort.Strings
