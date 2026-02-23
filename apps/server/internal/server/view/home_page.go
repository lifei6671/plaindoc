package view

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"strings"
	"time"
)

//go:embed templates/*.tmpl templates/partials/*.tmpl static/*
var homepageViewFS embed.FS

var homepageTemplates = template.Must(template.New("homepage").Funcs(template.FuncMap{
	"formatPublishedAt": func(value time.Time) string {
		if value.IsZero() {
			return ""
		}
		return value.Format("2006-01-02")
	},
}).ParseFS(homepageViewFS, "templates/*.tmpl", "templates/partials/*.tmpl"))

// HomeCategoryViewData 首页/分类页分类导航项。
type HomeCategoryViewData struct {
	CategoryID string
	Name       string
	IsDefault  bool
	IsActive   bool
	URL        string
}

// HomeSpaceViewData 首页/分类页空间卡片项。
type HomeSpaceViewData struct {
	SpaceID     string
	Name        string
	Description string
	CoverURL    string
}

// HomePageViewData 首页/分类页模板数据。
type HomePageViewData struct {
	Title                string
	Description          string
	CanonicalURL         string
	SiteName             string
	IsExplore            bool
	IsAuthenticated      bool
	CurrentUserName      string
	CurrentUserAvatar    string
	CurrentUserAvatarURL string
	LoginURL             string
	RegisterURL          string
	AdminURL             string
	LogoutURL            string
	ActiveCategoryID     string
	ActiveCategoryName   string
	Categories           []HomeCategoryViewData
	Spaces               []HomeSpaceViewData
	Page                 int
	PageSize             int
	Total                int64
}

// RenderHomePage 渲染首页或分类页 HTML。
func RenderHomePage(data HomePageViewData) ([]byte, error) {
	templateName := "home.tmpl"
	if data.IsExplore {
		templateName = "explore.tmpl"
	}

	var buffer bytes.Buffer
	if err := homepageTemplates.ExecuteTemplate(&buffer, templateName, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// MustStaticFS 返回样式静态资源文件系统。
func MustStaticFS() fs.FS {
	subFS, err := fs.Sub(homepageViewFS, "static")
	if err != nil {
		panic(err)
	}
	return subFS
}

// NormalizeCanonicalPath 统一模板中的 canonical path 表示。
func NormalizeCanonicalPath(value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
