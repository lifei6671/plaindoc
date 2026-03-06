package service

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestSitemapService_ListPublicDocuments_SecondGuardExcludesNonMarkdown(t *testing.T) {
	now := time.Now().UTC()
	svc := &SitemapService{
		sitemapRepo: sitemapServiceStubRepository{
			rows: []repository.SitemapPublicDocumentSourceRecord{
				{
					SpaceID:           "space-markdown",
					DocumentID:        "doc-markdown",
					DocumentRouteKey:  "doc-markdown",
					DocumentFormat:    models.DocumentFormatMarkdown,
					DocumentContentMD: "# markdown body",
					SpaceUpdatedAt:    now,
					DocumentUpdatedAt: now,
				},
				{
					SpaceID:           "space-office",
					DocumentID:        "doc-office",
					DocumentRouteKey:  "doc-office",
					DocumentFormat:    models.DocumentFormatDOCX,
					DocumentContentMD: "# office placeholder",
					SpaceUpdatedAt:    now,
					DocumentUpdatedAt: now,
				},
			},
		},
	}

	rows, err := svc.ListPublicDocuments(context.Background())
	if err != nil {
		t.Fatalf("list public documents failed: %v", err)
	}
	if len(rows) != 1 || rows[0].DocumentID != "doc-markdown" {
		t.Fatalf("expected only markdown document in sitemap, got %+v", rows)
	}
}

type sitemapServiceStubRepository struct {
	rows []repository.SitemapPublicDocumentSourceRecord
}

func (r sitemapServiceStubRepository) ListPublicDocuments(
	_ context.Context,
) ([]repository.SitemapPublicDocumentSourceRecord, error) {
	return r.rows, nil
}
