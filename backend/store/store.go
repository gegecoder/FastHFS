// Package store 提供 SQLite 持久化（modernc.org/sqlite，无 CGO）。
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"FastHFS/backend/appdir"
	"FastHFS/backend/models"
)

// Store 数据库与数据目录
type Store struct {
	db      *sql.DB
	baseDir string
	mu      sync.Mutex
}

// New 打开/创建数据库并完成建表
func New(appName string) (*Store, error) {
	baseDir := filepath.Join(appdir.Base(), appName)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	dbPath := filepath.Join(baseDir, "fasthfs.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// 单连接串行化写入，避免 SQLite 锁竞争；本应用请求量小，足够使用。
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("初始化数据库失败: %w", err)
		}
	}
	s := &Store{db: db, baseDir: baseDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库
func (s *Store) Close() error { return s.db.Close() }

// BaseDir 应用数据目录
func (s *Store) BaseDir() string { return s.baseDir }

// TusDir tus 分片临时目录
func (s *Store) TusDir() string {
	dir := filepath.Join(s.baseDir, "tus")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS mounts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  alias TEXT NOT NULL UNIQUE COLLATE NOCASE,
  root TEXT NOT NULL,
  allow_browse INTEGER NOT NULL DEFAULT 1,
  allow_download INTEGER NOT NULL DEFAULT 1,
  allow_upload INTEGER NOT NULL DEFAULT 0,
  allow_delete INTEGER NOT NULL DEFAULT 0,
  ignore_rules TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_login_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS shares (
  id TEXT PRIMARY KEY,
  token TEXT NOT NULL UNIQUE,
  mount_id TEXT NOT NULL,
  path TEXT NOT NULL,
  password_hash TEXT NOT NULL DEFAULT '',
  expires_at INTEGER NOT NULL DEFAULT 0,
  max_visits INTEGER NOT NULL DEFAULT 0,
  visits INTEGER NOT NULL DEFAULT 0,
  mode TEXT NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS temp_tasks (
  id TEXT PRIMARY KEY,
  mount_id TEXT NOT NULL,
  rel_dir TEXT NOT NULL DEFAULT '',
  filename TEXT NOT NULL,
  size INTEGER NOT NULL DEFAULT 0,
  offset INTEGER NOT NULL DEFAULT 0,
  temp_path TEXT NOT NULL,
  share_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token);
CREATE INDEX IF NOT EXISTS idx_shares_expires ON shares(expires_at);
CREATE INDEX IF NOT EXISTS idx_temp_created ON temp_tasks(created_at);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// 兼容旧库：补齐 share_id 列（已存在时忽略错误）
	_, _ = s.db.Exec(`ALTER TABLE temp_tasks ADD COLUMN share_id TEXT NOT NULL DEFAULT ''`)
	// 兼容旧库：补齐 last_login_at 列
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN last_login_at INTEGER NOT NULL DEFAULT 0`)
	return nil
}

func nowUnix() int64 { return time.Now().Unix() }

// RandomID 生成 16 字节随机十六进制 ID
func RandomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// RandomToken 生成 URL 安全的分享令牌
func RandomToken() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// === 配置 ===

// GetConfig 读取配置，缺失字段使用默认值
func (s *Store) GetConfig() (models.Config, error) {
	cfg := models.DefaultConfig()
	var raw string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = 'app'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return models.DefaultConfig(), nil
	}
	normalizeConfig(&cfg)
	return cfg, nil
}

// SaveConfig 保存配置
func (s *Store) SaveConfig(cfg models.Config) error {
	normalizeConfig(&cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO config(key, value) VALUES('app', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(raw))
	return err
}

func normalizeConfig(c *models.Config) {
	// 保留合法的 IPv4 监听地址（通配、回环或指定网卡地址），非法值回退到仅本机。
	// 网卡地址是否属于本机由 app/fileserver 层在保存与启动时校验。
	if ip := net.ParseIP(strings.TrimSpace(c.ListenAddr)); ip != nil && ip.To4() != nil {
		c.ListenAddr = ip.To4().String()
	} else {
		c.ListenAddr = "127.0.0.1"
	}
	if c.Port < 1 || c.Port > 65535 {
		c.Port = 8080
	}
	switch c.AnonymousMode {
	case models.AnonymousDeny, models.AnonymousReadonly, models.AnonymousUpload:
	default:
		c.AnonymousMode = models.AnonymousReadonly
	}
	if c.Theme != "dark" {
		c.Theme = "light"
	}
	if c.MaxUploadMB < 0 {
		c.MaxUploadMB = 0
	}
	if c.RateLimitPerMin <= 0 {
		c.RateLimitPerMin = 300
	}
	if c.Blacklist == nil {
		c.Blacklist = []string{}
	}
	if c.Language != "en" {
		c.Language = "zh"
	}
	c.BasePath = normalizeBasePath(c.BasePath)
}

func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	return p
}

// EnsureJWTSecret 首次运行时生成并持久化 JWT 密钥
func (s *Store) EnsureJWTSecret() ([]byte, error) {
	var secret string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = 'jwt_secret'`).Scan(&secret)
	if err == nil && secret != "" {
		return []byte(secret), nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	secret = hex.EncodeToString(b)
	_, err = s.db.Exec(`INSERT INTO config(key, value) VALUES('jwt_secret', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, secret)
	if err != nil {
		return nil, err
	}
	return []byte(secret), nil
}

// === 挂载 ===

// ListMounts 全部挂载点（按创建时间）
func (s *Store) ListMounts() ([]models.Mount, error) {
	rows, err := s.db.Query(`SELECT id,name,alias,root,allow_browse,allow_download,allow_upload,allow_delete,ignore_rules,created_at
		FROM mounts ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Mount, 0)
	for rows.Next() {
		m, err := scanMount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMount(r rowScanner) (models.Mount, error) {
	var m models.Mount
	var browse, download, upload, del int
	var rules string
	if err := r.Scan(&m.ID, &m.Name, &m.Alias, &m.Root, &browse, &download, &upload, &del, &rules, &m.CreatedAt); err != nil {
		return m, err
	}
	m.AllowBrowse = browse == 1
	m.AllowDownload = download == 1
	m.AllowUpload = upload == 1
	m.AllowDelete = del == 1
	m.IgnoreRules = []string{}
	_ = json.Unmarshal([]byte(rules), &m.IgnoreRules)
	return m, nil
}

// GetMount 按 ID 查询
func (s *Store) GetMount(id string) (models.Mount, error) {
	row := s.db.QueryRow(`SELECT id,name,alias,root,allow_browse,allow_download,allow_upload,allow_delete,ignore_rules,created_at
		FROM mounts WHERE id = ?`, id)
	return scanMount(row)
}

// GetMountByAlias 按访问别名查询
func (s *Store) GetMountByAlias(alias string) (models.Mount, error) {
	row := s.db.QueryRow(`SELECT id,name,alias,root,allow_browse,allow_download,allow_upload,allow_delete,ignore_rules,created_at
		FROM mounts WHERE alias = ? COLLATE NOCASE`, alias)
	return scanMount(row)
}

// AddMount 新增挂载
func (s *Store) AddMount(m models.Mount) (models.Mount, error) {
	if m.ID == "" {
		m.ID = RandomID()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = nowUnix()
	}
	if m.IgnoreRules == nil {
		m.IgnoreRules = []string{}
	}
	rules, _ := json.Marshal(m.IgnoreRules)
	_, err := s.db.Exec(`INSERT INTO mounts(id,name,alias,root,allow_browse,allow_download,allow_upload,allow_delete,ignore_rules,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.Alias, m.Root, boolInt(m.AllowBrowse), boolInt(m.AllowDownload),
		boolInt(m.AllowUpload), boolInt(m.AllowDelete), string(rules), m.CreatedAt)
	return m, err
}

// UpdateMount 更新挂载
func (s *Store) UpdateMount(m models.Mount) error {
	if m.IgnoreRules == nil {
		m.IgnoreRules = []string{}
	}
	rules, _ := json.Marshal(m.IgnoreRules)
	_, err := s.db.Exec(`UPDATE mounts SET name=?,alias=?,root=?,allow_browse=?,allow_download=?,
		allow_upload=?,allow_delete=?,ignore_rules=? WHERE id=?`,
		m.Name, m.Alias, m.Root, boolInt(m.AllowBrowse), boolInt(m.AllowDownload),
		boolInt(m.AllowUpload), boolInt(m.AllowDelete), string(rules), m.ID)
	return err
}

// DeleteMount 删除挂载（同时删除关联分享）
func (s *Store) DeleteMount(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM shares WHERE mount_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM mounts WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// CountMounts 挂载数量
func (s *Store) CountMounts() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mounts`).Scan(&n)
	return n, err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// === 用户 ===

// ListUsers 全部账号
func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(`SELECT id,username,password_hash,created_at,last_login_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.LastLoginAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUserByUsername 按用户名查询
func (s *Store) GetUserByUsername(username string) (models.User, error) {
	var u models.User
	err := s.db.QueryRow(`SELECT id,username,password_hash,created_at,last_login_at FROM users WHERE username = ? COLLATE NOCASE`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.LastLoginAt)
	return u, err
}

// AddUser 新增账号
func (s *Store) AddUser(username, passwordHash string) (models.User, error) {
	u := models.User{
		ID:           RandomID(),
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    nowUnix(),
	}
	_, err := s.db.Exec(`INSERT INTO users(id,username,password_hash,created_at) VALUES(?,?,?,?)`,
		u.ID, u.Username, u.PasswordHash, u.CreatedAt)
	return u, err
}

// DeleteUser 删除账号
func (s *Store) DeleteUser(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdateUserPassword 更新账号密码哈希
func (s *Store) UpdateUserPassword(id, passwordHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

// TouchUserLogin 记录账号最近登录时间
func (s *Store) TouchUserLogin(id string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, nowUnix(), id)
	return err
}

// CountUsers 账号数量
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// === 分享 ===

// ListShares 分享列表（联表挂载名）
func (s *Store) ListShares() ([]models.Share, error) {
	rows, err := s.db.Query(`SELECT s.id,s.token,s.mount_id,s.path,s.password_hash,s.expires_at,s.max_visits,s.visits,s.mode,s.created_by,s.created_at,
		COALESCE(m.alias,''),COALESCE(m.name,'')
		FROM shares s LEFT JOIN mounts m ON m.id = s.mount_id
		ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Share, 0)
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

func scanShare(r rowScanner) (models.Share, error) {
	var sh models.Share
	var pwd string
	if err := r.Scan(&sh.ID, &sh.Token, &sh.MountID, &sh.Path, &pwd, &sh.ExpiresAt,
		&sh.MaxVisits, &sh.Visits, &sh.Mode, &sh.CreatedBy, &sh.CreatedAt, &sh.MountAlias, &sh.MountName); err != nil {
		return sh, err
	}
	sh.PasswordHash = pwd
	sh.HasPassword = pwd != ""
	return sh, nil
}

// GetShareByToken 按令牌查询
func (s *Store) GetShareByToken(token string) (models.Share, error) {
	row := s.db.QueryRow(`SELECT s.id,s.token,s.mount_id,s.path,s.password_hash,s.expires_at,s.max_visits,s.visits,s.mode,s.created_by,s.created_at,
		COALESCE(m.alias,''),COALESCE(m.name,'')
		FROM shares s LEFT JOIN mounts m ON m.id = s.mount_id WHERE s.token = ?`, token)
	return scanShare(row)
}

// GetShareByID 按 ID 查询
func (s *Store) GetShareByID(id string) (models.Share, error) {
	row := s.db.QueryRow(`SELECT s.id,s.token,s.mount_id,s.path,s.password_hash,s.expires_at,s.max_visits,s.visits,s.mode,s.created_by,s.created_at,
		COALESCE(m.alias,''),COALESCE(m.name,'')
		FROM shares s LEFT JOIN mounts m ON m.id = s.mount_id WHERE s.id = ?`, id)
	return scanShare(row)
}

// CreateShare 新增分享
func (s *Store) CreateShare(sh models.Share) (models.Share, error) {
	if sh.ID == "" {
		sh.ID = RandomID()
	}
	if sh.Token == "" {
		sh.Token = RandomToken()
	}
	if sh.CreatedAt == 0 {
		sh.CreatedAt = nowUnix()
	}
	_, err := s.db.Exec(`INSERT INTO shares(id,token,mount_id,path,password_hash,expires_at,max_visits,visits,mode,created_by,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		sh.ID, sh.Token, sh.MountID, sh.Path, sh.PasswordHash, sh.ExpiresAt, sh.MaxVisits, sh.Visits, sh.Mode, sh.CreatedBy, sh.CreatedAt)
	return sh, err
}

// DeleteShare 删除分享
func (s *Store) DeleteShare(id string) error {
	_, err := s.db.Exec(`DELETE FROM shares WHERE id = ?`, id)
	return err
}

// IncrementShareVisit 访问次数 +1，返回最新次数；达到上限时不再增加。
func (s *Store) IncrementShareVisit(id string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var visits, max int64
	if err := tx.QueryRow(`SELECT visits,max_visits FROM shares WHERE id = ?`, id).Scan(&visits, &max); err != nil {
		return 0, err
	}
	if max > 0 && visits >= max {
		return visits, nil
	}
	visits++
	if _, err := tx.Exec(`UPDATE shares SET visits = ? WHERE id = ?`, visits, id); err != nil {
		return 0, err
	}
	return visits, tx.Commit()
}

// CleanupExpiredShares 删除已过期或访问次数用尽的分享，返回删除数量。
func (s *Store) CleanupExpiredShares() (int64, error) {
	now := nowUnix()
	res, err := s.db.Exec(`DELETE FROM shares WHERE (expires_at > 0 AND expires_at < ?) OR (max_visits > 0 AND visits >= max_visits)`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// === tus 临时任务 ===

// CreateTempTask 记录上传任务
func (s *Store) CreateTempTask(t models.TempTask) error {
	_, err := s.db.Exec(`INSERT INTO temp_tasks(id,mount_id,rel_dir,filename,size,offset,temp_path,share_id,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, t.ID, t.MountID, t.RelDir, t.Filename, t.Size, t.Offset, t.TempPath, t.ShareID, t.CreatedAt)
	return err
}

// GetTempTask 查询上传任务
func (s *Store) GetTempTask(id string) (models.TempTask, error) {
	var t models.TempTask
	err := s.db.QueryRow(`SELECT id,mount_id,rel_dir,filename,size,offset,temp_path,share_id,created_at FROM temp_tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.MountID, &t.RelDir, &t.Filename, &t.Size, &t.Offset, &t.TempPath, &t.ShareID, &t.CreatedAt)
	return t, err
}

// UpdateTempOffset 更新已上传偏移
func (s *Store) UpdateTempOffset(id string, offset int64) error {
	_, err := s.db.Exec(`UPDATE temp_tasks SET offset = ? WHERE id = ?`, offset, id)
	return err
}

// DeleteTempTask 删除上传任务
func (s *Store) DeleteTempTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM temp_tasks WHERE id = ?`, id)
	return err
}

// CleanupStaleTempTasks 清理超过 maxAge 的未完成上传，返回删除数量。
func (s *Store) CleanupStaleTempTasks(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge).Unix()
	rows, err := s.db.Query(`SELECT id,temp_path FROM temp_tasks WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	var ids []string
	var paths []string
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err == nil {
			ids = append(ids, id)
			paths = append(paths, p)
		}
	}
	_ = rows.Close()
	for i, id := range ids {
		_ = os.Remove(paths[i])
		_, _ = s.db.Exec(`DELETE FROM temp_tasks WHERE id = ?`, id)
	}
	return int64(len(ids)), nil
}
