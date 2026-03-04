package handler

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
)

const sitemapCacheControl = "public, max-age=300"

type sitemapHandler struct {
	sitemapService *service.SitemapService
	webOrigin      string
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapDocumentURLRecord struct {
	SpaceID           string
	DocumentID        string
	DocumentRouteKey  string
	DocumentUpdatedAt time.Time
}

// NewSitemapHandler 创建 sitemap 输出处理器。
func NewSitemapHandler(
	sitemapService *service.SitemapService,
	webOrigin string,
) *sitemapHandler {
	return &sitemapHandler{
		sitemapService: sitemapService,
		webOrigin:      normalizeWebOrigin(webOrigin),
	}
}

// Sitemap 输出站点公开内容的 sitemap XML。
func (h *sitemapHandler) Sitemap(c *gin.Context) {
	if h == nil || h.sitemapService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	records, err := h.sitemapService.ListPublicDocuments(c.Request.Context())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	baseOrigin := h.resolveBaseOrigin(c)
	urls := buildSitemapURLs(baseOrigin, records)
	payload := sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	body, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Cache-Control", sitemapCacheControl)
	c.Data(
		http.StatusOK,
		"application/xml; charset=utf-8",
		append([]byte(xml.Header), body...),
	)
}

func (h *sitemapHandler) resolveBaseOrigin(c *gin.Context) string {
	if c != nil && c.Request != nil {
		scheme := firstForwardedValue(c.GetHeader("X-Forwarded-Proto"))
		if scheme == "" {
			if c.Request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}

		host := firstForwardedValue(c.GetHeader("X-Forwarded-Host"))
		if host == "" {
			host = strings.TrimSpace(c.Request.Host)
		}

		if host != "" {
			return normalizeWebOrigin(scheme + "://" + host)
		}
	}
	return normalizeWebOrigin(h.webOrigin)
}

func buildSitemapURLs(
	baseOrigin string,
	records []service.SitemapPublicDocumentRecord,
) []sitemapURL {
	items := make([]sitemapURL, 0, 1+len(records)*2)
	items = append(items, sitemapURL{
		Loc: buildSitemapLoc(baseOrigin, "/"),
	})

	spaceLastMod := make(map[string]time.Time, len(records))
	documentItems := make([]sitemapDocumentURLRecord, 0, len(records))
	for _, record := range records {
		spaceID := strings.TrimSpace(record.SpaceID)
		documentID := strings.TrimSpace(record.DocumentID)
		if spaceID == "" || documentID == "" {
			continue
		}

		documentItems = append(documentItems, sitemapDocumentURLRecord{
			SpaceID:           spaceID,
			DocumentID:        documentID,
			DocumentRouteKey:  strings.TrimSpace(record.DocumentRouteKey),
			DocumentUpdatedAt: record.DocumentUpdatedAt,
		})

		lastMod := record.DocumentUpdatedAt
		if lastMod.IsZero() || (!record.SpaceUpdatedAt.IsZero() && record.SpaceUpdatedAt.After(lastMod)) {
			lastMod = record.SpaceUpdatedAt
		}
		if current, ok := spaceLastMod[spaceID]; !ok || lastMod.After(current) {
			spaceLastMod[spaceID] = lastMod
		}
	}

	spaceIDs := make([]string, 0, len(spaceLastMod))
	for spaceID := range spaceLastMod {
		spaceIDs = append(spaceIDs, spaceID)
	}
	sort.Strings(spaceIDs)
	for _, spaceID := range spaceIDs {
		items = append(items, sitemapURL{
			Loc:     buildSitemapLoc(baseOrigin, "/r/"+escapeSitemapPathSegment(spaceID)),
			LastMod: formatSitemapLastMod(spaceLastMod[spaceID]),
		})
	}

	sort.Slice(documentItems, func(left, right int) bool {
		if documentItems[left].SpaceID == documentItems[right].SpaceID {
			return documentItems[left].DocumentID < documentItems[right].DocumentID
		}
		return documentItems[left].SpaceID < documentItems[right].SpaceID
	})
	for _, item := range documentItems {
		documentRouteKey := strings.TrimSpace(item.DocumentRouteKey)
		if documentRouteKey == "" {
			documentRouteKey = strings.TrimSpace(item.DocumentID)
		}
		items = append(items, sitemapURL{
			Loc: buildSitemapLoc(
				baseOrigin,
				"/r/"+escapeSitemapPathSegment(item.SpaceID)+"/"+escapeSitemapPathSegment(documentRouteKey),
			),
			LastMod: formatSitemapLastMod(item.DocumentUpdatedAt),
		})
	}
	return items
}

func buildSitemapLoc(baseOrigin string, path string) string {
	normalizedPath := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if baseOrigin == "" {
		return normalizedPath
	}
	return normalizeWebOrigin(baseOrigin) + normalizedPath
}

func escapeSitemapPathSegment(raw string) string {
	return url.PathEscape(strings.TrimSpace(raw))
}

func formatSitemapLastMod(raw time.Time) string {
	if raw.IsZero() {
		return ""
	}
	return raw.UTC().Format(time.RFC3339)
}
