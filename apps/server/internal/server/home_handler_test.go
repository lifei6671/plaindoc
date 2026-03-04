package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestRouter_Home_AnonymousUsesPublicCacheAndVisibility(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "home-owner@example.com")
	categoryID := "01homecate000000000000000001"
	seedHomepageCategory(t, database, categoryID, "产品文档", false)

	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeanonspace00000000000001",
		Name:         "Public Space A",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   categoryID,
		CategoryName: "产品文档",
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeanonspace00000000000002",
		Name:         "Public Space B",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   models.DefaultSpaceCategoryID,
		CategoryName: models.DefaultSpaceCategoryName,
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeanonspace00000000000003",
		Name:         "Authenticated Hidden",
		OwnerUserID:  ownerUserID,
		Visibility:   "authenticated",
		CategoryID:   categoryID,
		CategoryName: "产品文档",
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeanonspace00000000000004",
		Name:         "Member Hidden",
		OwnerUserID:  ownerUserID,
		Visibility:   "member",
		CategoryID:   categoryID,
		CategoryName: "产品文档",
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeanonspace00000000000005",
		Name:         "Public With AuthDoc Hidden",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   categoryID,
		CategoryName: "产品文档",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeanonspace00000000000001",
		NodeID:     "01homeanondocnode000000000001",
		DocumentID: "01homeanondocument000000000001",
		Title:      "Public Space A - Doc",
		Visibility: "public",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeanonspace00000000000002",
		NodeID:     "01homeanondocnode000000000002",
		DocumentID: "01homeanondocument000000000002",
		Title:      "Public Space B - Doc",
		Visibility: "public",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeanonspace00000000000003",
		NodeID:     "01homeanondocnode000000000003",
		DocumentID: "01homeanondocument000000000003",
		Title:      "Authenticated Hidden - Doc",
		Visibility: "authenticated",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeanonspace00000000000004",
		NodeID:     "01homeanondocnode000000000004",
		DocumentID: "01homeanondocument000000000004",
		Title:      "Member Hidden - Doc",
		Visibility: "member",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeanonspace00000000000005",
		NodeID:     "01homeanondocnode000000000005",
		DocumentID: "01homeanondocument000000000005",
		Title:      "Public With AuthDoc Hidden - Doc",
		Visibility: "authenticated",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected text/html content type, got %q", rec.Header().Get("Content-Type"))
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60, s-maxage=300, stale-while-revalidate=60" {
		t.Fatalf("unexpected cache-control header: %q", got)
	}
	varyHeader := rec.Header().Get("Vary")
	if !strings.Contains(varyHeader, "Authorization") || !strings.Contains(varyHeader, "Cookie") {
		t.Fatalf("expected Vary include Authorization/Cookie, got %q", varyHeader)
	}

	body := rec.Body.String()
	if strings.Contains(body, `class="yt-header-search-form"`) {
		t.Fatalf("expected anonymous homepage hides search form when search is not configured, body=%s", body)
	}
	if !strings.Contains(body, `class="yt-signin-button"`) || !strings.Contains(body, ">登录<") {
		t.Fatalf("expected anonymous nav has login button, body=%s", body)
	}
	if !strings.Contains(body, `class="yt-register-button"`) || !strings.Contains(body, ">注册<") {
		t.Fatalf("expected anonymous nav has register button, body=%s", body)
	}
	if !strings.Contains(body, `href="http://localhost:5173/login?redirect=http%3A%2F%2Fexample.com%2F"`) {
		t.Fatalf("expected login button links web auth page with redirect, body=%s", body)
	}
	if !strings.Contains(body, `href="http://localhost:5173/register?redirect=http%3A%2F%2Fexample.com%2F"`) {
		t.Fatalf("expected register button links web auth page with redirect, body=%s", body)
	}
	if !strings.Contains(body, "Public Space A") || !strings.Contains(body, "Public Space B") {
		t.Fatalf("expected anonymous page contains public spaces, body=%s", body)
	}
	if strings.Contains(body, "Authenticated Hidden") || strings.Contains(body, "Member Hidden") || strings.Contains(body, "Public With AuthDoc Hidden") {
		t.Fatalf("expected anonymous page hide non-public spaces, body=%s", body)
	}
	if !strings.Contains(body, "/explore/"+categoryID) {
		t.Fatalf("expected category link in body, body=%s", body)
	}
	if !strings.Contains(body, `class="yt-sidebar-item is-active" href="/"`) {
		t.Fatalf("expected homepage sidebar has active all-category link, body=%s", body)
	}
	allIndex := strings.Index(body, `href="/">`)
	defaultIndex := strings.Index(body, `/explore/`+models.DefaultSpaceCategoryID)
	if allIndex < 0 || defaultIndex < 0 || allIndex >= defaultIndex {
		t.Fatalf("expected all-category link keeps first position, body=%s", body)
	}
}

func TestRouter_Explore_LoggedInUsesNoStoreAndCategoryFilter(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "explore-owner@example.com")
	viewerUserID, _, viewerToken := registerAccessUser(t, serve, "explore-viewer@example.com")
	categoryID := "01homecate000000000000000010"
	emptyCategoryID := "01homecate000000000000000011"
	seedHomepageCategory(t, database, categoryID, "技术文档", false)
	seedHomepageCategory(t, database, emptyCategoryID, "空分类", false)

	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeexplorespace000000000001",
		Name:         "Public In Category",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   categoryID,
		CategoryName: "技术文档",
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeexplorespace000000000002",
		Name:         "Authenticated In Category",
		OwnerUserID:  ownerUserID,
		Visibility:   "authenticated",
		CategoryID:   categoryID,
		CategoryName: "技术文档",
	})
	memberSpaceID := "01homeexplorespace000000000003"
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      memberSpaceID,
		Name:         "Member In Category",
		OwnerUserID:  ownerUserID,
		Visibility:   "member",
		CategoryID:   categoryID,
		CategoryName: "技术文档",
	})
	seedHomepageSpaceMember(t, database, memberSpaceID, viewerUserID, "reader")
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homeexplorespace000000000004",
		Name:         "Public In Other Category",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   models.DefaultSpaceCategoryID,
		CategoryName: models.DefaultSpaceCategoryName,
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeexplorespace000000000001",
		NodeID:     "01homeexploredocnode000000000001",
		DocumentID: "01homeexploredocument000000000001",
		Title:      "Public In Category - Doc",
		Visibility: "public",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeexplorespace000000000002",
		NodeID:     "01homeexploredocnode000000000002",
		DocumentID: "01homeexploredocument000000000002",
		Title:      "Authenticated In Category - Doc",
		Visibility: "authenticated",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeexplorespace000000000003",
		NodeID:     "01homeexploredocnode000000000003",
		DocumentID: "01homeexploredocument000000000003",
		Title:      "Member In Category - Doc",
		Visibility: "member",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homeexplorespace000000000004",
		NodeID:     "01homeexploredocnode000000000004",
		DocumentID: "01homeexploredocument000000000004",
		Title:      "Public In Other Category - Doc",
		Visibility: "public",
	})

	req := httptest.NewRequest(http.MethodGet, "/explore/"+categoryID, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store, max-age=0" {
		t.Fatalf("unexpected cache-control for logged-in request: %q", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Test User") || !strings.Contains(body, "后台管理") || !strings.Contains(body, "退出登录") {
		t.Fatalf("expected logged-in nav dropdown items, body=%s", body)
	}
	if !strings.Contains(body, "Public In Category") ||
		!strings.Contains(body, "Authenticated In Category") ||
		!strings.Contains(body, "Member In Category") {
		t.Fatalf("expected category page contains visible category spaces, body=%s", body)
	}
	if strings.Contains(body, "Public In Other Category") {
		t.Fatalf("expected category page filter by category id, body=%s", body)
	}

	emptyReq := httptest.NewRequest(http.MethodGet, "/explore/"+emptyCategoryID, nil)
	emptyReq.Header.Set("Authorization", "Bearer "+viewerToken)
	emptyRec := serve(emptyReq)
	if emptyRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for empty category, got %d body=%s", emptyRec.Code, emptyRec.Body.String())
	}
	if !strings.Contains(emptyRec.Body.String(), "该分类下暂无可见空间") {
		t.Fatalf("expected empty category message, body=%s", emptyRec.Body.String())
	}

	allReq := httptest.NewRequest(http.MethodGet, "/explore/all", nil)
	allReq.Header.Set("Authorization", "Bearer "+viewerToken)
	allRec := serve(allReq)
	if allRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for explore all, got %d body=%s", allRec.Code, allRec.Body.String())
	}
	allBody := allRec.Body.String()
	if !strings.Contains(allBody, `class="yt-sidebar-item is-active" href="/explore/all"`) {
		t.Fatalf("expected explore all sidebar category active, body=%s", allBody)
	}
	exploreAllIndex := strings.Index(allBody, `href="/explore/all"`)
	exploreDefaultIndex := strings.Index(allBody, `/explore/`+models.DefaultSpaceCategoryID)
	if exploreAllIndex < 0 || exploreDefaultIndex < 0 || exploreAllIndex >= exploreDefaultIndex {
		t.Fatalf("expected explore all sidebar category keeps first position, body=%s", allBody)
	}
	if !strings.Contains(allBody, "Public In Category") ||
		!strings.Contains(allBody, "Authenticated In Category") ||
		!strings.Contains(allBody, "Member In Category") ||
		!strings.Contains(allBody, "Public In Other Category") {
		t.Fatalf("expected explore all page contains all visible spaces, body=%s", allBody)
	}
}

func TestRouter_HomeSearchEnabled_ShowsSearchBoxAndResults(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, _ := registerAccessUser(t, serve, "home-search-owner@example.com")
	seedHomepageSearchConfig(t, database)

	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homesearchspace000000000001",
		Name:         "公开检索空间",
		OwnerUserID:  ownerUserID,
		Visibility:   "public",
		CategoryID:   models.DefaultSpaceCategoryID,
		CategoryName: models.DefaultSpaceCategoryName,
	})
	seedHomepageSpace(t, database, homepageSeedSpaceInput{
		SpaceID:      "01homesearchspace000000000002",
		Name:         "仅成员空间",
		OwnerUserID:  ownerUserID,
		Visibility:   "member",
		CategoryID:   models.DefaultSpaceCategoryID,
		CategoryName: models.DefaultSpaceCategoryName,
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homesearchspace000000000001",
		NodeID:     "01homesearchdocnode000000000001",
		DocumentID: "01homesearchdocument00000000001",
		Title:      "检索命中文档",
		Visibility: "public",
	})
	seedHomepageDocument(t, database, homepageSeedDocumentInput{
		SpaceID:    "01homesearchspace000000000002",
		NodeID:     "01homesearchdocnode000000000002",
		DocumentID: "01homesearchdocument00000000002",
		Title:      "不应被匿名检索命中",
		Visibility: "member",
	})

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRec := serve(homeReq)
	if homeRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", homeRec.Code, homeRec.Body.String())
	}
	homeBody := homeRec.Body.String()
	if !strings.Contains(homeBody, `class="yt-header-search-form"`) {
		t.Fatalf("expected homepage renders search form after enabling search config, body=%s", homeBody)
	}
	if !strings.Contains(homeBody, `action="/search"`) {
		t.Fatalf("expected homepage search form action to /search, body=%s", homeBody)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/search?q=命中", nil)
	searchRec := serve(searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /search, got %d body=%s", searchRec.Code, searchRec.Body.String())
	}
	searchBody := searchRec.Body.String()
	if !strings.Contains(searchBody, `检索<mark class="yt-search-highlight">命中</mark>文档`) {
		t.Fatalf("expected search page contains matched document title, body=%s", searchBody)
	}
	if !strings.Contains(searchBody, "/r/01homesearchspace000000000001/01homesearchdocument00000000001") {
		t.Fatalf("expected search result links reader page, body=%s", searchBody)
	}
	if strings.Contains(searchBody, "/r/01homesearchspace000000000002/01homesearchdocument00000000002") {
		t.Fatalf("expected anonymous search result excludes member-only document, body=%s", searchBody)
	}
}

func TestRouter_HomePages_RenderFooterWithSiteFilingNumber(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	seedHomepageSiteConfig(t, database, "沪ICP备2026000001号", "https://beian.miit.gov.cn/")

	paths := []string{"/", "/explore/all", "/search"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := serve(req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %s, got %d body=%s", path, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if !strings.Contains(body, "一个适合中小团队文档在线管理系统") {
			t.Fatalf("expected footer contains site description on %s, body=%s", path, body)
		}
		if !strings.Contains(body, "备案号：") || !strings.Contains(body, "沪ICP备2026000001号") {
			t.Fatalf("expected footer contains filing number on %s, body=%s", path, body)
		}
		if !strings.Contains(body, "https://beian.miit.gov.cn/") {
			t.Fatalf("expected footer filing link exists on %s, body=%s", path, body)
		}
	}
}

func TestRouter_HomeLogoutRedirectsAndClearsCookie(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	_, _, accessToken := registerAccessUser(t, serve, "home-logout@example.com")

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "accessToken",
		Value: accessToken,
		Path:  "/",
	})
	rec := serve(req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d body=%s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/" {
		t.Fatalf("expected redirect to /, got %q", location)
	}
	setCookieHeader := rec.Header().Values("Set-Cookie")
	if len(setCookieHeader) == 0 {
		t.Fatal("expected set-cookie headers for clearing session cookies")
	}
	joined := strings.Join(setCookieHeader, "\n")
	if !strings.Contains(joined, "accessToken=") || !strings.Contains(joined, "refreshToken=") {
		t.Fatalf("expected clear accessToken/refreshToken cookies, got %v", setCookieHeader)
	}
}

type homepageSeedSpaceInput struct {
	SpaceID      string
	Name         string
	OwnerUserID  string
	Visibility   string
	CategoryID   string
	CategoryName string
}

type homepageSeedDocumentInput struct {
	SpaceID    string
	NodeID     string
	DocumentID string
	Title      string
	Visibility string
}

func seedHomepageCategory(
	t *testing.T,
	database *storage.Database,
	categoryID string,
	name string,
	isDefault bool,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_categories").Create(map[string]any{
		"category_id": categoryID,
		"name":        name,
		"is_default":  isDefault,
		"created_at":  now,
		"updated_at":  now,
	}).Error; err != nil {
		t.Fatalf("insert space category failed: %v", err)
	}
}

func seedHomepageSpace(t *testing.T, database *storage.Database, input homepageSeedSpaceInput) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("spaces").Create(map[string]any{
		"space_id":      input.SpaceID,
		"name":          input.Name,
		"description":   input.Name + " description",
		"category_id":   input.CategoryID,
		"category":      input.CategoryName,
		"owner_user_id": input.OwnerUserID,
		"visibility":    input.Visibility,
		"cover_url":     "https://example.com/" + input.SpaceID + ".webp",
		"status":        "active",
		"created_at":    now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatalf("insert homepage space failed: %v", err)
	}
}

func seedHomepageSpaceMember(
	t *testing.T,
	database *storage.Database,
	spaceID string,
	userID string,
	role string,
) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("space_members").Create(map[string]any{
		"space_id":   spaceID,
		"user_id":    userID,
		"role":       role,
		"created_at": now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("insert homepage space member failed: %v", err)
	}
}

func seedHomepageDocument(t *testing.T, database *storage.Database, input homepageSeedDocumentInput) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("nodes").Create(map[string]any{
		"node_id":        input.NodeID,
		"space_id":       input.SpaceID,
		"parent_node_id": nil,
		"type":           "doc",
		"title":          input.Title,
		"sort":           1,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("insert homepage doc node failed: %v", err)
	}

	if err := database.ORM.Table("documents").Create(map[string]any{
		"document_id": input.DocumentID,
		"node_id":     input.NodeID,
		"theme_id":    "default",
		"visibility":  input.Visibility,
		"status":      "active",
		"title":       input.Title,
		"content_md":  "# " + input.Title,
		"version":     1,
		"created_at":  now,
		"updated_at":  now,
	}).Error; err != nil {
		t.Fatalf("insert homepage document failed: %v", err)
	}
}

func seedHomepageSearchConfig(t *testing.T, database *storage.Database) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "search",
		"config_value_json":  `{"enabled":true,"activeProvider":"database","fallbackPolicy":"degrade_to_bleve","analysis":{"activeAnalyzer":"simple","analyzers":{"simple":{"enabled":true},"jieba":{"enabled":false,"mode":"search","hmm":true,"stopwordsEnabled":false,"dictSource":"db","dictVersion":"default"}}}}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert homepage search config failed: %v", err)
	}
}

func seedHomepageSiteConfig(t *testing.T, database *storage.Database, filingNumber string, filingLink string) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := database.ORM.Table("system_configs").Create(map[string]any{
		"config_key":         "site",
		"config_value_json":  `{"allowRegistration":true,"defaultSpaceVisibility":"member","filingNumber":"` + filingNumber + `","filingLink":"` + filingLink + `"}`,
		"version":            1,
		"updated_by_user_id": nil,
		"created_at":         now,
		"updated_at":         now,
	}).Error; err != nil {
		t.Fatalf("insert homepage site config failed: %v", err)
	}
}
