package spreadsheet

import (
	"archive/zip"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/cellref"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func WriteXLSX(output io.Writer, request model.ExcelRequest) (model.ExcelSummary, error) {
	columns := append([]string(nil), request.Columns...)
	if len(columns) == 0 && len(request.Data) > 0 {
		for key := range request.Data[0] {
			columns = append(columns, key)
		}
		sort.Strings(columns)
	}
	if len(columns) == 0 {
		return model.ExcelSummary{}, ErrInvalidWorkbook
	}
	startValue := request.StartCell
	if startValue == "" {
		startValue = "A1"
	}
	selected, err := cellref.ParseRange(startValue)
	if err != nil || selected.Start != selected.End {
		return model.ExcelSummary{}, ErrInvalidWorkbook
	}
	start := selected.Start
	sheetName := sanitizeSheetName(request.SheetName)
	endRow := start.Row + len(request.Data)
	endCol := start.Col + len(columns) - 1
	rangeValue := cellName(start.Row, start.Col) + ":" + cellName(endRow, endCol)
	archive := zip.NewWriter(output)
	fixed := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	write := func(name, content string) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(fixed)
		writer, e := archive.CreateHeader(header)
		if e != nil {
			return e
		}
		_, e = io.WriteString(writer, content)
		return e
	}
	parts := [][2]string{{"[Content_Types].xml", contentTypesXML}, {"_rels/.rels", rootRelsXML}, {"xl/workbook.xml", fmt.Sprintf(workbookXML, xmlText(sheetName))}, {"xl/_rels/workbook.xml.rels", workbookRelsXML}, {"xl/styles.xml", stylesXML}, {"docProps/core.xml", coreXML}, {"xl/worksheets/sheet1.xml", worksheetXML(request, columns, start, endRow, endCol)}}
	for _, part := range parts {
		if err := write(part[0], part[1]); err != nil {
			return model.ExcelSummary{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return model.ExcelSummary{}, err
	}
	return model.ExcelSummary{Rows: len(request.Data), Columns: len(columns), SheetName: sheetName, Range: rangeValue}, nil
}

func worksheetXML(request model.ExcelRequest, columns []string, start cellref.Position, endRow, endCol int) string {
	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	if request.FreezeHeader {
		out.WriteString(fmt.Sprintf(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="%d" topLeftCell="%s" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`, start.Row+1, cellName(start.Row+1, start.Col)))
	}
	out.WriteString(`<sheetData>`)
	writeRow := func(row int, values []any) {
		out.WriteString(fmt.Sprintf(`<row r="%d">`, row+1))
		for index, value := range values {
			out.WriteString(xlsxCellXML(cellName(row, start.Col+index), value))
		}
		out.WriteString(`</row>`)
	}
	header := make([]any, len(columns))
	for i, v := range columns {
		header[i] = v
	}
	writeRow(start.Row, header)
	for index, row := range request.Data {
		values := make([]any, len(columns))
		for i, column := range columns {
			values[i] = row[column]
		}
		writeRow(start.Row+index+1, values)
	}
	out.WriteString(`</sheetData>`)
	if request.AutoFilter {
		out.WriteString(fmt.Sprintf(`<autoFilter ref="%s:%s"/>`, cellName(start.Row, start.Col), cellName(endRow, endCol)))
	}
	out.WriteString(`</worksheet>`)
	return out.String()
}
func xlsxCellXML(ref string, value any) string {
	switch v := value.(type) {
	case nil:
		return fmt.Sprintf(`<c r="%s"/>`, ref)
	case bool:
		n := 0
		if v {
			n = 1
		}
		return fmt.Sprintf(`<c r="%s" t="b"><v>%d</v></c>`, ref, n)
	case int:
		return fmt.Sprintf(`<c r="%s"><v>%d</v></c>`, ref, v)
	case int64:
		return fmt.Sprintf(`<c r="%s"><v>%d</v></c>`, ref, v)
	case float64:
		return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xmlText(fmt.Sprint(v)))
	}
}
func sanitizeSheetName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "Sheet1"
	}
	value = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\\`, r) || r < 32 {
			return ' '
		}
		return r
	}, value)
	runes := []rune(value)
	if len(runes) > 31 {
		runes = runes[:31]
	}
	return strings.TrimSpace(string(runes))
}
func cellName(row, column int) string {
	letters := ""
	for c := column + 1; c > 0; {
		c--
		letters = string(rune('A'+c%26)) + letters
		c /= 26
	}
	return letters + strconv.Itoa(row+1)
}
func xmlText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(value)
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`
const rootRelsXML = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>`
const workbookXML = `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="%s" sheetId="1" r:id="rId1"/></sheets></workbook>`
const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
const stylesXML = `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Arial"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>`
const coreXML = `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>Astron Table Tools</dc:creator></cp:coreProperties>`
