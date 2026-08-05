package spreadsheet

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	internalencoding "github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/encoding"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

type biffSheet struct {
	name   string
	offset uint32
}

type biffRecordValue struct {
	id      uint16
	payload []byte
	offset  int
}

func readXLS(data []byte, options Options) (model.Workbook, error) {
	stream, err := extractWorkbookStream(data, options)
	if err != nil {
		return model.Workbook{}, err
	}
	return parseBIFF(stream, options)
}

func biffRecords(data []byte, start int) ([]biffRecordValue, error) {
	var records []biffRecordValue
	for offset := start; offset < len(data); {
		if offset+4 > len(data) {
			return nil, ErrInvalidWorkbook
		}
		length := int(binary.LittleEndian.Uint16(data[offset+2:]))
		if offset+4+length > len(data) {
			return nil, ErrInvalidWorkbook
		}
		records = append(records, biffRecordValue{id: binary.LittleEndian.Uint16(data[offset:]), payload: data[offset+4 : offset+4+length], offset: offset})
		offset += 4 + length
	}
	return records, nil
}

func parseBIFF(data []byte, options Options) (model.Workbook, error) {
	records, err := biffRecords(data, 0)
	if err != nil || len(records) == 0 || records[0].id != 0x0809 || len(records[0].payload) < 4 {
		return model.Workbook{}, ErrInvalidWorkbook
	}
	version := binary.LittleEndian.Uint16(records[0].payload)
	if version != 0x0600 && version != 0x0500 {
		return model.Workbook{}, ErrUnsupportedFormat
	}
	biff8 := version == 0x0600
	codepage, date1904 := uint16(936), false
	var sheets []biffSheet
	var shared []string
	var xfs []uint16
	formats := map[uint16]string{}
	for _, record := range records {
		switch record.id {
		case 0x002F:
			return model.Workbook{}, ErrEncryptedWorkbook
		case 0x0042:
			if len(record.payload) >= 2 {
				codepage = binary.LittleEndian.Uint16(record.payload)
			}
		case 0x0022:
			date1904 = len(record.payload) >= 2 && binary.LittleEndian.Uint16(record.payload) != 0
		case 0x0085:
			sheet, parseErr := parseBoundSheet(record.payload, biff8, codepage)
			if parseErr != nil {
				return model.Workbook{}, parseErr
			}
			sheets = append(sheets, sheet)
		case 0x00FC:
			shared, err = parseSST(record.payload)
			if err != nil {
				return model.Workbook{}, err
			}
		case 0x00E0, 0x0043:
			if len(record.payload) >= 4 {
				xfs = append(xfs, binary.LittleEndian.Uint16(record.payload[2:]))
			}
		case 0x041E, 0x001E:
			if len(record.payload) >= 3 {
				id := binary.LittleEndian.Uint16(record.payload)
				value, _, parseErr := parseShortString(record.payload[2:], biff8, codepage)
				if parseErr == nil {
					formats[id] = value
				}
			}
		}
	}
	if len(sheets) == 0 {
		return model.Workbook{}, ErrInvalidWorkbook
	}
	selected := make([]biffSheet, 0, len(sheets))
	for _, sheet := range sheets {
		if options.SheetName == "" || options.SheetName == sheet.name {
			selected = append(selected, sheet)
		}
	}
	if len(selected) == 0 {
		return model.Workbook{}, ErrSheetNotFound
	}
	workbook := model.Workbook{Sheets: make([]model.Sheet, 0, len(selected))}
	for _, sheet := range selected {
		if int(sheet.offset) >= len(data) {
			return model.Workbook{}, ErrInvalidWorkbook
		}
		rows, parseErr := parseBIFFSheet(data[sheet.offset:], shared, xfs, formats, date1904, biff8, codepage, options)
		if parseErr != nil {
			return model.Workbook{}, parseErr
		}
		workbook.Sheets = append(workbook.Sheets, model.Sheet{Name: sheet.name, Rows: rows})
	}
	return workbook, nil
}

func parseBoundSheet(payload []byte, biff8 bool, codepage uint16) (biffSheet, error) {
	if len(payload) < 7 {
		return biffSheet{}, ErrInvalidWorkbook
	}
	value, _, err := parseByteLengthString(payload[6:], biff8, codepage)
	return biffSheet{name: value, offset: binary.LittleEndian.Uint32(payload)}, err
}

func parseByteLengthString(data []byte, biff8 bool, codepage uint16) (string, int, error) {
	if len(data) < 1 {
		return "", 0, ErrInvalidWorkbook
	}
	length := int(data[0])
	if !biff8 {
		if len(data) < 1+length {
			return "", 0, ErrInvalidWorkbook
		}
		value, err := decodeLegacy(data[1:1+length], codepage)
		return value, 1 + length, err
	}
	if len(data) < 2 {
		return "", 0, ErrInvalidWorkbook
	}
	wide := data[1]&1 != 0
	width := 1
	if wide {
		width = 2
	}
	if len(data) < 2+length*width {
		return "", 0, ErrInvalidWorkbook
	}
	return decodeBIFF8Chars(data[2:2+length*width], length, wide), 2 + length*width, nil
}

func parseShortString(data []byte, biff8 bool, codepage uint16) (string, int, error) {
	if !biff8 {
		return parseByteLengthString(data, false, codepage)
	}
	if len(data) < 3 {
		return "", 0, ErrInvalidWorkbook
	}
	length := int(binary.LittleEndian.Uint16(data))
	flags := data[2]
	wide := flags&1 != 0
	width := 1
	if wide {
		width = 2
	}
	if len(data) < 3+length*width {
		return "", 0, ErrInvalidWorkbook
	}
	return decodeBIFF8Chars(data[3:3+length*width], length, wide), 3 + length*width, nil
}

func parseSST(payload []byte) ([]string, error) {
	if len(payload) < 8 {
		return nil, ErrInvalidWorkbook
	}
	unique := int(binary.LittleEndian.Uint32(payload[4:]))
	if unique < 0 || unique > 1_000_000 {
		return nil, ErrInvalidWorkbook
	}
	values := make([]string, 0, unique)
	offset := 8
	for len(values) < unique {
		if offset+3 > len(payload) {
			return nil, ErrInvalidWorkbook
		}
		length := int(binary.LittleEndian.Uint16(payload[offset:]))
		flags := payload[offset+2]
		offset += 3
		richRuns, extSize := 0, 0
		if flags&0x08 != 0 {
			if offset+2 > len(payload) {
				return nil, ErrInvalidWorkbook
			}
			richRuns = int(binary.LittleEndian.Uint16(payload[offset:]))
			offset += 2
		}
		if flags&0x04 != 0 {
			if offset+4 > len(payload) {
				return nil, ErrInvalidWorkbook
			}
			extSize = int(binary.LittleEndian.Uint32(payload[offset:]))
			offset += 4
		}
		wide := flags&1 != 0
		width := 1
		if wide {
			width = 2
		}
		needed := length*width + richRuns*4 + extSize
		if needed < 0 || offset+needed > len(payload) {
			return nil, ErrInvalidWorkbook
		}
		values = append(values, decodeBIFF8Chars(payload[offset:offset+length*width], length, wide))
		offset += needed
	}
	return values, nil
}

func parseBIFFSheet(data []byte, shared []string, xfs []uint16, formats map[uint16]string, date1904, biff8 bool, codepage uint16, options Options) ([][]model.Cell, error) {
	records, err := biffRecords(data, 0)
	if err != nil {
		return nil, err
	}
	var rows [][]model.Cell
	setCell := func(row, column, xf int, cell model.Cell) error {
		if row < 0 || row >= options.MaxRows {
			return ErrRowLimit
		}
		if column < 0 || column >= options.MaxColumns {
			return ErrColumnLimit
		}
		if xf >= 0 && xf < len(xfs) && isDateFormat(int(xfs[xf]), formats[xfs[xf]]) {
			var number float64
			var numeric bool
			switch value := cell.Value.(type) {
			case float64:
				number, numeric = value, true
			case int64:
				number, numeric = float64(value), true
			}
			if numeric {
				text, kind := excelDate(number, date1904)
				cell = model.Cell{Text: text, Value: text, Kind: kind}
			}
		}
		for len(rows) <= row {
			rows = append(rows, nil)
		}
		for len(rows[row]) <= column {
			rows[row] = append(rows[row], model.Cell{})
		}
		rows[row][column] = cell
		return nil
	}
	for _, record := range records {
		if record.id == 0x000A {
			break
		}
		payload := record.payload
		switch record.id {
		case 0x002F:
			return nil, ErrEncryptedWorkbook
		case 0x0203:
			if len(payload) < 14 {
				return nil, ErrInvalidWorkbook
			}
			number := math.Float64frombits(binary.LittleEndian.Uint64(payload[6:]))
			if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), numericCell(number)); err != nil {
				return nil, err
			}
		case 0x00FD:
			if len(payload) < 10 {
				return nil, ErrInvalidWorkbook
			}
			index := int(binary.LittleEndian.Uint32(payload[6:]))
			if index < 0 || index >= len(shared) {
				return nil, ErrInvalidWorkbook
			}
			if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), model.Cell{Text: shared[index], Value: shared[index], Kind: "string"}); err != nil {
				return nil, err
			}
		case 0x0204:
			if len(payload) < 8 {
				return nil, ErrInvalidWorkbook
			}
			value, _, parseErr := parseShortString(payload[6:], biff8, codepage)
			if parseErr != nil {
				return nil, parseErr
			}
			if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), model.Cell{Text: value, Value: value, Kind: "string"}); err != nil {
				return nil, err
			}
		case 0x0205:
			if len(payload) < 8 {
				return nil, ErrInvalidWorkbook
			}
			if payload[7] == 0 {
				value := payload[6] != 0
				if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), model.Cell{Text: strconv.FormatBool(value), Value: value, Kind: "boolean"}); err != nil {
					return nil, err
				}
			}
		case 0x027E:
			if len(payload) < 10 {
				return nil, ErrInvalidWorkbook
			}
			number := decodeRK(binary.LittleEndian.Uint32(payload[6:]))
			if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), numericCell(number)); err != nil {
				return nil, err
			}
		case 0x00BD:
			if len(payload) < 6 {
				return nil, ErrInvalidWorkbook
			}
			row, first := int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:]))
			count := (len(payload) - 6) / 6
			if len(payload) != 6+count*6 {
				return nil, ErrInvalidWorkbook
			}
			for index := 0; index < count; index++ {
				pos := 4 + index*6
				if err := setCell(row, first+index, int(binary.LittleEndian.Uint16(payload[pos:])), numericCell(decodeRK(binary.LittleEndian.Uint32(payload[pos+2:])))); err != nil {
					return nil, err
				}
			}
		case 0x0006:
			if len(payload) < 14 {
				return nil, ErrInvalidWorkbook
			}
			result := payload[6:14]
			if !(result[6] == 0xFF && result[7] == 0xFF) {
				number := math.Float64frombits(binary.LittleEndian.Uint64(result))
				if err := setCell(int(binary.LittleEndian.Uint16(payload)), int(binary.LittleEndian.Uint16(payload[2:])), int(binary.LittleEndian.Uint16(payload[4:])), numericCell(number)); err != nil {
					return nil, err
				}
			}
		}
	}
	return cropCells(rows, options.Range)
}

func numericCell(number float64) model.Cell {
	if math.Trunc(number) == number && number >= math.MinInt64 && number <= math.MaxInt64 {
		integer := int64(number)
		return model.Cell{Text: strconv.FormatInt(integer, 10), Value: integer, Kind: "integer"}
	}
	return model.Cell{Text: strconv.FormatFloat(number, 'f', -1, 64), Value: number, Kind: "number"}
}

func decodeRK(value uint32) float64 {
	div100, integer := value&1 != 0, value&2 != 0
	var result float64
	if integer {
		result = float64(int32(value) >> 2)
	} else {
		bits := uint64(value&0xFFFFFFFC) << 32
		result = math.Float64frombits(bits)
	}
	if div100 {
		result /= 100
	}
	return result
}

func decodeBIFF8Chars(data []byte, count int, wide bool) string {
	if !wide {
		return string(data[:count])
	}
	units := make([]uint16, count)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(data[index*2:])
	}
	return string(utf16.Decode(units))
}

func decodeLegacy(data []byte, codepage uint16) (string, error) {
	if codepage == 936 || codepage == 950 || codepage == 0 {
		value, err := internalencoding.DecodeGBK(data)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
		}
		return value, nil
	}
	if codepage == 65001 {
		return string(data), nil
	}
	var output strings.Builder
	for _, value := range data {
		output.WriteRune(rune(value))
	}
	return output.String(), nil
}
