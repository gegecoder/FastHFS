// Package appconfig 读取编译期内嵌的 app.config.json，作为应用名称/版本唯一来源。
package appconfig

import (
	"encoding/json"
	"fmt"
)

// AppConfig 应用元信息
type AppConfig struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Version           string `json:"version"`
	CompanyName       string `json:"companyName"`
	ProductIdentifier string `json:"productIdentifier"`
	Homepage          string `json:"homepage"`
	Copyright         string `json:"copyright"`
	Author            Author `json:"author"`
	Update            Update `json:"update"`
}

// Author 作者信息
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Blog  string `json:"blog"`
}

// Update 更新检测配置
type Update struct {
	APIURL          string `json:"apiUrl"`
	ReleasePageBase string `json:"releasePageBase"`
}

// Load 解析内嵌的应用配置
func Load(raw []byte) (AppConfig, error) {
	var cfg AppConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("解析 app.config.json 失败: %w", err)
	}
	if cfg.Name == "" {
		cfg.Name = "FastHFS"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.Description == "" {
		cfg.Description = "HTTP 文件服务"
	}
	return cfg, nil
}
