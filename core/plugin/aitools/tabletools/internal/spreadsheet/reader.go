package spreadsheet

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

var (
	ErrUnsupportedFormat   = errors.New("unsupported spreadsheet format")
	ErrUnsupportedEncoding = errors.New("unsupported text encoding")
	ErrInputLimit          = errors.New("spreadsheet input exceeds size limit")
	ErrExpandedLimit       = errors.New("expanded spreadsheet exceeds size limit")
	ErrRowLimit            = errors.New("spreadsheet row limit exceeded")
	ErrColumnLimit         = errors.New("spreadsheet column limit exceeded")
	ErrSheetNotFound       = errors.New("spreadsheet sheet not found")
	ErrInvalidWorkbook     = errors.New("invalid spreadsheet workbook")
	ErrEncryptedWorkbook   = errors.New("encrypted spreadsheet workbook is not supported")
)

type Options struct {
	Format           string
	FileName         string
	Encoding         string
	SheetName        string
	Range            string
	MaxInputBytes    int64
	MaxExpandedBytes int64
	MaxRows          int
	MaxColumns       int
}

func (options Options) defaults() Options {
	if options.Encoding == "" {
		options.Encoding = "auto"
	}
	if options.MaxInputBytes <= 0 {
		options.MaxInputBytes = 50 << 20
	}
	if options.MaxExpandedBytes <= 0 {
		options.MaxExpandedBytes = 256 << 20
	}
	if options.MaxRows <= 0 {
		options.MaxRows = 100_000
	}
	if options.MaxColumns <= 0 {
		options.MaxColumns = 1_024
	}
	return options
}

func Read(reader io.ReaderAt, size int64, options Options) (model.Workbook, error) {
	options = options.defaults()
	if size < 0 || size > options.MaxInputBytes {
		return model.Workbook{}, ErrInputLimit
	}
	data := make([]byte, size)
	if _, err := reader.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
		return model.Workbook{}, err
	}
	format := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(options.Format), "."))
	if format == "" {
		format = detectFormat(data, options.FileName)
	}
	switch format {
	case "csv":
		return readCSV(data, options)
	case "xlsx":
		return readXLSX(data, options)
	case "xls":
		return readXLS(data, options)
	default:
		return model.Workbook{}, ErrUnsupportedFormat
	}
}

func ReadFile(path string, options Options) (model.Workbook, error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Workbook{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return model.Workbook{}, err
	}
	if options.FileName == "" {
		options.FileName = info.Name()
	}
	return Read(file, info.Size(), options)
}

func detectFormat(data []byte, fileName string) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return "xls"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return "xlsx"
	}
	extension := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fileName), "."))
	if dot := strings.LastIndex(extension, "."); dot >= 0 {
		extension = extension[dot+1:]
	}
	if extension == "xls" || extension == "xlsx" || extension == "csv" {
		return extension
	}
	return "csv"
}
