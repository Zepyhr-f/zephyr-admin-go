package adminextra

import (
	"errors"
	"sort"
	"strings"
	"time"

	"zephyr-go/app/gateway/internal/types"
)

var dangerousSQLKeywords = []string{
	"insert", "update", "delete", "drop", "alter", "truncate", "create", "replace", "grant", "revoke", "copy", "execute", "call", "do", "vacuum", "analyze",
}

type SQLExecuteReq struct {
	SQL string `json:"sql"`
}

func CodegenList(req AdminPageReq) *types.AdminListResp {
	items := []map[string]any{
		{
			"tableName":   "zephyr_sys_menu",
			"module":      "system",
			"description": "菜单表，可生成树表 CRUD 模板预览",
			"safeMode":    "preview-download-only",
			"createdAt":   bootTime.UTC().Format(time.RFC3339),
		},
		{
			"tableName":   "runtime_admin_records",
			"module":      "adminextra",
			"description": "后台运行时记录聚合，仅用于安全版代码生成预览",
			"safeMode":    "preview-download-only",
			"createdAt":   bootTime.UTC().Format(time.RFC3339),
		},
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if matchKeyword(req.Keyword, item["tableName"].(string), item["module"].(string), item["description"].(string)) {
			filtered = append(filtered, item)
		}
	}
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func CodegenPreview(tableName string) (map[string]any, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return nil, errors.New("表名不能为空")
	}
	if !safeIdentifier(tableName) {
		return nil, errors.New("表名包含非法字符")
	}
	entity := toPascal(tableName)
	return map[string]any{
		"tableName": tableName,
		"safeMode":  "preview-download-only",
		"files": []map[string]any{
			{"path": "internal/model/" + tableName + ".go", "language": "go", "content": "type " + entity + " struct {\n    Id string `json:\"id\"`\n}\n"},
			{"path": "src/pages/generated/" + entity + ".tsx", "language": "tsx", "content": "// 预览文件：生产版仅下载，不直接覆盖仓库\n"},
		},
		"generatedAt": nowStr(),
	}, nil
}

func CodegenDownload(tableName string) (map[string]any, error) {
	preview, err := CodegenPreview(tableName)
	if err != nil {
		return nil, err
	}
	preview["downloadName"] = tableName + "-codegen-preview.zip"
	preview["downloadMode"] = "metadata-only"
	preview["note"] = "当前安全版返回生成包元数据，前端可据此触发后续下载服务；不会覆盖业务仓库"
	return preview, nil
}

func ApiDocInfo() map[string]any {
	groups := []map[string]any{
		{"group": "security", "prefix": "/api/v1/security", "status": "enabled", "description": "安全审计接口"},
		{"group": "monitor", "prefix": "/api/v1/monitor", "status": "enabled", "description": "系统监控接口"},
		{"group": "infrastructure", "prefix": "/api/v1/infrastructure", "status": "enabled", "description": "基础设施管理接口"},
		{"group": "devtools", "prefix": "/api/v1/devtools", "status": "safe-mode", "description": "开发工具安全版接口"},
	}
	return map[string]any{
		"status":      "enabled",
		"generatedAt": nowStr(),
		"groups":      groups,
		"policy":      "接口文档由 Gateway 路由聚合，不暴露内部密钥、连接串或宿主机路径",
	}
}

func SQLStatus() map[string]any {
	return map[string]any{
		"enabled":        true,
		"mode":           "readonly-validated",
		"allowed":        []string{"SELECT", "EXPLAIN"},
		"maxRows":        100,
		"timeoutSeconds": 3,
		"audit":          true,
		"connected":      false,
		"note":           "Gateway 安全版先执行语法和风险校验；未注入数据库句柄时不执行真实 SQL",
		"checkedAt":      nowStr(),
	}
}

func SQLExecute(sql string) (map[string]any, error) {
	cleaned, err := validateReadOnlySQL(sql)
	if err != nil {
		AppendOperationLog(OperationRecord{Module: "devtools.sql", Action: "execute", Method: "POST", Path: "/api/v1/devtools/sql/execute", Summary: err.Error(), Status: "rejected"})
		return nil, err
	}
	AppendOperationLog(OperationRecord{Module: "devtools.sql", Action: "execute", Method: "POST", Path: "/api/v1/devtools/sql/execute", Summary: "readonly sql validated", Status: "accepted"})
	return map[string]any{
		"columns":    []string{},
		"rows":       []map[string]any{},
		"rowCount":   0,
		"durationMs": 0,
		"executed":   false,
		"validated":  true,
		"sqlDigest":  digestSQL(cleaned),
		"message":    "SQL 已通过只读校验；当前 Gateway 未注入数据库执行句柄，生产执行需接入只读数据源后启用",
	}, nil
}

func validateReadOnlySQL(sql string) (string, error) {
	cleaned := strings.TrimSpace(sql)
	if cleaned == "" {
		return "", errors.New("SQL 不能为空")
	}
	if strings.Count(cleaned, ";") > 0 {
		trimmed := strings.TrimRight(cleaned, "; \t\n\r")
		if strings.Contains(trimmed, ";") || trimmed != strings.TrimSuffix(cleaned, ";") {
			return "", errors.New("SQL 终端禁止多语句")
		}
		cleaned = trimmed
	}
	lower := strings.ToLower(cleaned)
	if !(strings.HasPrefix(lower, "select ") || strings.HasPrefix(lower, "explain ")) {
		return "", errors.New("SQL 终端仅允许 SELECT 或 EXPLAIN")
	}
	words := strings.FieldsFunc(lower, func(r rune) bool { return !(r >= 'a' && r <= 'z') })
	wordSet := map[string]struct{}{}
	for _, w := range words {
		wordSet[w] = struct{}{}
	}
	for _, k := range dangerousSQLKeywords {
		if _, ok := wordSet[k]; ok {
			return "", errors.New("SQL 包含禁止关键字: " + k)
		}
	}
	return cleaned, nil
}

func digestSQL(sql string) string {
	s := strings.Join(strings.Fields(sql), " ")
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func safeIdentifier(v string) bool {
	for _, r := range v {
		if !(r == '_' || r == '-' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

func toPascal(v string) string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == '_' || r == '-' || r == '.' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	if len(parts) == 0 {
		return "GeneratedEntity"
	}
	return strings.Join(parts, "")
}

var _ = sort.Strings
