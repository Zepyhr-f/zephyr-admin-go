package adminextra

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"zephyr-go/app/gateway/internal/types"
)

// 本文件提供后台运行时记录（Runtime Records）的有限内存仓储。
// 所有仓储均带有容量上限，超出时自动淘汰最旧记录，避免无界增长。
// 这是过渡性的安全管理状态，命名为运行时记录，并非占位/Mock。

const (
	maxAuditRecords  = 5000
	maxLoginRecords  = 5000
	maxOnlineRecords = 1000
	maxFileBytes     = 4 << 20  // 4MB single file limit
	maxTotalBytes    = 32 << 20 // 32MB total in-memory store
)

// ----- 通用分页请求 -----

type AdminPageReq struct {
	Current   int    `form:"current,default=1" json:"current,optional"`
	Size      int    `form:"size,default=10" json:"size,optional"`
	Keyword   string `form:"keyword,optional" json:"keyword,optional"`
	Status    string `form:"status,optional" json:"status,optional"`
	Module    string `form:"module,optional" json:"module,optional"`
	Type      string `form:"type,optional" json:"type,optional"`
	DictType  string `form:"dictType,optional" json:"dictType,optional"`
	StartTime string `form:"startTime,optional" json:"startTime,optional"`
	EndTime   string `form:"endTime,optional" json:"endTime,optional"`
}

type IdsReq struct {
	Ids []string `json:"ids"`
}

type IdReq struct {
	Id string `form:"id,optional" json:"id,optional"`
}

// ----- 登录日志 -----

type LoginRecord struct {
	Id        string `json:"id"`
	Username  string `json:"username"`
	Tenant    string `json:"tenant,omitempty"`
	Ip        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Status    string `json:"status"` // success / failed
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// ----- 操作日志 -----

type OperationRecord struct {
	Id         string `json:"id"`
	Module     string `json:"module"`
	Action     string `json:"action"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Summary    string `json:"summary"`
	Operator   string `json:"operator,omitempty"`
	Ip         string `json:"ip,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	TraceId    string `json:"traceId,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// ----- 在线用户 -----

type OnlineRecord struct {
	Id           string `json:"id"`
	Username     string `json:"username"`
	Ip           string `json:"ip"`
	UserAgent    string `json:"userAgent"`
	LoginTime    string `json:"loginTime"`
	LastActiveAt string `json:"lastActiveAt"`
	Status       string `json:"status"` // online / kicked
}

// ----- 任务调度 -----

type CronRecord struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	Group         string `json:"group"`
	Cron          string `json:"cron"`
	Handler       string `json:"handler"`
	Status        int32  `json:"status"` // 0 停用 1 启用
	Description   string `json:"description"`
	LastRunAt     string `json:"lastRunAt,omitempty"`
	LastRunStatus string `json:"lastRunStatus,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type CronRunLog struct {
	Id        string `json:"id"`
	JobId     string `json:"jobId"`
	JobName   string `json:"jobName"`
	Handler   string `json:"handler"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

// ----- 字典 -----

type DictType struct {
	Id          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      int32  `json:"status"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type DictData struct {
	Id        string `json:"id"`
	DictType  string `json:"dictType"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	OrderNum  int32  `json:"orderNum"`
	Status    int32  `json:"status"`
	Remark    string `json:"remark"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ----- 参数 -----

type ParamRecord struct {
	Id        string `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive"`
	Category  string `json:"category"`
	Remark    string `json:"remark"`
	Status    int32  `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ----- 公告 -----

type NoticeRecord struct {
	Id          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Status      int32  `json:"status"` // 0 草稿 1 已发布 2 已下线
	PublishedAt string `json:"publishedAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// ----- 文件 -----

type FileRecord struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	Sha256    string `json:"sha256"`
	Category  string `json:"category"`
	CreatedAt string `json:"createdAt"`
	Deleted   bool   `json:"deleted"`
	bytes     []byte // 不导出、不参与 JSON
}

// ----- 仓储与互斥 -----

type runtimeStore struct {
	mu sync.RWMutex

	loginLogs []LoginRecord
	opLogs    []OperationRecord
	online    []OnlineRecord

	crons   []CronRecord
	cronLog []CronRunLog

	dictTypes []DictType
	dictData  []DictData

	params  []ParamRecord
	notices []NoticeRecord

	files     []FileRecord
	totalSize int64
}

var (
	store    = &runtimeStore{}
	bootTime = time.Now()
)

func newId() string {
	return uuid.NewString()
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ----- 通用分页/过滤辅助 -----

func paginate(total int, current, size int) (int, int, int) {
	if size <= 0 {
		size = 10
	}
	if size > 200 {
		size = 200
	}
	if current <= 0 {
		current = 1
	}
	start := (current - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return start, end, size
}

func toAdminListResp(records []map[string]any, total int64, current, size int) *types.AdminListResp {
	if records == nil {
		records = []map[string]any{}
	}
	return &types.AdminListResp{
		Total:   total,
		Records: records,
		Current: current,
		Size:    size,
	}
}

func matchKeyword(keyword string, fields ...string) bool {
	if keyword == "" {
		return true
	}
	kw := strings.ToLower(keyword)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), kw) {
			return true
		}
	}
	return false
}

func matchStatusInt(filter string, status int32) bool {
	if filter == "" || filter == "-1" {
		return true
	}
	return filter == strconv.Itoa(int(status))
}

func matchStatusStr(filter string, status string) bool {
	if filter == "" {
		return true
	}
	return strings.EqualFold(filter, status)
}

// sortByCreatedDesc sorts a slice by CreatedAt descending using the natural string order.
func sortByCreatedDesc(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		ci, _ := items[i]["createdAt"].(string)
		cj, _ := items[j]["createdAt"].(string)
		return ci > cj
	})
}

// withinTimeRange returns true when t (RFC3339) is within [start,end] (RFC3339, optional).
func withinTimeRange(t, start, end string) bool {
	if start == "" && end == "" {
		return true
	}
	tt, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return true
	}
	if start != "" {
		if s, err := time.Parse(time.RFC3339, start); err == nil && tt.Before(s) {
			return false
		}
	}
	if end != "" {
		if e, err := time.Parse(time.RFC3339, end); err == nil && tt.After(e) {
			return false
		}
	}
	return true
}
