package www

import "embed"

// Dist 对外访问站点构建产物（由 webshare/ 前端构建输出）。
//
//go:embed all:dist
var Dist embed.FS
