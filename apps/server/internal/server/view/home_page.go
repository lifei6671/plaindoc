package view

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	"avatarInitial":       avatarInitial,
	"highlightSearchText": highlightSearchText,
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

type textHighlightRange struct {
	Start int
	End   int
}

func highlightSearchText(text string, keyword string) template.HTML {
	normalizedText := strings.TrimSpace(text)
	if normalizedText == "" {
		return template.HTML("")
	}

	highlightKeywords := buildHighlightKeywords(keyword)
	if len(highlightKeywords) == 0 {
		return template.HTML(template.HTMLEscapeString(normalizedText))
	}

	mergedRanges := collectHighlightRanges(normalizedText, highlightKeywords)
	if len(mergedRanges) == 0 {
		return template.HTML(template.HTMLEscapeString(normalizedText))
	}

	textRunes := []rune(normalizedText)
	var builder strings.Builder
	lastIndex := 0
	for _, item := range mergedRanges {
		if item.Start > lastIndex {
			builder.WriteString(template.HTMLEscapeString(string(textRunes[lastIndex:item.Start])))
		}
		builder.WriteString(`<mark class="yt-search-highlight">`)
		builder.WriteString(template.HTMLEscapeString(string(textRunes[item.Start:item.End])))
		builder.WriteString(`</mark>`)
		lastIndex = item.End
	}
	if lastIndex < len(textRunes) {
		builder.WriteString(template.HTMLEscapeString(string(textRunes[lastIndex:])))
	}
	return template.HTML(builder.String())
}

func buildHighlightKeywords(keyword string) []string {
	normalizedKeyword := strings.TrimSpace(keyword)
	if normalizedKeyword == "" {
		return []string{}
	}

	result := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	appendKeyword := func(raw string) {
		candidate := strings.ToLower(strings.TrimSpace(raw))
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}

	appendKeyword(normalizedKeyword)
	for _, item := range strings.Fields(normalizedKeyword) {
		appendKeyword(item)
	}
	for _, item := range extractCompoundHighlightVariants(normalizedKeyword) {
		appendKeyword(item)
	}
	for _, item := range splitHighlightKeywordsBySeparators(normalizedKeyword) {
		appendKeyword(item)
	}
	for _, item := range extractCJKBigrams(normalizedKeyword) {
		appendKeyword(item)
	}
	return result
}

func splitHighlightKeywordsBySeparators(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}
	}
	return strings.FieldsFunc(trimmed, func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}
		switch r {
		case '_', '-', '.', '/', '\\', ':':
			return true
		default:
			return false
		}
	})
}

func extractCompoundHighlightVariants(value string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	if len(fields) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(fields)*8)
	seen := make(map[string]struct{}, len(fields)*8)
	appendUnique := func(item string) {
		term := strings.TrimSpace(strings.ToLower(item))
		if term == "" {
			return
		}
		if _, exists := seen[term]; exists {
			return
		}
		seen[term] = struct{}{}
		result = append(result, term)
	}

	for _, item := range fields {
		if !strings.ContainsAny(item, "_-./\\:") {
			continue
		}
		segments := splitHighlightCompoundSegments(item)
		if len(segments) < 2 {
			continue
		}

		appendUnique(strings.Join(segments, " "))
		appendUnique(strings.Join(segments, "-"))
		appendUnique(strings.Join(segments, "."))
		appendUnique(strings.Join(segments, "/"))
		appendUnique(strings.Join(segments, ":"))
		appendUnique(strings.Join(segments, ""))
	}
	return result
}

func splitHighlightCompoundSegments(value string) []string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return []string{}
	}

	segments := make([]string, 0, 8)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		part := strings.TrimSpace(current.String())
		current.Reset()
		if part == "" {
			return
		}
		segments = append(segments, part)
	}

	for _, r := range trimmed {
		if isHighlightSeparator(r) {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return segments
}

func isHighlightSeparator(r rune) bool {
	switch r {
	case '_', '-', '.', '/', '\\', ':':
		return true
	default:
		return false
	}
}

func extractCJKBigrams(value string) []string {
	text := strings.TrimSpace(value)
	if text == "" {
		return []string{}
	}
	runes := []rune(text)
	if len(runes) < 2 {
		return []string{}
	}

	result := make([]string, 0, len(runes)-1)
	for index := 0; index < len(runes)-1; index++ {
		left := runes[index]
		right := runes[index+1]
		if !isHanRune(left) || !isHanRune(right) {
			continue
		}
		result = append(result, string([]rune{left, right}))
	}
	return result
}

func isHanRune(r rune) bool {
	return (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

func collectHighlightRanges(text string, keywords []string) []textHighlightRange {
	if strings.TrimSpace(text) == "" || len(keywords) == 0 {
		return []textHighlightRange{}
	}

	lowerText := strings.ToLower(text)
	ranges := make([]textHighlightRange, 0, 8)
	for _, keyword := range keywords {
		lowerKeyword := strings.ToLower(strings.TrimSpace(keyword))
		if lowerKeyword == "" {
			continue
		}
		for startByte := 0; startByte < len(lowerText); {
			matchOffset := strings.Index(lowerText[startByte:], lowerKeyword)
			if matchOffset < 0 {
				break
			}
			matchStartByte := startByte + matchOffset
			matchEndByte := matchStartByte + len(lowerKeyword)
			matchStartRune := utf8.RuneCountInString(lowerText[:matchStartByte])
			matchEndRune := matchStartRune + utf8.RuneCountInString(lowerText[matchStartByte:matchEndByte])
			if matchEndRune > matchStartRune {
				ranges = append(ranges, textHighlightRange{
					Start: matchStartRune,
					End:   matchEndRune,
				})
			}
			startByte = matchEndByte
		}
	}
	if len(ranges) == 0 {
		return []textHighlightRange{}
	}

	sort.Slice(ranges, func(left int, right int) bool {
		if ranges[left].Start == ranges[right].Start {
			return ranges[left].End < ranges[right].End
		}
		return ranges[left].Start < ranges[right].Start
	})

	merged := make([]textHighlightRange, 0, len(ranges))
	for _, item := range ranges {
		if len(merged) == 0 {
			merged = append(merged, item)
			continue
		}
		lastIndex := len(merged) - 1
		if item.Start > merged[lastIndex].End {
			merged = append(merged, item)
			continue
		}
		if item.End > merged[lastIndex].End {
			merged[lastIndex].End = item.End
		}
	}
	return merged
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

// HomeSearchHitViewData 首页全文检索命中项。
type HomeSearchHitViewData struct {
	SpaceID    string
	SpaceName  string
	DocumentID string
	Title      string
	Snippet    string
	UpdatedAt  time.Time
	URL        string
}

// HomePageViewData 首页/分类页模板数据。
type HomePageViewData struct {
	Title                string
	Description          string
	CanonicalURL         string
	SiteName             string
	SiteDescription      string
	FilingNumber         string
	FilingLink           string
	IsExplore            bool
	IsSearch             bool
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
	SearchEnabled        bool
	SearchActionURL      string
	SearchKeyword        string
	SearchResults        []HomeSearchHitViewData
	SearchPage           int
	SearchPageSize       int
	SearchTotal          int64
}

// RenderHomePage 渲染首页或分类页 HTML。
func RenderHomePage(data HomePageViewData) ([]byte, error) {
	templateName := "home.tmpl"
	if data.IsSearch {
		templateName = "search.tmpl"
	} else if data.IsExplore {
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
