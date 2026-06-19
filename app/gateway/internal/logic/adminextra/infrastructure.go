package adminextra

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strings"

	"zephyr-go/app/gateway/internal/types"
)

// ----- 字典类型 -----

type DictTypeSaveReq struct {
	Id          string `json:"id,optional"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      int32  `json:"status,optional"`
	Description string `json:"description,optional"`
}

func DictTypeList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]DictType, len(store.dictTypes))
	copy(src, store.dictTypes)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, d := range src {
		if !matchKeyword(req.Keyword, d.Code, d.Name) {
			continue
		}
		if !matchStatusInt(req.Status, d.Status) {
			continue
		}
		filtered = append(filtered, dictTypeToMap(d))
	}
	sortByCreatedDesc(filtered)
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func DictTypeSave(req DictTypeSaveReq) (map[string]any, error) {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("字典编码与名称不能为空")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := nowStr()

	if req.Id != "" {
		for i := range store.dictTypes {
			if store.dictTypes[i].Id == req.Id {
				// code 唯一
				for j, d := range store.dictTypes {
					if j != i && strings.EqualFold(d.Code, req.Code) {
						return nil, errors.New("字典编码已存在")
					}
				}
				store.dictTypes[i].Code = req.Code
				store.dictTypes[i].Name = req.Name
				store.dictTypes[i].Status = req.Status
				store.dictTypes[i].Description = req.Description
				store.dictTypes[i].UpdatedAt = now
				return dictTypeToMap(store.dictTypes[i]), nil
			}
		}
		return nil, errors.New("字典类型不存在")
	}

	for _, d := range store.dictTypes {
		if strings.EqualFold(d.Code, req.Code) {
			return nil, errors.New("字典编码已存在")
		}
	}
	rec := DictType{
		Id: newId(), Code: req.Code, Name: req.Name, Status: req.Status,
		Description: req.Description, CreatedAt: now, UpdatedAt: now,
	}
	store.dictTypes = append(store.dictTypes, rec)
	return dictTypeToMap(rec), nil
}

func DictTypeRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.dictTypes[:0]
	removed := 0
	codesRemoved := map[string]struct{}{}
	for _, d := range store.dictTypes {
		if _, ok := idSet[d.Id]; ok {
			removed++
			codesRemoved[d.Code] = struct{}{}
			continue
		}
		out = append(out, d)
	}
	store.dictTypes = out
	// 级联删除字典数据
	if len(codesRemoved) > 0 {
		dataOut := store.dictData[:0]
		for _, dd := range store.dictData {
			if _, ok := codesRemoved[dd.DictType]; ok {
				continue
			}
			dataOut = append(dataOut, dd)
		}
		store.dictData = dataOut
	}
	return removed
}

func DictTypeStatus(id string, status int32) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := range store.dictTypes {
		if store.dictTypes[i].Id == id {
			store.dictTypes[i].Status = status
			store.dictTypes[i].UpdatedAt = nowStr()
			return nil
		}
	}
	return errors.New("字典类型不存在")
}

func dictTypeToMap(d DictType) map[string]any {
	return map[string]any{
		"id": d.Id, "code": d.Code, "name": d.Name, "status": d.Status,
		"description": d.Description, "createdAt": d.CreatedAt, "updatedAt": d.UpdatedAt,
	}
}

// ----- 字典数据 -----

type DictDataSaveReq struct {
	Id       string `json:"id,optional"`
	DictType string `json:"dictType"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	OrderNum int32  `json:"orderNum,optional"`
	Status   int32  `json:"status,optional"`
	Remark   string `json:"remark,optional"`
}

func DictDataList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]DictData, len(store.dictData))
	copy(src, store.dictData)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, d := range src {
		if req.DictType != "" && !strings.EqualFold(req.DictType, d.DictType) {
			continue
		}
		if !matchKeyword(req.Keyword, d.Label, d.Value) {
			continue
		}
		if !matchStatusInt(req.Status, d.Status) {
			continue
		}
		filtered = append(filtered, dictDataToMap(d))
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		oi, _ := filtered[i]["orderNum"].(int32)
		oj, _ := filtered[j]["orderNum"].(int32)
		if oi != oj {
			return oi < oj
		}
		ci, _ := filtered[i]["createdAt"].(string)
		cj, _ := filtered[j]["createdAt"].(string)
		return ci > cj
	})
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func DictDataSave(req DictDataSaveReq) (map[string]any, error) {
	if strings.TrimSpace(req.DictType) == "" || strings.TrimSpace(req.Label) == "" {
		return nil, errors.New("字典类型与标签不能为空")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := nowStr()

	if req.Id != "" {
		for i := range store.dictData {
			if store.dictData[i].Id == req.Id {
				store.dictData[i].DictType = req.DictType
				store.dictData[i].Label = req.Label
				store.dictData[i].Value = req.Value
				store.dictData[i].OrderNum = req.OrderNum
				store.dictData[i].Status = req.Status
				store.dictData[i].Remark = req.Remark
				store.dictData[i].UpdatedAt = now
				return dictDataToMap(store.dictData[i]), nil
			}
		}
		return nil, errors.New("字典数据不存在")
	}

	rec := DictData{
		Id: newId(), DictType: req.DictType, Label: req.Label, Value: req.Value,
		OrderNum: req.OrderNum, Status: req.Status, Remark: req.Remark,
		CreatedAt: now, UpdatedAt: now,
	}
	store.dictData = append(store.dictData, rec)
	return dictDataToMap(rec), nil
}

func DictDataRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.dictData[:0]
	removed := 0
	for _, d := range store.dictData {
		if _, ok := idSet[d.Id]; ok {
			removed++
			continue
		}
		out = append(out, d)
	}
	store.dictData = out
	return removed
}

func DictDataStatus(id string, status int32) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := range store.dictData {
		if store.dictData[i].Id == id {
			store.dictData[i].Status = status
			store.dictData[i].UpdatedAt = nowStr()
			return nil
		}
	}
	return errors.New("字典数据不存在")
}

func dictDataToMap(d DictData) map[string]any {
	return map[string]any{
		"id": d.Id, "dictType": d.DictType, "label": d.Label, "value": d.Value,
		"orderNum": d.OrderNum, "status": d.Status, "remark": d.Remark,
		"createdAt": d.CreatedAt, "updatedAt": d.UpdatedAt,
	}
}

// ----- 参数 -----

type ParamSaveReq struct {
	Id        string `json:"id,optional"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive,optional"`
	Category  string `json:"category,optional"`
	Remark    string `json:"remark,optional"`
	Status    int32  `json:"status,optional"`
}

func ParamList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]ParamRecord, len(store.params))
	copy(src, store.params)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, p := range src {
		if !matchKeyword(req.Keyword, p.Key, p.Category, p.Remark) {
			continue
		}
		if !matchStatusInt(req.Status, p.Status) {
			continue
		}
		filtered = append(filtered, paramToMap(p, true))
	}
	sortByCreatedDesc(filtered)
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func ParamSave(req ParamSaveReq) (map[string]any, error) {
	if strings.TrimSpace(req.Key) == "" {
		return nil, errors.New("参数 key 不能为空")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := nowStr()
	if req.Id != "" {
		for i := range store.params {
			if store.params[i].Id == req.Id {
				for j, p := range store.params {
					if j != i && strings.EqualFold(p.Key, req.Key) {
						return nil, errors.New("参数 key 已存在")
					}
				}
				store.params[i].Key = req.Key
				store.params[i].Value = req.Value
				store.params[i].Sensitive = req.Sensitive
				store.params[i].Category = req.Category
				store.params[i].Remark = req.Remark
				store.params[i].Status = req.Status
				store.params[i].UpdatedAt = now
				return paramToMap(store.params[i], true), nil
			}
		}
		return nil, errors.New("参数不存在")
	}
	for _, p := range store.params {
		if strings.EqualFold(p.Key, req.Key) {
			return nil, errors.New("参数 key 已存在")
		}
	}
	rec := ParamRecord{
		Id: newId(), Key: req.Key, Value: req.Value, Sensitive: req.Sensitive,
		Category: req.Category, Remark: req.Remark, Status: req.Status,
		CreatedAt: now, UpdatedAt: now,
	}
	store.params = append(store.params, rec)
	return paramToMap(rec, true), nil
}

func ParamRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.params[:0]
	removed := 0
	for _, p := range store.params {
		if _, ok := idSet[p.Id]; ok {
			removed++
			continue
		}
		out = append(out, p)
	}
	store.params = out
	return removed
}

// ParamRefreshCache 在网关层未注入 Redis 时返回 noop，但保留接口语义。
func ParamRefreshCache() map[string]any {
	store.mu.RLock()
	count := len(store.params)
	store.mu.RUnlock()
	return map[string]any{
		"refreshed":   count,
		"strategy":    "in-memory-snapshot",
		"refreshedAt": nowStr(),
		"note":        "网关层不直连 Redis，仅在运行时记录上做快照刷新",
	}
}

// paramToMap 始终对敏感参数掩码。
func paramToMap(p ParamRecord, mask bool) map[string]any {
	val := p.Value
	if mask && p.Sensitive {
		val = maskValue(val)
	}
	return map[string]any{
		"id": p.Id, "key": p.Key, "value": val, "sensitive": p.Sensitive,
		"category": p.Category, "remark": p.Remark, "status": p.Status,
		"createdAt": p.CreatedAt, "updatedAt": p.UpdatedAt,
	}
}

func maskValue(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + "****" + v[len(v)-2:]
}

// ----- 公告 -----

type NoticeSaveReq struct {
	Id      string `json:"id,optional"`
	Title   string `json:"title"`
	Type    string `json:"type,optional"`
	Content string `json:"content"`
	Status  int32  `json:"status,optional"`
}

func NoticeList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]NoticeRecord, len(store.notices))
	copy(src, store.notices)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, n := range src {
		if !matchKeyword(req.Keyword, n.Title, n.Type) {
			continue
		}
		if req.Type != "" && !strings.EqualFold(req.Type, n.Type) {
			continue
		}
		if !matchStatusInt(req.Status, n.Status) {
			continue
		}
		filtered = append(filtered, noticeToMap(n))
	}
	sortByCreatedDesc(filtered)
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

func NoticeSave(req NoticeSaveReq) (map[string]any, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("公告标题不能为空")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	now := nowStr()
	if req.Id != "" {
		for i := range store.notices {
			if store.notices[i].Id == req.Id {
				store.notices[i].Title = req.Title
				store.notices[i].Type = req.Type
				store.notices[i].Content = req.Content
				store.notices[i].Status = req.Status
				store.notices[i].UpdatedAt = now
				return noticeToMap(store.notices[i]), nil
			}
		}
		return nil, errors.New("公告不存在")
	}
	rec := NoticeRecord{
		Id: newId(), Title: req.Title, Type: req.Type, Content: req.Content,
		Status: req.Status, CreatedAt: now, UpdatedAt: now,
	}
	store.notices = append(store.notices, rec)
	return noticeToMap(rec), nil
}

func NoticeRemove(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	out := store.notices[:0]
	removed := 0
	for _, n := range store.notices {
		if _, ok := idSet[n.Id]; ok {
			removed++
			continue
		}
		out = append(out, n)
	}
	store.notices = out
	return removed
}

func NoticePublish(id string, publish bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i := range store.notices {
		if store.notices[i].Id == id {
			now := nowStr()
			if publish {
				store.notices[i].Status = 1
				store.notices[i].PublishedAt = now
			} else {
				store.notices[i].Status = 2
			}
			store.notices[i].UpdatedAt = now
			return nil
		}
	}
	return errors.New("公告不存在")
}

func noticeToMap(n NoticeRecord) map[string]any {
	return map[string]any{
		"id": n.Id, "title": n.Title, "type": n.Type, "content": n.Content,
		"status": n.Status, "publishedAt": n.PublishedAt,
		"createdAt": n.CreatedAt, "updatedAt": n.UpdatedAt,
	}
}

// ----- 文件管理 -----

func FileList(req AdminPageReq) *types.AdminListResp {
	store.mu.RLock()
	src := make([]FileRecord, len(store.files))
	copy(src, store.files)
	store.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(src))
	for _, f := range src {
		if f.Deleted {
			continue
		}
		if !matchKeyword(req.Keyword, f.Name, f.Category) {
			continue
		}
		filtered = append(filtered, fileToMap(f))
	}
	sortByCreatedDesc(filtered)
	total := len(filtered)
	start, end, size := paginate(total, req.Current, req.Size)
	return toAdminListResp(filtered[start:end], int64(total), req.Current, size)
}

// FileUpload 从 multipart 读取一个文件，校验大小/总量后落入运行时仓库。
func FileUpload(fh *multipart.FileHeader, category string) (map[string]any, error) {
	if fh == nil {
		return nil, errors.New("缺少上传文件")
	}
	if fh.Size <= 0 || fh.Size > maxFileBytes {
		return nil, errors.New("文件大小超出限制（上限 4MB）")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, errors.New("文件打开失败")
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, errors.New("文件读取失败")
	}
	if int64(len(buf)) > maxFileBytes {
		return nil, errors.New("文件大小超出限制（上限 4MB）")
	}

	sum := sha256.Sum256(buf)
	rec := FileRecord{
		Id:        newId(),
		Name:      filepath.Base(fh.Filename),
		Size:      int64(len(buf)),
		MimeType:  detectMime(fh),
		Sha256:    hex.EncodeToString(sum[:]),
		Category:  category,
		CreatedAt: nowStr(),
		bytes:     buf,
	}

	store.mu.Lock()
	if store.totalSize+rec.Size > maxTotalBytes {
		// 触发软淘汰：移除最旧的非软删
		for i := 0; i < len(store.files) && store.totalSize+rec.Size > maxTotalBytes; i++ {
			if !store.files[i].Deleted && store.files[i].bytes != nil {
				store.totalSize -= store.files[i].Size
				store.files[i].bytes = nil
				store.files[i].Deleted = true
			}
		}
		if store.totalSize+rec.Size > maxTotalBytes {
			store.mu.Unlock()
			return nil, errors.New("文件仓库容量已达上限")
		}
	}
	store.files = append(store.files, rec)
	store.totalSize += rec.Size
	store.mu.Unlock()

	return fileToMap(rec), nil
}

// FileRead 返回文件二进制及元数据；若软删除或不存在返回 nil/false。
func FileRead(id string) ([]byte, FileRecord, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, f := range store.files {
		if f.Id == id && !f.Deleted && f.bytes != nil {
			cp := make([]byte, len(f.bytes))
			copy(cp, f.bytes)
			return cp, f, true
		}
	}
	return nil, FileRecord{}, false
}

func FileSoftDelete(ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	idSet := toSet(ids)
	store.mu.Lock()
	defer store.mu.Unlock()
	removed := 0
	for i := range store.files {
		if _, ok := idSet[store.files[i].Id]; ok && !store.files[i].Deleted {
			store.files[i].Deleted = true
			if store.files[i].bytes != nil {
				store.totalSize -= store.files[i].Size
				store.files[i].bytes = nil
			}
			removed++
		}
	}
	return removed
}

func detectMime(fh *multipart.FileHeader) string {
	if mt := fh.Header.Get("Content-Type"); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

func fileToMap(f FileRecord) map[string]any {
	return map[string]any{
		"id":        f.Id,
		"name":      f.Name,
		"size":      f.Size,
		"mimeType":  f.MimeType,
		"sha256":    f.Sha256,
		"category":  f.Category,
		"createdAt": f.CreatedAt,
		"deleted":   f.Deleted,
	}
}
