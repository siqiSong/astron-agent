package spreadsheet

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/cellref"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

const (
	maxZIPEntries       = 10_000
	maxCompressionRatio = 1_000
	workbookPart        = "xl/workbook.xml"
	workbookRelsPart    = "xl/_rels/workbook.xml.rels"
	sharedStringsPart   = "xl/sharedStrings.xml"
	stylesPart          = "xl/styles.xml"
)

type xlsxSheet struct {
	Name string
	RID  string
}

func readXLSX(data []byte, options Options) (model.Workbook, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return model.Workbook{}, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
	}
	parts, err := boundedZIPParts(archive, options.MaxExpandedBytes)
	if err != nil {
		return model.Workbook{}, err
	}
	workbookXML, ok := parts[workbookPart]
	if !ok {
		return model.Workbook{}, ErrInvalidWorkbook
	}
	sheets, date1904, err := parseWorkbook(workbookXML)
	if err != nil {
		return model.Workbook{}, err
	}
	relationships, err := parseRelationships(parts[workbookRelsPart])
	if err != nil {
		return model.Workbook{}, err
	}
	shared, err := parseSharedStrings(parts[sharedStringsPart])
	if err != nil {
		return model.Workbook{}, err
	}
	dateStyles, err := parseDateStyles(parts[stylesPart])
	if err != nil {
		return model.Workbook{}, err
	}

	selected := make([]xlsxSheet, 0, len(sheets))
	for _, sheet := range sheets {
		if options.SheetName == "" || sheet.Name == options.SheetName {
			selected = append(selected, sheet)
		}
	}
	if options.SheetName != "" && len(selected) == 0 {
		return model.Workbook{}, ErrSheetNotFound
	}

	workbook := model.Workbook{Sheets: make([]model.Sheet, 0, len(selected))}
	for _, sheet := range selected {
		target, ok := relationships[sheet.RID]
		if !ok {
			return model.Workbook{}, ErrInvalidWorkbook
		}
		worksheetPart, err := normalizeWorksheetTarget(target)
		if err != nil {
			return model.Workbook{}, err
		}
		worksheetXML, ok := parts[worksheetPart]
		if !ok {
			return model.Workbook{}, ErrInvalidWorkbook
		}
		rows, err := parseWorksheet(worksheetXML, shared, dateStyles, date1904, options)
		if err != nil {
			return model.Workbook{}, err
		}
		workbook.Sheets = append(workbook.Sheets, model.Sheet{Name: sheet.Name, Rows: rows})
	}
	return workbook, nil
}

func boundedZIPParts(archive *zip.Reader, maxExpanded int64) (map[string][]byte, error) {
	if len(archive.File) > maxZIPEntries {
		return nil, ErrExpandedLimit
	}
	var expanded uint64
	parts := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		expanded += file.UncompressedSize64
		if expanded > uint64(maxExpanded) || file.UncompressedSize64 > uint64(maxExpanded) {
			return nil, ErrExpandedLimit
		}
		if file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > maxCompressionRatio {
			return nil, ErrExpandedLimit
		}
		cleanName := strings.TrimPrefix(path.Clean("/"+file.Name), "/")
		if cleanName != file.Name || strings.HasPrefix(cleanName, "../") {
			return nil, ErrInvalidWorkbook
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, maxExpanded+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(content)) > maxExpanded {
			return nil, ErrExpandedLimit
		}
		parts[cleanName] = content
	}
	return parts, nil
}

func parseWorkbook(content []byte) ([]xlsxSheet, bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var sheets []xlsxSheet
	date1904 := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "workbookPr":
			date1904 = attr(start.Attr, "date1904") == "1" || strings.EqualFold(attr(start.Attr, "date1904"), "true")
		case "sheet":
			name, rid := attr(start.Attr, "name"), attr(start.Attr, "id")
			if name == "" || rid == "" {
				return nil, false, ErrInvalidWorkbook
			}
			sheets = append(sheets, xlsxSheet{Name: name, RID: rid})
		}
	}
	if len(sheets) == 0 {
		return nil, false, ErrInvalidWorkbook
	}
	return sheets, date1904, nil
}

func parseRelationships(content []byte) (map[string]string, error) {
	if len(content) == 0 {
		return nil, ErrInvalidWorkbook
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	relationships := make(map[string]string)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "Relationship" {
			relationships[attr(start.Attr, "Id")] = attr(start.Attr, "Target")
		}
	}
	return relationships, nil
}

func parseSharedStrings(content []byte) ([]string, error) {
	if len(content) == 0 {
		return nil, nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var stringsTable []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		value, err := collectText(decoder, "si")
		if err != nil {
			return nil, err
		}
		stringsTable = append(stringsTable, value)
	}
	return stringsTable, nil
}

func parseDateStyles(content []byte) (map[int]bool, error) {
	styles := make(map[int]bool)
	if len(content) == 0 {
		return styles, nil
	}
	customFormats := make(map[int]string)
	decoder := xml.NewDecoder(bytes.NewReader(content))
	inCellXFs := false
	styleIndex := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "numFmt":
				id, _ := strconv.Atoi(attr(value.Attr, "numFmtId"))
				customFormats[id] = attr(value.Attr, "formatCode")
			case "cellXfs":
				inCellXFs = true
				styleIndex = 0
			case "xf":
				if inCellXFs {
					id, _ := strconv.Atoi(attr(value.Attr, "numFmtId"))
					styles[styleIndex] = isDateFormat(id, customFormats[id])
					styleIndex++
				}
			}
		case xml.EndElement:
			if value.Name.Local == "cellXfs" {
				inCellXFs = false
			}
		}
	}
	return styles, nil
}

func isDateFormat(id int, format string) bool {
	if (id >= 14 && id <= 22) || (id >= 45 && id <= 47) {
		return true
	}
	format = strings.ToLower(format)
	format = strings.ReplaceAll(format, `\`, "")
	return strings.ContainsAny(format, "yd") || (strings.Contains(format, "m") && strings.ContainsAny(format, "hs"))
}

func parseWorksheet(content []byte, shared []string, dateStyles map[int]bool, date1904 bool, options Options) ([][]model.Cell, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	rows := make([][]model.Cell, 0)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "c" {
			continue
		}
		reference := attr(start.Attr, "r")
		positionRange, err := cellref.ParseRange(reference)
		if err != nil {
			return nil, ErrInvalidWorkbook
		}
		position := positionRange.Start
		if position.Row >= options.MaxRows {
			return nil, ErrRowLimit
		}
		if position.Col >= options.MaxColumns {
			return nil, ErrColumnLimit
		}
		cellType := attr(start.Attr, "t")
		styleIndex, _ := strconv.Atoi(attr(start.Attr, "s"))
		rawValue, inlineValue, err := parseXMLCell(decoder)
		if err != nil {
			return nil, err
		}
		cell, err := xlsxCell(cellType, rawValue, inlineValue, styleIndex, shared, dateStyles, date1904)
		if err != nil {
			return nil, err
		}
		for len(rows) <= position.Row {
			rows = append(rows, nil)
		}
		for len(rows[position.Row]) <= position.Col {
			rows[position.Row] = append(rows[position.Row], model.Cell{})
		}
		rows[position.Row][position.Col] = cell
	}
	return cropCells(rows, options.Range)
}

func parseXMLCell(decoder *xml.Decoder) (string, string, error) {
	var rawValue strings.Builder
	var inlineValue strings.Builder
	depth := 1
	inValue, inInline := false, false
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", "", fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if value.Name.Local == "v" {
				inValue = true
			}
			if value.Name.Local == "t" {
				inInline = true
			}
		case xml.CharData:
			if inValue {
				rawValue.Write(value)
			}
			if inInline {
				inlineValue.Write(value)
			}
		case xml.EndElement:
			if value.Name.Local == "v" {
				inValue = false
			}
			if value.Name.Local == "t" {
				inInline = false
			}
			depth--
		}
	}
	return rawValue.String(), inlineValue.String(), nil
}

func xlsxCell(cellType, rawValue, inlineValue string, styleIndex int, shared []string, dateStyles map[int]bool, date1904 bool) (model.Cell, error) {
	switch cellType {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(rawValue))
		if err != nil || index < 0 || index >= len(shared) {
			return model.Cell{}, ErrInvalidWorkbook
		}
		return model.Cell{Text: shared[index], Value: shared[index], Kind: "string"}, nil
	case "inlineStr":
		return model.Cell{Text: inlineValue, Value: inlineValue, Kind: "string"}, nil
	case "str", "e":
		return model.Cell{Text: rawValue, Value: rawValue, Kind: "string"}, nil
	case "b":
		value := strings.TrimSpace(rawValue) == "1" || strings.EqualFold(strings.TrimSpace(rawValue), "true")
		return model.Cell{Text: strconv.FormatBool(value), Value: value, Kind: "boolean"}, nil
	}
	if strings.TrimSpace(rawValue) == "" {
		return model.Cell{}, nil
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(rawValue), 64)
	if err != nil {
		return model.Cell{}, ErrInvalidWorkbook
	}
	if dateStyles[styleIndex] {
		text, kind := excelDate(number, date1904)
		return model.Cell{Text: text, Value: text, Kind: kind}, nil
	}
	if math.Trunc(number) == number && number >= math.MinInt64 && number <= math.MaxInt64 {
		integer := int64(number)
		return model.Cell{Text: strconv.FormatInt(integer, 10), Value: integer, Kind: "integer"}, nil
	}
	return model.Cell{Text: strconv.FormatFloat(number, 'f', -1, 64), Value: number, Kind: "number"}, nil
}

func excelDate(serial float64, date1904 bool) (string, string) {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	if date1904 {
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	duration := time.Duration(math.Round(serial * float64(24*time.Hour)))
	value := epoch.Add(duration)
	if value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 {
		return value.Format("2006-01-02"), "date"
	}
	return value.Format("2006-01-02 15:04:05"), "datetime"
}

func cropCells(rows [][]model.Cell, rangeValue string) ([][]model.Cell, error) {
	if strings.TrimSpace(rangeValue) == "" {
		return rows, nil
	}
	selected, err := cellref.ParseRange(rangeValue)
	if err != nil {
		return nil, err
	}
	output := make([][]model.Cell, selected.End.Row-selected.Start.Row+1)
	for rowIndex := range output {
		output[rowIndex] = make([]model.Cell, selected.End.Col-selected.Start.Col+1)
		sourceRow := selected.Start.Row + rowIndex
		if sourceRow >= len(rows) {
			continue
		}
		for columnIndex := range output[rowIndex] {
			sourceColumn := selected.Start.Col + columnIndex
			if sourceColumn < len(rows[sourceRow]) {
				output[rowIndex][columnIndex] = rows[sourceRow][sourceColumn]
			}
		}
	}
	return output, nil
}

func normalizeWorksheetTarget(target string) (string, error) {
	var cleaned string
	if strings.HasPrefix(target, "/") {
		cleaned = strings.TrimPrefix(path.Clean(target), "/")
	} else {
		cleaned = path.Clean(path.Join("xl", target))
	}
	if !strings.HasPrefix(cleaned, "xl/worksheets/") || strings.Contains(cleaned, "..") {
		return "", ErrInvalidWorkbook
	}
	return cleaned, nil
}

func collectText(decoder *xml.Decoder, endName string) (string, error) {
	var output strings.Builder
	depth := 1
	inText := false
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if value.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inText {
				output.Write(value)
			}
		case xml.EndElement:
			if value.Name.Local == "t" {
				inText = false
			}
			depth--
			if depth == 0 && value.Name.Local != endName {
				return "", ErrInvalidWorkbook
			}
		}
	}
	return output.String(), nil
}

func attr(attributes []xml.Attr, local string) string {
	for _, attribute := range attributes {
		if attribute.Name.Local == local {
			return attribute.Value
		}
	}
	return ""
}
