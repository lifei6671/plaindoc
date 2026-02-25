package view

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
	"formatFriendlyRelativeTime": func(value time.Time) string {
		return formatFriendlyRelativeTime(value, time.Now())
	},
	"avatarInitial": avatarInitial,
}).ParseFS(homepageViewFS, "templates/*.tmpl", "templates/partials/*.tmpl"))

const (
	justNowThreshold = 5 * time.Second
	dayDuration      = 24 * time.Hour
	weekDuration     = 7 * dayDuration
	monthDuration    = 30 * dayDuration
	yearDuration     = 365 * dayDuration
)

func formatFriendlyRelativeTime(value time.Time, now time.Time) string {
	if value.IsZero() {
		return "时间未知"
	}

	delta := now.Sub(value)
	isFuture := delta < 0
	if isFuture {
		delta = -delta
	}

	if delta <= justNowThreshold {
		return "刚刚"
	}
	if delta < time.Minute {
		seconds := maxInt(1, int(delta/time.Second))
		if isFuture {
			return strconv.Itoa(seconds) + "秒后"
		}
		return strconv.Itoa(seconds) + "秒前"
	}
	if delta < time.Hour {
		minutes := maxInt(1, int(delta/time.Minute))
		if isFuture {
			return strconv.Itoa(minutes) + "分钟后"
		}
		return strconv.Itoa(minutes) + "分钟前"
	}
	if delta < dayDuration {
		hours := maxInt(1, int(delta/time.Hour))
		if isFuture {
			return strconv.Itoa(hours) + "小时后"
		}
		return strconv.Itoa(hours) + "小时前"
	}
	if delta < weekDuration {
		days := maxInt(1, int(delta/dayDuration))
		if isFuture {
			return strconv.Itoa(days) + "天后"
		}
		return strconv.Itoa(days) + "天前"
	}
	if delta < monthDuration {
		weeks := maxInt(1, int(delta/weekDuration))
		if isFuture {
			return strconv.Itoa(weeks) + "周后"
		}
		return strconv.Itoa(weeks) + "周前"
	}
	if delta < yearDuration {
		months := maxInt(1, int(delta/monthDuration))
		if isFuture {
			return strconv.Itoa(months) + "个月后"
		}
		return strconv.Itoa(months) + "个月前"
	}
	years := maxInt(1, int(delta/yearDuration))
	if isFuture {
		return strconv.Itoa(years) + "年后"
	}
	return strconv.Itoa(years) + "年前"
}

func avatarInitial(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "U"
	}
	r, _ := utf8.DecodeRuneInString(trimmed)
	if r == utf8.RuneError {
		return "U"
	}
	return strings.ToUpper(string(r))
}

func maxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

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
	OwnerName   string
	OwnerAvatar string
	UpdatedAt   time.Time
}

// HomePageViewData 首页/分类页模板数据。
type HomePageViewData struct {
	Title                string
	Description          string
	CanonicalURL         string
	SiteName             string
	IsExplore            bool
	IsAuthenticated      bool
	CanManageSpace       bool
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
