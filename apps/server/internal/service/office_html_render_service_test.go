package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
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
	if err := workbook.SetCellValue("Detail", "A3", "Total"); err != nil {
		t.Fatalf("set detail A3 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "B3", "$1,000"); err != nil {
		t.Fatalf("set detail B3 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "A4", "Margin"); err != nil {
		t.Fatalf("set detail A4 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "B4", "12.5%"); err != nil {
		t.Fatalf("set detail B4 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "A5", "Updated"); err != nil {
		t.Fatalf("set detail A5 failed: %v", err)
	}
	if err := workbook.SetCellValue("Detail", "B5", "2026-03-08"); err != nil {
		t.Fatalf("set detail B5 failed: %v", err)
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
		`data-office-xlsx-fullscreen-toggle="1"`,
		`class="office-xlsx-tab office-xlsx-tab--active"`,
		`data-office-sheet-tab="sheet-1"`,
		`data-office-sheet-panel="sheet-2"`,
		`class="office-xlsx-sheet__axis office-xlsx-sheet__axis--corner"`,
		`<col class="office-xlsx-sheet__axis-col" style="width:58px" />`,
		`scope="col" class="office-xlsx-sheet__axis office-xlsx-sheet__axis--col">A</th>`,
		`scope="row" class="office-xlsx-sheet__axis office-xlsx-sheet__axis--row">1</th>`,
		`data-cell-ref="A1"`,
		`office-xlsx-sheet__cell office-xlsx-sheet__cell--merged`,
		`office-xlsx-sheet__row office-xlsx-sheet__row--header`,
		`office-xlsx-sheet__cell office-xlsx-sheet__cell--header`,
		`office-xlsx-sheet__cell office-xlsx-sheet__cell--numeric`,
		`office-xlsx-sheet__row office-xlsx-sheet__row--summary`,
		`office-xlsx-sheet__cell--currency`,
		`office-xlsx-sheet__cell--percent`,
		`office-xlsx-sheet__cell--date`,
		`Summary`,
		`Detail`,
		`colspan="2"`,
		`Budget`,
		`Server`,
		`Total`,
		`12.5%`,
		`2026-03-08`,
		`800`,
		`hidden`,
	} {
		if !strings.Contains(renderedHTML, expected) {
			t.Fatalf("expected rendered html to contain %q, got %q", expected, renderedHTML)
		}
	}
}

func TestRenderXLSXHTMLRendersPicturesAndChartWarning(t *testing.T) {
	t.Parallel()

	workbook := excelize.NewFile()
	workbook.SetSheetName("Sheet1", "Overview")
	if err := workbook.SetCellValue("Overview", "A1", "Name"); err != nil {
		t.Fatalf("set overview A1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Overview", "B1", "Value"); err != nil {
		t.Fatalf("set overview B1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Overview", "A2", "Alpha"); err != nil {
		t.Fatalf("set overview A2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Overview", "B2", 12); err != nil {
		t.Fatalf("set overview B2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Overview", "A3", "Beta"); err != nil {
		t.Fatalf("set overview A3 failed: %v", err)
	}
	if err := workbook.SetCellValue("Overview", "B3", 18); err != nil {
		t.Fatalf("set overview B3 failed: %v", err)
	}

	imageBytes, err := base64.StdEncoding.DecodeString(minimalPNGBase64)
	if err != nil {
		t.Fatalf("decode png failed: %v", err)
	}
	if err := workbook.AddPictureFromBytes("Overview", "D2", &excelize.Picture{
		Extension: ".png",
		File:      imageBytes,
		Format:    &excelize.GraphicOptions{AltText: "预算截图", Name: "Budget Snapshot"},
	}); err != nil {
		t.Fatalf("add picture failed: %v", err)
	}
	if err := workbook.AddChart("Overview", "F2", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{
			{
				Name:       "Overview!$A$2",
				Categories: "Overview!$A$2:$A$3",
				Values:     "Overview!$B$2:$B$3",
			},
		},
	}); err != nil {
		t.Fatalf("add chart failed: %v", err)
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
		`class="office-xlsx-alert"`,
		`当前文档存在复杂图表`,
		`office-xlsx-sheet__cell--with-media`,
		`class="office-xlsx-sheet__cell-media-image"`,
		`data:image/png;base64,`,
		`预算截图`,
		`D2`,
	} {
		if !strings.Contains(renderedHTML, expected) {
			t.Fatalf("expected rendered html to contain %q, got %q", expected, renderedHTML)
		}
	}
	if strings.Contains(renderedHTML, `class="office-xlsx-sheet__media"`) {
		t.Fatalf("expected rendered html to stop rendering detached media gallery, got %q", renderedHTML)
	}
}

func TestRenderXLSXHTMLKeepsChartSheetWithWarning(t *testing.T) {
	t.Parallel()

	workbook := excelize.NewFile()
	workbook.SetSheetName("Sheet1", "Data")
	if err := workbook.SetCellValue("Data", "A1", "Month"); err != nil {
		t.Fatalf("set data A1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Data", "B1", "Revenue"); err != nil {
		t.Fatalf("set data B1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Data", "A2", "Jan"); err != nil {
		t.Fatalf("set data A2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Data", "B2", 10); err != nil {
		t.Fatalf("set data B2 failed: %v", err)
	}
	if err := workbook.SetCellValue("Data", "A3", "Feb"); err != nil {
		t.Fatalf("set data A3 failed: %v", err)
	}
	if err := workbook.SetCellValue("Data", "B3", 18); err != nil {
		t.Fatalf("set data B3 failed: %v", err)
	}
	if err := workbook.AddChartSheet("ChartView", &excelize.Chart{
		Type: excelize.Line,
		Series: []excelize.ChartSeries{
			{
				Name:       "Data!$B$1",
				Categories: "Data!$A$2:$A$3",
				Values:     "Data!$B$2:$B$3",
			},
		},
	}); err != nil {
		t.Fatalf("add chart sheet failed: %v", err)
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
		`ChartView`,
		`class="office-xlsx-sheet__chart-warning"`,
		`当前工作表包含复杂图表`,
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

const minimalPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO5m7xkAAAAASUVORK5CYII="
