package fileserver

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"FastHFS/backend/applog"
	"FastHFS/backend/models"
)

// fail 统一错误响应
func fail(c *gin.Context, status int, msg, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg, "code": code})
}

// mountFromQuery 从查询参数 mount（挂载别名）解析挂载点
func (s *Server) mountFromQuery(c *gin.Context) (models.Mount, bool) {
	alias := strings.TrimSpace(c.Query("mount"))
	if alias == "" {
		fail(c, http.StatusBadRequest, "缺少挂载点参数", "bad_request")
		return models.Mount{}, false
	}
	m, err := s.store.GetMountByAlias(alias)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(c, http.StatusNotFound, "挂载点不存在", "mount_not_found")
		} else {
			fail(c, http.StatusInternalServerError, "读取挂载点失败", "internal_error")
		}
		return models.Mount{}, false
	}
	return m, true
}

// handleMounts 返回当前身份可见的挂载点
func (s *Server) handleMounts(c *gin.Context) {
	a := s.access(c)
	cfg := s.cfg()
	all, err := s.store.ListMounts()
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取挂载点失败", "internal_error")
		return
	}
	out := models.MountsResult{
		Anonymous:    cfg.AnonymousMode,
		AuthRequired: !a.Authenticated && cfg.AnonymousMode == models.AnonymousDeny,
		Mounts:       make([]models.MountOption, 0, len(all)),
	}
	for _, m := range all {
		if !s.canBrowse(a, m) {
			continue
		}
		out.Mounts = append(out.Mounts, models.MountOption{
			Alias:         m.Alias,
			Name:          m.Name,
			AllowBrowse:   true,
			AllowDownload: s.canDownload(a, m),
			AllowUpload:   s.canUpload(a, m),
		})
	}
	c.JSON(http.StatusOK, out)
}

// handleList 目录列表
func (s *Server) handleList(c *gin.Context) {
	m, ok := s.mountFromQuery(c)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canBrowse(a, m) {
		fail(c, http.StatusForbidden, "没有该目录的浏览权限", "forbidden")
		return
	}
	abs, clean, err := resolveMountPath(m, c.Query("path"))
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		fail(c, http.StatusNotFound, "目录不存在", "not_found")
		return
	}
	if !info.IsDir() {
		fail(c, http.StatusBadRequest, "目标不是目录", "not_a_dir")
		return
	}
	entries, err := listEntries(abs, clean, m.IgnoreRules)
	if err != nil {
		applog.Warnf("list", c.ClientIP(), a.Username, "读取目录失败 %s: %v", abs, err)
		fail(c, http.StatusInternalServerError, "读取目录失败", "read_failed")
		return
	}
	applog.Infof("list", c.ClientIP(), a.Username, "浏览 %s:%s", m.Alias, clean)
	c.JSON(http.StatusOK, models.ListResult{
		Mount:       m.Alias,
		MountName:   m.Name,
		Path:        clean,
		Parent:      parentRel(clean),
		Entries:     entries,
		CanUpload:   s.canUpload(a, m) || s.canDelete(a, m),
		CanDelete:   s.canDelete(a, m),
		CanDownload: s.canDownload(a, m),
		CanShare:    a.Authenticated && m.AllowBrowse,
	})
}

// listEntries 读取目录并应用忽略规则
func listEntries(absDir, relDir string, rules []string) ([]models.Entry, error) {
	items, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	out := make([]models.Entry, 0, len(items))
	for _, it := range items {
		name := it.Name()
		info, err := it.Info()
		if err != nil {
			continue
		}
		isDir := info.IsDir()
		rel := joinRel(relDir, name)
		if ignored(rules, rel, isDir) {
			continue
		}
		e := models.Entry{Name: name, IsDir: isDir, ModTime: info.ModTime().Unix()}
		if !isDir {
			e.Size = info.Size()
			if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
				e.MIME = t
			} else {
				e.MIME = "application/octet-stream"
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// handleDownload 单文件下载（支持 Range/ETag/Last-Modified）
func (s *Server) handleDownload(c *gin.Context) {
	m, ok := s.mountFromQuery(c)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canDownload(a, m) {
		fail(c, http.StatusForbidden, "没有下载权限", "forbidden")
		return
	}
	rel := strings.TrimSpace(c.Query("path"))
	if rel == "" {
		fail(c, http.StatusBadRequest, "请指定文件路径", "bad_request")
		return
	}
	abs, _, err := resolveMountPath(m, rel)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		fail(c, http.StatusNotFound, "文件不存在", "not_found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取文件失败", "read_failed")
		return
	}
	if info.IsDir() {
		fail(c, http.StatusBadRequest, "目录请使用打包下载", "is_dir")
		return
	}
	action := "下载"
	if c.Query("inline") == "1" {
		action = "预览"
	}
	applog.Infof("file", c.ClientIP(), a.Username, "%s %s:%s", action, m.Alias, rel)
	s.serveFile(c, f, info, filepath.Base(abs), c.Query("inline") == "1")
}

// serveFile 输出文件内容，支持断点续传与缓存校验
func (s *Server) serveFile(c *gin.Context, f *os.File, info os.FileInfo, name string, inline bool) {
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	c.Header("Content-Disposition", contentDisposition(name, disposition))
	c.Header("Content-Type", detectMIME(f.Name(), name))
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	c.Header("ETag", fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size()))
	tw := &throttleWriter{ResponseWriter: c.Writer, bw: newBandwidth(s.cfg().DownloadLimitKB)}
	http.ServeContent(tw, c.Request, name, info.ModTime(), f)
}

// handleZip 目录/文件流式打包下载
func (s *Server) handleZip(c *gin.Context) {
	m, ok := s.mountFromQuery(c)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canDownload(a, m) {
		fail(c, http.StatusForbidden, "没有下载权限", "forbidden")
		return
	}
	rel := strings.TrimSpace(c.Query("path"))
	abs, clean, err := resolveMountPath(m, rel)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		fail(c, http.StatusNotFound, "目标不存在", "not_found")
		return
	}

	name := filepath.Base(abs)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = m.Alias
	}
	applog.Infof("zip", c.ClientIP(), a.Username, "打包下载 %s:%s", m.Alias, clean)
	s.streamZip(c, abs, clean, m.IgnoreRules, name+".zip")
}

// streamZip 以流式方式输出 zip（不生成临时大包文件）
func (s *Server) streamZip(c *gin.Context, abs, clean string, rules []string, downloadName string) {
	info, err := os.Stat(abs)
	if err != nil {
		fail(c, http.StatusNotFound, "目标不存在", "not_found")
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", contentDisposition(downloadName, "attachment"))
	c.Header("Cache-Control", "no-store")
	tw := &throttleWriter{ResponseWriter: c.Writer, bw: newBandwidth(s.cfg().DownloadLimitKB)}
	zw := zip.NewWriter(tw)
	defer zw.Close()

	if err := addToZip(zw, abs, clean, info.IsDir(), rules); err != nil {
		applog.Warnf("zip", c.ClientIP(), s.access(c).Username, "打包 %s 失败: %v", abs, err)
	}
}

// addToZip 将目标（文件或目录）写入 zip，目录名作为压缩包内顶层目录
func addToZip(zw *zip.Writer, abs, clean string, isDir bool, rules []string) error {
	if !isDir {
		return addZipFile(zw, abs, filepath.Base(abs))
	}
	prefix := filepath.Base(abs)
	if prefix == "." || prefix == string(filepath.Separator) || prefix == "" {
		prefix = ""
	}
	return filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if p == abs {
			return nil
		}
		relFromBase, err := filepath.Rel(abs, p)
		if err != nil {
			return nil
		}
		relFromBase = filepath.ToSlash(relFromBase)
		entryName := relFromBase
		if prefix != "" {
			entryName = prefix + "/" + relFromBase
		}
		mountRel := joinRel(clean, relFromBase)
		if ignored(rules, mountRel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			hdr := &zip.FileHeader{Name: entryName + "/", Method: zip.Store, Modified: info.ModTime()}
			_, err = zw.CreateHeader(hdr)
			return err
		}
		return addZipFile(zw, p, entryName)
	})
}

func addZipFile(zw *zip.Writer, abs, entryName string) error {
	f, err := os.Open(abs)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return nil
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil
	}
	hdr.Name = entryName
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

type pathRequest struct {
	Mount   string   `json:"mount"`
	Path    string   `json:"path"`
	Name    string   `json:"name"`
	Paths   []string `json:"paths"`
	NewName string   `json:"newName"`
}

// ResolveMountPath 供管理端校验挂载内路径（返回绝对路径与规范化相对路径）
func ResolveMountPath(m models.Mount, rel string) (string, string, error) {
	return resolveMountPath(m, rel)
}

// ListMountEntries 列出挂载目录内容（管理端分享选择器使用）
func ListMountEntries(m models.Mount, rel string) ([]models.Entry, error) {
	abs, clean, err := resolveMountPath(m, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("目标不是目录")
	}
	return listEntries(abs, clean, m.IgnoreRules)
}

// handleMkdir 新建文件夹
func (s *Server) handleMkdir(c *gin.Context) {
	var req pathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误", "bad_request")
		return
	}
	m, ok := s.mountByAlias(c, req.Mount)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canUpload(a, m) && !s.canDelete(a, m) {
		fail(c, http.StatusForbidden, "没有新建文件夹权限", "forbidden")
		return
	}
	abs, _, err := resolveMountPath(m, req.Path)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		fail(c, http.StatusNotFound, "父目录不存在", "not_found")
		return
	}
	name, err := sanitizeName(req.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error(), "invalid_name")
		return
	}
	target := filepath.Join(abs, name)
	if _, err := os.Lstat(target); err == nil {
		fail(c, http.StatusConflict, "同名文件夹已存在", "exists")
		return
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "创建文件夹失败", "mkdir_failed")
		return
	}
	applog.Infof("file", c.ClientIP(), a.Username, "新建文件夹 %s:%s", m.Alias, joinRel(req.Path, name))
	c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "新建成功"})
}

// handleRename 重命名文件/文件夹
func (s *Server) handleRename(c *gin.Context) {
	var req pathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误", "bad_request")
		return
	}
	m, ok := s.mountByAlias(c, req.Mount)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canDelete(a, m) {
		fail(c, http.StatusForbidden, "没有重命名权限", "forbidden")
		return
	}
	rel := strings.TrimSpace(req.Path)
	if rel == "" {
		fail(c, http.StatusBadRequest, "不能重命名挂载根目录", "bad_request")
		return
	}
	abs, _, err := resolveMountPath(m, rel)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	if _, err := os.Lstat(abs); err != nil {
		fail(c, http.StatusNotFound, "目标不存在", "not_found")
		return
	}
	newName := req.NewName
	if strings.TrimSpace(newName) == "" {
		newName = req.Name
	}
	newName, err = sanitizeName(newName)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error(), "invalid_name")
		return
	}
	target := filepath.Join(filepath.Dir(abs), newName)
	if target == abs {
		c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "名称未变更"})
		return
	}
	if _, err := os.Lstat(target); err == nil {
		fail(c, http.StatusConflict, "同名文件已存在", "exists")
		return
	}
	if err := os.Rename(abs, target); err != nil {
		fail(c, http.StatusInternalServerError, "重命名失败", "rename_failed")
		return
	}
	applog.Infof("file", c.ClientIP(), a.Username, "重命名 %s:%s → %s", m.Alias, rel, newName)
	c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "重命名成功"})
}

// handleDelete 删除文件/文件夹（可批量）
func (s *Server) handleDelete(c *gin.Context) {
	var req pathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误", "bad_request")
		return
	}
	m, ok := s.mountByAlias(c, req.Mount)
	if !ok {
		return
	}
	a := s.access(c)
	if !s.canDelete(a, m) {
		fail(c, http.StatusForbidden, "没有删除权限", "forbidden")
		return
	}
	targets := req.Paths
	if len(targets) == 0 && req.Path != "" {
		targets = []string{req.Path}
	}
	if len(targets) == 0 {
		fail(c, http.StatusBadRequest, "请指定要删除的路径", "bad_request")
		return
	}
	for _, rel := range targets {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			fail(c, http.StatusBadRequest, "不能删除挂载根目录", "bad_request")
			return
		}
		abs, _, err := resolveMountPath(m, rel)
		if err != nil {
			fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
			return
		}
		if _, err := os.Lstat(abs); err != nil {
			fail(c, http.StatusNotFound, "目标不存在："+rel, "not_found")
			return
		}
		if err := os.RemoveAll(abs); err != nil {
			fail(c, http.StatusInternalServerError, "删除失败："+rel, "delete_failed")
			return
		}
		applog.Infof("file", c.ClientIP(), a.Username, "删除 %s:%s", m.Alias, rel)
	}
	c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "删除成功"})
}

// mountByAlias 按别名解析挂载点（JSON 请求体场景）
func (s *Server) mountByAlias(c *gin.Context, alias string) (models.Mount, bool) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		fail(c, http.StatusBadRequest, "缺少挂载点参数", "bad_request")
		return models.Mount{}, false
	}
	m, err := s.store.GetMountByAlias(alias)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(c, http.StatusNotFound, "挂载点不存在", "mount_not_found")
		} else {
			fail(c, http.StatusInternalServerError, "读取挂载点失败", "internal_error")
		}
		return models.Mount{}, false
	}
	return m, true
}

// handleCreateShare 创建分享链接（仅登录用户）
func (s *Server) handleCreateShare(c *gin.Context) {
	var req models.ShareCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误", "bad_request")
		return
	}
	m, err := s.store.GetMount(strings.TrimSpace(req.MountID))
	if err != nil && strings.TrimSpace(req.Mount) != "" {
		m, err = s.store.GetMountByAlias(strings.TrimSpace(req.Mount))
	}
	if err != nil {
		fail(c, http.StatusNotFound, "挂载点不存在", "mount_not_found")
		return
	}
	abs, clean, err := resolveMountPath(m, req.Path)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	if _, err := os.Lstat(abs); err != nil {
		fail(c, http.StatusNotFound, "目标不存在", "not_found")
		return
	}

	mode := strings.TrimSpace(req.Mode)
	switch mode {
	case models.SharePreview, models.ShareDownload, models.ShareCollect:
	case "":
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			mode = models.ShareDownload
		} else {
			mode = models.SharePreview
		}
	default:
		fail(c, http.StatusBadRequest, "分享模式不合法", "invalid_mode")
		return
	}

	pwdHash := ""
	if p := strings.TrimSpace(req.Password); p != "" {
		if len(p) < 1 || len(p) > 64 {
			fail(c, http.StatusBadRequest, "分享密码长度需为 1-64 位", "invalid_password")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
		if err != nil {
			fail(c, http.StatusInternalServerError, "生成分享失败", "internal_error")
			return
		}
		pwdHash = string(hash)
	}

	var expiresAt int64
	if req.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour).Unix()
	}
	maxVisits := req.MaxVisits
	if maxVisits < 0 {
		maxVisits = 0
	}

	sh, err := s.store.CreateShare(models.Share{
		MountID:      m.ID,
		MountAlias:   m.Alias,
		MountName:    m.Name,
		Path:         clean,
		PasswordHash: pwdHash,
		HasPassword:  pwdHash != "",
		ExpiresAt:    expiresAt,
		MaxVisits:    maxVisits,
		Mode:         mode,
		CreatedBy:    s.access(c).Username,
	})
	if err != nil {
		applog.Errorf("share", c.ClientIP(), s.access(c).Username, "创建分享失败: %v", err)
		fail(c, http.StatusInternalServerError, "创建分享失败", "internal_error")
		return
	}
	sh.MountAlias = m.Alias
	sh.MountName = m.Name
	applog.Infof("share", c.ClientIP(), s.access(c).Username, "创建分享 → %s", s.shareURL(sh.Token))
	c.JSON(http.StatusOK, models.ShareCreated{Share: sh, URL: s.shareURL(sh.Token)})
}
