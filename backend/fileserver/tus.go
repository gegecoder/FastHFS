package fileserver

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"FastHFS/backend/applog"
	"FastHFS/backend/models"
	"FastHFS/backend/store"
)

const tusVersion = "1.0.0"

// tusHeaders 写入 tus 协议公共响应头
func (s *Server) tusHeaders(c *gin.Context) {
	c.Header("Tus-Resumable", tusVersion)
	c.Header("Tus-Version", tusVersion)
	c.Header("Tus-Extension", "creation,termination")
	if maxBytes := s.maxUploadBytes(); maxBytes > 0 {
		c.Header("Tus-Max-Size", strconv.FormatInt(maxBytes, 10))
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Access-Control-Expose-Headers", "Upload-Offset,Location,Upload-Length,Tus-Resumable,Tus-Version,Tus-Max-Size,Tus-Extension")
}

func (s *Server) maxUploadBytes() int64 {
	mb := s.cfg().MaxUploadMB
	if mb <= 0 {
		return 0
	}
	return mb * 1024 * 1024
}

// handleTusOptions 返回协议能力
func (s *Server) handleTusOptions(c *gin.Context) {
	s.tusHeaders(c)
	c.Status(http.StatusNoContent)
}

// handleTusCreate 创建普通上传任务
func (s *Server) handleTusCreate(c *gin.Context) {
	s.tusHeaders(c)
	m, ok := s.mountByAlias(c, c.Param("mount"))
	if !ok {
		return
	}
	if !s.canUpload(s.access(c), m) {
		fail(c, http.StatusForbidden, "没有上传权限", "forbidden")
		return
	}
	s.createTusUpload(c, m, "", "", "")
}

// handleShareTusCreate 创建分享收集上传任务
func (s *Server) handleShareTusCreate(c *gin.Context) {
	s.tusHeaders(c)
	sess, ok := s.shareSessionFrom(c)
	if !ok {
		fail(c, http.StatusForbidden, "分享会话无效", "share_required")
		return
	}
	if sess.share.Mode != models.ShareCollect {
		fail(c, http.StatusForbidden, "该分享不允许上传", "forbidden")
		return
	}
	s.createTusUpload(c, sess.mount, sess.share.ID, sess.share.Path, sess.root)
}

// createTusUpload 创建分片上传任务；within 非空时限制上传目录必须位于该目录内。
func (s *Server) createTusUpload(c *gin.Context, m models.Mount, shareID, baseRel, within string) {
	lengthStr := strings.TrimSpace(c.GetHeader("Upload-Length"))
	if lengthStr == "" {
		fail(c, http.StatusBadRequest, "缺少 Upload-Length", "bad_request")
		return
	}
	size, err := strconv.ParseInt(lengthStr, 10, 64)
	if err != nil || size < 0 {
		fail(c, http.StatusBadRequest, "Upload-Length 不合法", "bad_request")
		return
	}
	if maxBytes := s.maxUploadBytes(); maxBytes > 0 && size > maxBytes {
		fail(c, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("单文件大小超过限制（%d MB）", s.cfg().MaxUploadMB), "too_large")
		return
	}

	meta := parseTusMetadata(c.GetHeader("Upload-Metadata"))
	rawName := strings.TrimSpace(meta["filename"])
	if rawName == "" {
		rawName = strings.TrimSpace(c.Query("filename"))
	}
	name, err := sanitizeName(rawName)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error(), "invalid_name")
		return
	}
	if hasBlackSuffix(name, s.cfg().Blacklist) {
		fail(c, http.StatusBadRequest, "该类型文件禁止上传", "forbidden_type")
		return
	}

	rawDir := meta["dir"]
	if rawDir == "" {
		rawDir = c.Query("dir")
	}
	dir, err := sanitizeRelDir(rawDir)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error(), "invalid_dir")
		return
	}

	dirAbs, dirRel, err := resolveMountPath(m, joinRel(baseRel, dir))
	if err != nil {
		fail(c, http.StatusBadRequest, "上传路径不合法", "invalid_path")
		return
	}
	if within != "" && !isWithin(within, dirAbs) {
		fail(c, http.StatusForbidden, "上传路径超出分享范围", "forbidden")
		return
	}
	if info, statErr := os.Stat(dirAbs); statErr != nil || !info.IsDir() {
		if err := os.MkdirAll(dirAbs, 0o755); err != nil {
			fail(c, http.StatusInternalServerError, "创建上传目录失败", "mkdir_failed")
			return
		}
	}

	id := store.RandomID()
	tempPath := filepath.Join(s.store.TusDir(), id+".part")
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		fail(c, http.StatusInternalServerError, "创建临时文件失败", "internal_error")
		return
	}
	_ = f.Close()

	task := models.TempTask{
		ID:        id,
		MountID:   m.ID,
		RelDir:    dirRel,
		Filename:  name,
		Size:      size,
		Offset:    0,
		TempPath:  tempPath,
		ShareID:   shareID,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.store.CreateTempTask(task); err != nil {
		_ = os.Remove(tempPath)
		applog.Errorf("upload", c.ClientIP(), s.access(c).Username, "创建上传任务失败: %v", err)
		fail(c, http.StatusInternalServerError, "创建上传任务失败", "internal_error")
		return
	}
	if shareID != "" {
		applog.Infof("share", c.ClientIP(), "", "开始接收上传 %s（%d 字节）", name, size)
	} else {
		applog.Infof("upload", c.ClientIP(), s.access(c).Username, "开始上传 %s（%d 字节）", name, size)
	}

	c.Header("Location", s.tusLocation(c, id))
	c.Status(http.StatusCreated)
}

// tusLocation 生成后续分片请求地址
func (s *Server) tusLocation(c *gin.Context, id string) string {
	base := s.cfg().BasePath
	if token := c.Param("token"); token != "" {
		return base + "/api/s/" + token + "/tus/uploads/" + id
	}
	return base + "/api/tus/uploads/" + id
}

func parseTusMetadata(raw string) map[string]string {
	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, " ", 2)
		key := strings.TrimSpace(parts[0])
		val := ""
		if len(parts) == 2 {
			if b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1])); err == nil {
				val = string(b)
			}
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
}

// authorizedTusTask 校验上传任务归属与权限
func (s *Server) authorizedTusTask(c *gin.Context) (models.TempTask, bool) {
	task, err := s.store.GetTempTask(c.Param("id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(c, http.StatusNotFound, "上传任务不存在或已完成", "not_found")
		} else {
			fail(c, http.StatusInternalServerError, "读取上传任务失败", "internal_error")
		}
		return task, false
	}
	m, err := s.store.GetMount(task.MountID)
	if err != nil {
		fail(c, http.StatusGone, "上传目标的挂载点已被移除", "mount_missing")
		return task, false
	}
	if task.ShareID != "" {
		sess, ok := s.shareSessionFrom(c)
		if !ok {
			fail(c, http.StatusForbidden, "分享会话无效", "share_required")
			return task, false
		}
		if sess.share.ID != task.ShareID || sess.share.Mode != models.ShareCollect {
			fail(c, http.StatusForbidden, "分享不允许上传", "forbidden")
			return task, false
		}
	} else {
		if c.Param("token") != "" {
			fail(c, http.StatusForbidden, "分享会话与上传任务不匹配", "forbidden")
			return task, false
		}
		if !s.canUpload(s.access(c), m) {
			fail(c, http.StatusForbidden, "没有上传权限", "forbidden")
			return task, false
		}
	}
	if _, err := os.Stat(task.TempPath); err != nil {
		_ = s.store.DeleteTempTask(task.ID)
		fail(c, http.StatusNotFound, "上传任务已失效，请重新上传", "not_found")
		return task, false
	}
	return task, true
}

// handleTusHead 查询上传进度（断点续传）
func (s *Server) handleTusHead(c *gin.Context) {
	s.tusHeaders(c)
	task, ok := s.authorizedTusTask(c)
	if !ok {
		return
	}
	offset := task.Offset
	if info, err := os.Stat(task.TempPath); err == nil && info.Size() != offset {
		offset = info.Size()
		_ = s.store.UpdateTempOffset(task.ID, offset)
	}
	c.Header("Upload-Offset", strconv.FormatInt(offset, 10))
	c.Header("Upload-Length", strconv.FormatInt(task.Size, 10))
	c.Status(http.StatusOK)
}

// handleTusPatch 接收分片
func (s *Server) handleTusPatch(c *gin.Context) {
	s.tusHeaders(c)
	if strings.TrimSpace(c.GetHeader("Tus-Resumable")) != tusVersion {
		fail(c, http.StatusPreconditionFailed, "Tus-Resumable 版本不支持", "precondition_failed")
		return
	}
	task, ok := s.authorizedTusTask(c)
	if !ok {
		return
	}
	if ct := strings.ToLower(c.GetHeader("Content-Type")); !strings.HasPrefix(ct, "application/offset+octet-stream") {
		fail(c, http.StatusUnsupportedMediaType, "Content-Type 必须为 application/offset+octet-stream", "invalid_content_type")
		return
	}
	offset, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader("Upload-Offset")), 10, 64)
	if err != nil || offset < 0 {
		fail(c, http.StatusBadRequest, "Upload-Offset 不合法", "bad_request")
		return
	}
	if offset != task.Offset {
		c.Header("Upload-Offset", strconv.FormatInt(task.Offset, 10))
		fail(c, http.StatusConflict, "上传偏移不匹配", "offset_mismatch")
		return
	}
	remaining := task.Size - task.Offset
	if remaining <= 0 {
		c.Header("Upload-Offset", strconv.FormatInt(task.Size, 10))
		c.Status(http.StatusNoContent)
		return
	}

	f, err := os.OpenFile(task.TempPath, os.O_WRONLY, 0o600)
	if err != nil {
		fail(c, http.StatusGone, "临时文件不可写，请重新上传", "not_found")
		return
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		fail(c, http.StatusInternalServerError, "写入失败", "write_failed")
		return
	}

	var body io.Reader = c.Request.Body
	if limit := s.cfg().UploadLimitKB; limit > 0 {
		body = &throttleReader{r: c.Request.Body, bw: newBandwidth(limit)}
	}
	written, err := io.Copy(f, io.LimitReader(body, remaining))
	if err != nil {
		fail(c, http.StatusInternalServerError, "写入上传分片失败", "write_failed")
		return
	}
	newOffset := offset + written
	if newOffset < task.Size {
		if err := s.store.UpdateTempOffset(task.ID, newOffset); err != nil {
			fail(c, http.StatusInternalServerError, "更新上传进度失败", "internal_error")
			return
		}
		c.Header("Upload-Offset", strconv.FormatInt(newOffset, 10))
		c.Status(http.StatusNoContent)
		return
	}

	// 先关闭句柄再移动：Windows 不允许重命名仍被打开的文件
	if err := f.Close(); err != nil {
		fail(c, http.StatusInternalServerError, "写入上传分片失败", "write_failed")
		return
	}
	// 接收完成，移动到目标位置
	if err := s.finishTusUpload(task, c.ClientIP(), s.access(c).Username); err != nil {
		applog.Errorf("upload", c.ClientIP(), s.access(c).Username, "保存上传文件失败: %v", err)
		_ = s.store.DeleteTempTask(task.ID)
		_ = os.Remove(task.TempPath)
		fail(c, http.StatusInternalServerError, "保存文件失败", "save_failed")
		return
	}
	c.Header("Upload-Offset", strconv.FormatInt(task.Size, 10))
	c.Status(http.StatusNoContent)
}

// finishTusUpload 将临时文件原子移动到挂载目录（跨盘时回退为复制）
func (s *Server) finishTusUpload(task models.TempTask, clientIP, user string) error {
	m, err := s.store.GetMount(task.MountID)
	if err != nil {
		return err
	}
	dirAbs, _, err := resolveMountPath(m, task.RelDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirAbs, 0o755); err != nil {
		return err
	}
	target := uniqueTarget(dirAbs, task.Filename)
	if err := moveFile(task.TempPath, target); err != nil {
		return err
	}
	if err := s.store.DeleteTempTask(task.ID); err != nil {
		return err
	}
	if task.ShareID != "" {
		applog.Infof("share", clientIP, user, "分享上传完成 %s", target)
	} else {
		applog.Infof("upload", clientIP, user, "接收完成 %s", target)
	}
	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Remove(src)
}

// handleTusDelete 终止上传并清理临时文件
func (s *Server) handleTusDelete(c *gin.Context) {
	s.tusHeaders(c)
	task, ok := s.authorizedTusTask(c)
	if !ok {
		return
	}
	_ = os.Remove(task.TempPath)
	_ = s.store.DeleteTempTask(task.ID)
	applog.Infof("upload", c.ClientIP(), s.access(c).Username, "取消上传 %s", task.Filename)
	c.Status(http.StatusNoContent)
}
