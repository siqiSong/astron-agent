package tabular

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

var ErrInvalidHeaderRow = errors.New("invalid header row")

type Options struct {
	HeaderRow int
	MaxRows   int
}

func Normalize(input [][]model.Cell, options Options) (model.Table, error) {
	if options.HeaderRow < 0 || options.HeaderRow >= len(input) {
		return model.Table{}, ErrInvalidHeaderRow
	}
	maxColumns := 0
	for _, row := range input[options.HeaderRow:] {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}
	if maxColumns == 0 {
		return model.Table{Columns: []model.Column{}, Rows: []map[string]any{}}, nil
	}

	names := headerNames(input[options.HeaderRow], maxColumns)
	dataRows := input[options.HeaderRow+1:]
	totalRows := len(dataRows)
	maxRows := options.MaxRows
	if maxRows <= 0 {
		maxRows = totalRows
	}
	if len(dataRows) > maxRows {
		dataRows = dataRows[:maxRows]
	}

	columnTypes := make([]string, maxColumns)
	rows := make([]map[string]any, 0, len(dataRows))
	for _, inputRow := range dataRows {
		row := make(map[string]any, maxColumns)
		for column := 0; column < maxColumns; column++ {
			cell := model.Cell{}
			if column < len(inputRow) {
				cell = inputRow[column]
			}
			value, kind := typedValue(cell)
			row[names[column]] = value
			columnTypes[column] = mergeType(columnTypes[column], kind)
		}
		rows = append(rows, row)
	}

	columns := make([]model.Column, maxColumns)
	for index, name := range names {
		kind := columnTypes[index]
		if kind == "" {
			kind = "string"
		}
		columns[index] = model.Column{Name: name, Type: kind, SourceIndex: index}
	}
	return model.Table{
		Columns:       columns,
		Rows:          rows,
		RowCount:      len(rows),
		TotalRowCount: totalRows,
		Truncated:     totalRows > len(rows),
	}, nil
}

func headerNames(header []model.Cell, columns int) []string {
	names := make([]string, columns)
	counts := make(map[string]int, columns)
	for index := 0; index < columns; index++ {
		name := ""
		if index < len(header) {
			name = strings.TrimSpace(header[index].Text)
		}
		if name == "" {
			name = fmt.Sprintf("column_%d", index+1)
		}
		counts[name]++
		if counts[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, counts[name])
		}
		names[index] = name
	}
	return names
}

func typedValue(cell model.Cell) (any, string) {
	if cell.Value != nil && cell.Kind != "" {
		return cell.Value, cell.Kind
	}
	text := strings.TrimSpace(cell.Text)
	if text == "" {
		return nil, ""
	}
	if strings.EqualFold(text, "true") {
		return true, "boolean"
	}
	if strings.EqualFold(text, "false") {
		return false, "boolean"
	}
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return value, "integer"
	}
	if value, err := strconv.ParseFloat(text, 64); err == nil {
		return value, "number"
	}
	if _, err := time.Parse("2006-01-02", text); err == nil {
		return text, "date"
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if _, err := time.Parse(layout, text); err == nil {
			return text, "datetime"
		}
	}
	return cell.Text, "string"
}

func mergeType(current, next string) string {
	if next == "" {
		return current
	}
	if current == "" || current == next {
		return next
	}
	if (current == "integer" && next == "number") || (current == "number" && next == "integer") {
		return "number"
	}
	if (current == "date" && next == "datetime") || (current == "datetime" && next == "date") {
		return "datetime"
	}
	return "string"
}
