// Package fileserver 实现对外 HTTP 文件服务：认证、浏览、下载、上传、分享与安全校验。
package fileserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"FastHFS/backend/models"
)

var errInvalidPath = errors.New("非法路径")

// resolveMountPath 将挂载点内的相对路径安全解析为绝对路径，并校验路径穿越/软链接逃逸。
// 返回规范化后的相对路径（使用 / 分隔）。
func resolveMountPath(m models.Mount, rel string) (string, string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", "", errInvalidPath
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "/")
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		// 显式拒绝 .. 段，避免被 path.Clean 归一化后掩盖穿越意图
		if seg == ".." {
			return "", "", errInvalidPath
		}
		if strings.Contains(seg, ":") {
			return "", "", errInvalidPath
		}
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if cleaned == "." {
		cleaned = ""
	}
	root, err := filepath.Abs(m.Root)
	if err != nil {
		return "", "", errInvalidPath
	}
	root, err = evalExisting(root)
	if err != nil {
		return "", "", errInvalidPath
	}
	abs := filepath.Join(root, filepath.FromSlash(cleaned))
	if !isWithin(root, abs) {
		return "", "", errInvalidPath
	}
	if err := checkSymlink(root, abs); err != nil {
		return "", "", errInvalidPath
	}
	return abs, cleaned, nil
}

func evalExisting(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

// isWithin 判断 target 是否位于 root 内（含 root 本身）
func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// checkSymlink 校验目标路径（或最近的存在祖先）解析软链接后仍在 root 内。
func checkSymlink(root, target string) error {
	cur := target
	for {
		if _, err := os.Lstat(cur); err == nil {
			resolved, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return err
			}
			if !isWithin(root, resolved) {
				return errInvalidPath
			}
			return nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return errInvalidPath
		}
		cur = parent
	}
}

// sanitizeName 净化单个文件/目录名，去掉路径分隔符与 Windows 非法字符。
func sanitizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimRight(name, ". ")
	if name == "" || name == "." || name == ".." {
		return "", errors.New("文件名不合法")
	}
	if len([]rune(name)) > 200 {
		return "", errors.New("文件名过长")
	}
	upper := strings.ToUpper(name)
	base := strings.ToUpper(strings.TrimSuffix(upper, filepath.Ext(upper)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name, nil
}

// sanitizeRelDir 净化“文件夹上传”带来的相对目录。
func sanitizeRelDir(rel string) (string, error) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", errors.New("目录不合法")
		}
		clean, err := sanitizeName(p)
		if err != nil {
			return "", err
		}
		out = append(out, clean)
	}
	return strings.Join(out, "/"), nil
}

// uniqueTarget 目标已存在时自动追加 (1)/(2)…，返回可用的完整路径。
func uniqueTarget(dir, name string) string {
	target := filepath.Join(dir, name)
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return target
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 10000; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Lstat(cand); os.IsNotExist(err) {
			return cand
		}
	}
	return target
}

// detectMIME 优先按扩展名识别，未知时读取文件头嗅探，避免类型误判。
func detectMIME(abs, name string) string {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
		return t
	}
	f, err := os.Open(abs)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

func isTextMIME(t string) bool {
	t = strings.ToLower(t)
	return strings.HasPrefix(t, "text/") ||
		strings.Contains(t, "json") || strings.Contains(t, "xml") ||
		strings.Contains(t, "javascript") || strings.Contains(t, "yaml") ||
		strings.Contains(t, "x-sh")
}

// ignoreRule 编译后的忽略规则
type ignoreRule struct {
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
}

// parseIgnoreRules 解析 .gitignore 风格规则（支持 *、**、?、目录后缀 /、! 取反）。
func parseIgnoreRules(rules []string) []ignoreRule {
	out := make([]ignoreRule, 0, len(rules))
	for _, raw := range rules {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := false
		if strings.HasPrefix(line, "!") {
			negate = true
			line = strings.TrimSpace(line[1:])
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.Trim(line, "/")
		if line == "" {
			continue
		}
		anchored := strings.Contains(line, "/")
		expr := globToRegexp(line, anchored)
		if re, err := regexp.Compile(expr); err == nil {
			out = append(out, ignoreRule{re: re, negate: negate, dirOnly: dirOnly})
		}
	}
	return out
}

func globToRegexp(pattern string, anchored bool) string {
	var b strings.Builder
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString(`(^|.*/)`)
	}
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}

// ignored 判断相对路径是否被忽略规则命中（后面的规则可覆盖前面的）。
func ignored(rules []string, rel string, isDir bool) bool {
	if len(rules) == 0 {
		return false
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "/")
	parsed := parseIgnoreRules(rules)
	matched := false
	for _, r := range parsed {
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(rel) {
			matched = !r.negate
		}
	}
	return matched
}

// hasBlackSuffix 检查上传后缀黑名单
func hasBlackSuffix(name string, blacklist []string) bool {
	lower := strings.ToLower(name)
	for _, s := range blacklist {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, ".") {
			s = "." + s
		}
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

// contentDisposition 生成兼容中文名的 Content-Disposition。
func contentDisposition(name, disposition string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 128 && r != '"' && r != '\\' {
			return r
		}
		if unicode.IsSpace(r) {
			return '_'
		}
		return '_'
	}, name)
	if ascii == "" {
		ascii = "download"
	}
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, ascii, pathEscape(name))
}

func pathEscape(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&0xF])
	}
	return b.String()
}

// joinRel 拼接虚拟路径，保持 / 分隔且不使用 ..
func joinRel(base, name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.Trim(name, "/")
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "/" + name
}

// parentRel 返回父级虚拟路径
func parentRel(rel string) string {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return ""
	}
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}
