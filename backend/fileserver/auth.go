package fileserver

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"FastHFS/backend/applog"
	"FastHFS/backend/models"
)

const (
	authCookieName  = "fasthfs_auth"
	shareCookieName = "fasthfs_share"
	accessKey       = "fasthfs_access"
)

// Access 当前请求的身份（匿名或登录用户）
type Access struct {
	Authenticated bool
	UserID        string
	Username      string
}

func (s *Server) authCookiePath() string {
	base := s.cfg().BasePath
	if base == "" {
		return "/"
	}
	return base + "/"
}

func (s *Server) apiCookiePath() string {
	return s.cfg().BasePath + "/api"
}

// issueUserToken 生成登录 JWT
func (s *Server) issueUserToken(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"name": user.Username,
		"kind": "user",
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Server) parseUserToken(token string) (models.User, bool) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return models.User{}, false
	}
	if kind, _ := claims["kind"].(string); kind != "user" {
		return models.User{}, false
	}
	id, _ := claims["sub"].(string)
	if id == "" {
		return models.User{}, false
	}
	u, err := s.store.GetUserByUsername(firstString(claims["name"]))
	if err == nil && u.ID == id {
		return u, true
	}
	// 用户名可能被修改，按全部账号兜底匹配 ID
	users, err := s.store.ListUsers()
	if err != nil {
		return models.User{}, false
	}
	for _, item := range users {
		if item.ID == id {
			return item, true
		}
	}
	return models.User{}, false
}

func firstString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// userFromRequest 从 Basic / Bearer / Cookie 解析登录用户
func (s *Server) userFromRequest(c *gin.Context) (models.User, bool) {
	if user, pass, ok := c.Request.BasicAuth(); ok {
		u, err := s.store.GetUserByUsername(user)
		if err == nil && bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pass)) == nil {
			return u, true
		}
		return models.User{}, false
	}
	if h := c.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		if u, ok := s.parseUserToken(strings.TrimSpace(h[7:])); ok {
			return u, true
		}
	}
	if cookie, err := c.Cookie(authCookieName); err == nil && cookie != "" {
		if u, ok := s.parseUserToken(cookie); ok {
			return u, true
		}
	}
	return models.User{}, false
}

// authMiddleware 解析身份但不拦截（由各接口按权限判定）
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var a Access
		if u, ok := s.userFromRequest(c); ok {
			a = Access{Authenticated: true, UserID: u.ID, Username: u.Username}
		}
		c.Set(accessKey, a)
		c.Next()
	}
}

func (s *Server) access(c *gin.Context) Access {
	if v, ok := c.Get(accessKey); ok {
		if a, ok := v.(Access); ok {
			return a
		}
	}
	return Access{}
}

// requireUser 需要登录用户
func (s *Server) requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.access(c).Authenticated {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要登录后操作", "code": "login_required"})
			return
		}
		c.Next()
	}
}

// === 权限判定 ===

func (s *Server) canBrowse(a Access, m models.Mount) bool {
	if !m.AllowBrowse {
		return false
	}
	if a.Authenticated {
		return true
	}
	return s.cfg().AnonymousMode != models.AnonymousDeny
}

func (s *Server) canDownload(a Access, m models.Mount) bool {
	if !m.AllowDownload {
		return false
	}
	if a.Authenticated {
		return true
	}
	return s.cfg().AnonymousMode != models.AnonymousDeny
}

func (s *Server) canUpload(a Access, m models.Mount) bool {
	if !m.AllowUpload {
		return false
	}
	if a.Authenticated {
		return true
	}
	return s.cfg().AnonymousMode == models.AnonymousUpload
}

func (s *Server) canDelete(a Access, m models.Mount) bool {
	return a.Authenticated && m.AllowDelete
}

// === 登录接口 ===

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	u, err := s.store.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			applog.Warnf("auth", c.ClientIP(), logName(req.Username), "登录失败：账号不存在")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误", "code": "invalid_credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		applog.Warnf("auth", c.ClientIP(), u.Username, "登录失败：密码错误")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误", "code": "invalid_credentials"})
		return
	}
	token, err := s.issueUserToken(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
		return
	}
	_ = s.store.TouchUserLogin(u.ID)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     s.authCookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
		MaxAge:   7 * 24 * 3600,
	})
	applog.Infof("auth", c.ClientIP(), u.Username, "登录成功")
	c.JSON(http.StatusOK, models.AuthInfo{
		Authenticated: true,
		Username:      u.Username,
		AnonymousMode: s.cfg().AnonymousMode,
		CanLogin:      true,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	a := s.access(c)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     s.authCookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	if a.Authenticated {
		applog.Infof("auth", c.ClientIP(), a.Username, "退出登录")
	}
	c.JSON(http.StatusOK, models.AuthInfo{AnonymousMode: s.cfg().AnonymousMode, CanLogin: true})
}

// logName 空值兜底
func logName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "（空）"
	}
	return name
}

func (s *Server) handleMe(c *gin.Context) {
	a := s.access(c)
	c.JSON(http.StatusOK, models.AuthInfo{
		Authenticated: a.Authenticated,
		Username:      a.Username,
		AnonymousMode: s.cfg().AnonymousMode,
		CanLogin:      true,
	})
}
