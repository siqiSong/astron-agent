package spreadsheet

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

func buildXLSX(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, content := range entries {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func representativeXLSX(t *testing.T) []byte {
	return buildXLSX(t, map[string]string{
		"xl/workbook.xml":            `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><workbookPr date1904="0"/><sheets><sheet name="数据" sheetId="1" r:id="rId1"/><sheet name="第二页" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="/xl/worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<?xml version="1.0"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>姓名</t></si><si><r><t>日</t></r><r><t>期</t></r></si></sst>`,
		"xl/styles.xml":              `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="inlineStr"><is><t>合计</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>张三</t></is></c><c r="B2" s="1"><v>46239</v></c><c r="C2"><f>SUM(40,2)</f><v>42</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>第二页内容</t></is></c></row></sheetData></worksheet>`,
	})
}

func TestReadXLSXSharedInlineFormulaDateAndSheets(t *testing.T) {
	data := representativeXLSX(t)
	wb, err := Read(bytes.NewReader(data), int64(len(data)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 || wb.Sheets[0].Name != "数据" || wb.Sheets[1].Name != "第二页" {
		t.Fatalf("sheets = %+v", wb.Sheets)
	}
	rows := wb.Sheets[0].Rows
	if rows[0][0].Text != "姓名" || rows[0][1].Text != "日期" || rows[1][0].Text != "张三" {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[1][1].Kind != "date" || rows[1][1].Text != "2026-08-05" {
		t.Fatalf("date cell = %+v", rows[1][1])
	}
	if rows[1][2].Text != "42" || rows[1][2].Kind != "integer" {
		t.Fatalf("formula cell = %+v", rows[1][2])
	}
}

func TestReadXLSXAppliesRange(t *testing.T) {
	data := representativeXLSX(t)
	wb, err := Read(bytes.NewReader(data), int64(len(data)), Options{SheetName: "数据", Range: "B1:C2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 1 || len(wb.Sheets[0].Rows) != 2 || wb.Sheets[0].Rows[1][1].Text != "42" {
		t.Fatalf("workbook = %+v", wb)
	}
}

func TestReadXLSXRejectsExpandedLimit(t *testing.T) {
	data := buildXLSX(t, map[string]string{"xl/workbook.xml": string(bytes.Repeat([]byte("x"), 128))})
	_, err := Read(bytes.NewReader(data), int64(len(data)), Options{MaxExpandedBytes: 32})
	if !errors.Is(err, ErrExpandedLimit) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadXLSXRejectsMissingSheet(t *testing.T) {
	data := representativeXLSX(t)
	_, err := Read(bytes.NewReader(data), int64(len(data)), Options{SheetName: "不存在"})
	if !errors.Is(err, ErrSheetNotFound) {
		t.Fatalf("err = %v", err)
	}
}
