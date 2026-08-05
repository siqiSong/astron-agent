package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/cellref"
	textencoding "github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/encoding"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func readCSV(data []byte, options Options) (model.Workbook, error) {
	text, err := decodeCSV(data, options.Encoding)
	if err != nil {
		return model.Workbook{}, err
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false
	records := make([][]string, 0)
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return model.Workbook{}, readErr
		}
		if len(record) > options.MaxColumns {
			return model.Workbook{}, ErrColumnLimit
		}
		if len(records) >= options.MaxRows {
			return model.Workbook{}, ErrRowLimit
		}
		records = append(records, append([]string(nil), record...))
	}
	rows, err := cropCSV(records, options.Range)
	if err != nil {
		return model.Workbook{}, err
	}
	cellRows := make([][]model.Cell, len(rows))
	for rowIndex, row := range rows {
		cellRows[rowIndex] = make([]model.Cell, len(row))
		for columnIndex, value := range row {
			cellRows[rowIndex][columnIndex] = model.Cell{Text: value}
		}
	}
	return model.Workbook{Sheets: []model.Sheet{{Name: "Sheet1", Rows: cellRows}}}, nil
}

func decodeCSV(data []byte, encoding string) (string, error) {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	switch encoding {
	case "", "auto":
		if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
			data = data[3:]
		}
		if utf8.Valid(data) {
			return string(data), nil
		}
		return textencoding.DecodeGBK(data)
	case "utf-8", "utf8", "utf-8-bom":
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if !utf8.Valid(data) {
			return "", textencoding.ErrInvalidGBK
		}
		return string(data), nil
	case "gbk", "gb2312", "cp936":
		return textencoding.DecodeGBK(data)
	default:
		return "", ErrUnsupportedEncoding
	}
}

func cropCSV(records [][]string, rangeValue string) ([][]string, error) {
	if strings.TrimSpace(rangeValue) == "" {
		return records, nil
	}
	selected, err := cellref.ParseRange(rangeValue)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, selected.End.Row-selected.Start.Row+1)
	for rowIndex := range rows {
		rows[rowIndex] = make([]string, selected.End.Col-selected.Start.Col+1)
		sourceRow := selected.Start.Row + rowIndex
		if sourceRow >= len(records) {
			continue
		}
		for columnIndex := range rows[rowIndex] {
			sourceColumn := selected.Start.Col + columnIndex
			if sourceColumn < len(records[sourceRow]) {
				rows[rowIndex][columnIndex] = records[sourceRow][sourceColumn]
			}
		}
	}
	return rows, nil
}
