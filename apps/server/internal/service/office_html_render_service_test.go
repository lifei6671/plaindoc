package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/xuri/excelize/v2"
)

func TestOfficeHTMLRenderServiceRenderDOCXHTMLUsesMammoth(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime is required for mammoth rendering")
	}

	sourceContent := buildMinimalDOCX(t, "Hello Mammoth")
	service := &OfficeHTMLRenderService{}

	renderedHTML, err := service.renderDOCXHTML(context.Background(), OfficeHTMLRenderTask{
		DocumentID:    "doc-1",
		SpaceID:       "space-1",
		Format:        models.DocumentFormatDOCX,
		SourceContent: sourceContent,
	})
	if err != nil {
		t.Fatalf("renderDOCXHTML returned error: %v", err)
	}

	if !strings.Contains(renderedHTML, `<div class="office-docx-reader">`) {
		t.Fatalf("expected docx reader wrapper, got %q", renderedHTML)
	}
	if !strings.Contains(renderedHTML, "<p>Hello Mammoth</p>") {
		t.Fatalf("expected mammoth paragraph output, got %q", renderedHTML)
	}
}

func TestRenderXLSXHTMLRendersTabsAndMergedCells(t *testing.T) {
	t.Parallel()

	workbook := excelize.NewFile()
	workbook.SetSheetName("Sheet1", "Summary")
	if err := workbook.SetCellValue("Summary", "A1", "Budget"); err != nil {
		t.Fatalf("set summary A1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Summary", "A2", "Q1"); err != nil {
		t.Fatalf("set summary A2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Summary", "B2", 1200); err != nil {
		t.Fatalf("set summary B2 failed: %v", err)
	}
	if err := workbook.MergeCell("Summary", "A1", "B1"); err != nil {
		t.Fatalf("merge summary header failed: %v", err)
	}

	detailSheet, err := workbook.NewSheet("Detail")
	if err != nil {
		t.Fatalf("create detail sheet failed: %v", err)
	}
	workbook.SetActiveSheet(detailSheet)
	if err := workbook.SetCellValue("Detail", "A1", "Item"); err != nil {
		t.Fatalf("set detail A1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "B1", "Amount"); err != nil {
		t.Fatalf("set detail B1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "A2", "Server"); err != nil {
		t.Fatalf("set detail A2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "B2", 800); err != nil {
		t.Fatalf("set detail B2 failed: %v", err)
	}

	var buffer bytes.Buffer
	if err := workbook.Write(&buffer); err != nil {
		t.Fatalf("write workbook failed: %v", err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatalf("close workbook failed: %v", err)
	}

	renderedHTML, err := renderXLSXHTML(buffer.Bytes())
	if err != nil {
		t.Fatalf("renderXLSXHTML returned error: %v", err)
	}

	for _, expected := range []string{
		`class="office-xlsx-reader"`,
		`class="office-xlsx-tab office-xlsx-tab--active"`,
		`data-office-sheet-tab="sheet-1"`,
		`data-office-sheet-panel="sheet-2"`,
		`Summary`,
		`Detail`,
		`colspan="2"`,
		`Budget`,
		`Server`,
		`800`,
		`hidden`,
	} {
		if !strings.Contains(renderedHTML, expected) {
			t.Fatalf("expected rendered html to contain %q, got %q", expected, renderedHTML)
		}
	}
}

func buildMinimalDOCX(t *testing.T, text string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	}

	for name, content := range files {
		fileWriter, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create docx entry %s failed: %v", name, err)
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
			t.Fatalf("write docx entry %s failed: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close docx archive failed: %v", err)
	}
	return buffer.Bytes()
}
