package fileserver

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"FastHFS/backend/applog"
	"FastHFS/backend/models"
)

const shareKey = "fasthfs_share_session"

// shareSession 已解锁的分享上下文
type shareSession struct {
	share models.Share
	mount models.Mount
	root  string // 分享目标绝对路径
}

// shareURL 生成分享访问地址
func (s *Server) shareURL(token string) string {
	return strings.TrimRight(s.PrimaryURL(), "/") + "/s/" + token
}

// shareExpired 分享是否已过期或用尽访问次数
func shareExpired(sh models.Share) bool {
	if sh.ExpiresAt > 0 && time.Now().Unix() > sh.ExpiresAt {
		return true
	}
	return sh.MaxVisits > 0 && sh.Visits >= sh.MaxVisits
}

func (s *Server) shareSessionFrom(c *gin.Context) (shareSession, bool) {
	v, ok := c.Get(shareKey)
	if !ok {
		return shareSession{}, false
	}
	sess, ok := v.(shareSession)
	return sess, ok
}

// handleShareInfo 分享公开信息
func (s *Server) handleShareInfo(c *gin.Context) {
	sh, err := s.store.GetShareByToken(c.Param("token"))
	if err != nil {
		fail(c, http.StatusNotFound, "分享不存在或已撤销", "share_not_found")
		return
	}
	if shareExpired(sh) {
		fail(c, http.StatusGone, "分享已过期或访问次数已用尽", "share_expired")
		return
	}
	applog.Infof("share", c.ClientIP(), "", "打开分享 %s（%s）", sh.Token, sh.MountName)
	c.JSON(http.StatusOK, models.ShareInfo{
		Token:         sh.Token,
		MountName:     sh.MountName,
		Path:          sh.Path,
		Mode:          sh.Mode,
		HasPassword:   sh.HasPassword,
		ExpiresAt:     sh.ExpiresAt,
		MaxVisits:     sh.MaxVisits,
		Visits:        sh.Visits,
		Authenticated: s.shareCookieValid(c, sh),
	})
}

// handleShareUnlock 校验密码并建立分享会话
func (s *Server) handleShareUnlock(c *gin.Context) {
	var req models.ShareUnlockRequest
	_ = c.ShouldBindJSON(&req)

	sh, err := s.store.GetShareByToken(c.Param("token"))
	if err != nil {
		fail(c, http.StatusNotFound, "分享不存在或已撤销", "share_not_found")
		return
	}
	if shareExpired(sh) {
		fail(c, http.StatusGone, "分享已过期或访问次数已用尽", "share_expired")
		return
	}
	if s.shareCookieValid(c, sh) {
		c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "已解锁"})
		return
	}
	if sh.HasPassword && bcrypt.CompareHashAndPassword([]byte(sh.PasswordHash), []byte(req.Password)) != nil {
		applog.Warnf("share", c.ClientIP(), "", "分享 %s 访问密码错误", sh.Token)
		fail(c, http.StatusForbidden, "访问密码错误", "invalid_password")
		return
	}
	visits, err := s.store.IncrementShareVisit(sh.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "分享校验失败", "internal_error")
		return
	}
	if sh.MaxVisits > 0 && visits >= sh.MaxVisits {
		// 本次访问即最后一次，仍允许进入
		_ = visits
	}
	token, err := s.issueShareToken(sh)
	if err != nil {
		fail(c, http.StatusInternalServerError, "分享校验失败", "internal_error")
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     shareCookieName,
		Value:    token,
		Path:     s.apiCookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
		MaxAge:   24 * 3600,
	})
	applog.Infof("share", c.ClientIP(), "", "分享 %s 解锁成功", sh.Token)
	c.JSON(http.StatusOK, models.APIResult{Success: true, Message: "解锁成功"})
}

func (s *Server) issueShareToken(sh models.Share) (string, error) {
	exp := time.Now().Add(24 * time.Hour).Unix()
	if sh.ExpiresAt > 0 && sh.ExpiresAt < exp {
		exp = sh.ExpiresAt
	}
	claims := jwt.MapClaims{
		"kind":  "share",
		"sid":   sh.ID,
		"token": sh.Token,
		"exp":   exp,
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

// shareCookieValid 当前请求的分享会话是否对该分享有效
func (s *Server) shareCookieValid(c *gin.Context, sh models.Share) bool {
	raw, err := c.Cookie(shareCookieName)
	if err != nil || raw == "" {
		return false
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return false
	}
	if kind, _ := claims["kind"].(string); kind != "share" {
		return false
	}
	sid, _ := claims["sid"].(string)
	token, _ := claims["token"].(string)
	return sid == sh.ID && token == sh.Token
}

// shareMiddleware 分享内容接口鉴权
func (s *Server) shareMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sh, err := s.store.GetShareByToken(c.Param("token"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(c, http.StatusNotFound, "分享不存在或已撤销", "share_not_found")
			} else {
				fail(c, http.StatusInternalServerError, "读取分享失败", "internal_error")
			}
			return
		}
		if shareExpired(sh) {
			fail(c, http.StatusGone, "分享已过期或访问次数已用尽", "share_expired")
			return
		}
		if !s.shareCookieValid(c, sh) {
			fail(c, http.StatusForbidden, "请先输入访问密码", "share_locked")
			return
		}
		m, err := s.store.GetMount(sh.MountID)
		if err != nil {
			fail(c, http.StatusGone, "分享关联的目录已被移除", "mount_missing")
			return
		}
		root, _, err := resolveMountPath(m, sh.Path)
		if err != nil {
			fail(c, http.StatusGone, "分享路径不可用", "share_invalid")
			return
		}
		c.Set(shareKey, shareSession{share: sh, mount: m, root: root})
		c.Next()
	}
}

// resolveSharePath 将分享内的相对路径解析为绝对路径，并确保不越出分享根目录。
// 返回的第二个值为相对分享根目录的路径（/ 分隔）。
func (s *Server) resolveSharePath(sess shareSession, rel string) (string, string, error) {
	rel = strings.TrimSpace(rel)
	target := sess.share.Path
	if rel != "" {
		target = joinRel(sess.share.Path, rel)
	}
	abs, _, err := resolveMountPath(sess.mount, target)
	if err != nil {
		return "", "", err
	}
	if !isWithin(sess.root, abs) {
		return "", "", errInvalidPath
	}
	relRoot, err := filepath.Rel(sess.root, abs)
	if err != nil {
		return "", "", errInvalidPath
	}
	if relRoot == "." {
		relRoot = ""
	}
	return abs, filepath.ToSlash(relRoot), nil
}

// mountRelFor 计算用于忽略规则匹配的挂载相对路径
func mountRelFor(sharePath, relRoot string) string {
	if relRoot == "" {
		return sharePath
	}
	return joinRel(sharePath, relRoot)
}

// handleShareList 分享目录浏览
func (s *Server) handleShareList(c *gin.Context) {
	sess, ok := s.shareSessionFrom(c)
	if !ok {
		fail(c, http.StatusForbidden, "分享会话无效", "share_required")
		return
	}
	if sess.share.Mode == models.ShareCollect {
		fail(c, http.StatusForbidden, "该分享仅允许上传文件", "collect_mode")
		return
	}
	abs, relRoot, err := s.resolveSharePath(sess, c.Query("path"))
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		fail(c, http.StatusNotFound, "分享内容不存在", "not_found")
		return
	}
	pathLabel := "/"
	if relRoot != "" {
		pathLabel = "/" + relRoot
	}
	applog.Infof("share", c.ClientIP(), "", "分享 %s 浏览 %s", sess.share.Token, pathLabel)
	if !info.IsDir() {
		if relRoot != "" {
			fail(c, http.StatusBadRequest, "目标不是目录", "not_a_dir")
			return
		}
		name := filepath.Base(abs)
		c.JSON(http.StatusOK, models.ListResult{
			Mount:     sess.mount.Alias,
			MountName: sess.share.MountName,
			Path:      "",
			Parent:    "",
			Entries: []models.Entry{{
				Name:    name,
				IsDir:   false,
				Size:    info.Size(),
				ModTime: info.ModTime().Unix(),
				MIME:    detectMIME(abs, name),
			}},
			CanDownload: sess.share.Mode == models.ShareDownload,
			CanShare:    false,
		})
		return
	}
	entries, err := listEntries(abs, mountRelFor(sess.share.Path, relRoot), sess.mount.IgnoreRules)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取目录失败", "read_failed")
		return
	}
	parent := ""
	if relRoot != "" {
		parent = parentRel(relRoot)
	}
	c.JSON(http.StatusOK, models.ListResult{
		Mount:       sess.mount.Alias,
		MountName:   sess.share.MountName,
		Path:        relRoot,
		Parent:      parent,
		Entries:     entries,
		CanDownload: sess.share.Mode == models.ShareDownload,
		CanShare:    false,
	})
}

// handleShareDownload 分享内容下载/预览
func (s *Server) handleShareDownload(c *gin.Context) {
	sess, ok := s.shareSessionFrom(c)
	if !ok {
		fail(c, http.StatusForbidden, "分享会话无效", "share_required")
		return
	}
	if sess.share.Mode == models.ShareCollect {
		fail(c, http.StatusForbidden, "该分享仅允许上传文件", "collect_mode")
		return
	}
	abs, _, err := s.resolveSharePath(sess, c.Query("path"))
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		fail(c, http.StatusNotFound, "文件不存在", "not_found")
		return
	}
	inline := c.Query("inline") == "1"
	if sess.share.Mode == models.SharePreview && !inline {
		fail(c, http.StatusForbidden, "该分享仅支持在线预览", "preview_only")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		fail(c, http.StatusNotFound, "文件不存在", "not_found")
		return
	}
	defer f.Close()
	action := "下载"
	if inline {
		action = "预览"
	}
	applog.Infof("share", c.ClientIP(), "", "分享 %s %s %s", sess.share.Token, action, filepath.Base(abs))
	s.serveFile(c, f, info, filepath.Base(abs), inline)
}

// handleShareZip 分享内容打包下载（仅下载模式）
func (s *Server) handleShareZip(c *gin.Context) {
	sess, ok := s.shareSessionFrom(c)
	if !ok {
		fail(c, http.StatusForbidden, "分享会话无效", "share_required")
		return
	}
	if sess.share.Mode != models.ShareDownload {
		fail(c, http.StatusForbidden, "该分享不支持打包下载", "forbidden")
		return
	}
	abs, relRoot, err := s.resolveSharePath(sess, c.Query("path"))
	if err != nil {
		fail(c, http.StatusBadRequest, "路径不合法", "invalid_path")
		return
	}
	if _, err := os.Stat(abs); err != nil {
		fail(c, http.StatusNotFound, "分享内容不存在", "not_found")
		return
	}
	name := filepath.Base(abs) + ".zip"
	applog.Infof("share", c.ClientIP(), "", "分享 %s 打包下载 %s", sess.share.Token, name)
	s.streamZip(c, abs, mountRelFor(sess.share.Path, relRoot), sess.mount.IgnoreRules, name)
}
