package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/artifact"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/chart"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/config"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/spreadsheet"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/tabular"
)

type Service struct {
	config config.Config
	store  *artifact.Store
}

func New(cfg config.Config, store *artifact.Store) *Service {
	return &Service{config: cfg, store: store}
}

type ExtractRequest struct {
	URL           string `json:"url,omitempty"`
	ExcelURL      string `json:"excel_url,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
	FileName      string `json:"file_name,omitempty"`
	Format        string `json:"format,omitempty"`
	Encoding      string `json:"encoding,omitempty"`
	SheetName     string `json:"sheet_name,omitempty"`
	StartCell     string `json:"start_cell,omitempty"`
	EndCell       string `json:"end_cell,omitempty"`
	Range         string `json:"range,omitempty"`
	HeaderRow     int    `json:"header_row,omitempty"`
	MaxResultRows int    `json:"max_result_rows,omitempty"`
}
type ExtractResult struct {
	Rows     []map[string]any `json:"rows"`
	JSON     string           `json:"json"`
	Markdown string           `json:"markdown"`
	Table    model.Table      `json:"table"`
}
type ChartResult struct {
	Path       string             `json:"path"`
	ImageURL   string             `json:"image_url"`
	ImageURLMD string             `json:"image_url_md"`
	Mapping    model.ChartMapping `json:"mapping"`
	Artifact   artifact.Artifact  `json:"artifact,omitempty"`
}
type ExcelRequest struct {
	Data         []map[string]any `json:"data"`
	Columns      []string         `json:"columns,omitempty"`
	SheetName    string           `json:"sheet_name,omitempty"`
	StartCell    string           `json:"start_cell,omitempty"`
	FreezeHeader bool             `json:"freeze_header,omitempty"`
	AutoFilter   bool             `json:"auto_filter,omitempty"`
}
type ExcelResult struct {
	Path      string             `json:"path"`
	FileURL   string             `json:"file_url"`
	FileURLMD string             `json:"file_url_md"`
	Summary   model.ExcelSummary `json:"summary"`
	Artifact  artifact.Artifact  `json:"artifact,omitempty"`
}

func (service *Service) Extract(ctx context.Context, request ExtractRequest) (ExtractResult, error) {
	var content []byte
	format := request.Format
	sourceURL := request.URL
	if sourceURL == "" {
		sourceURL = request.ExcelURL
	}
	if sourceURL != "" {
		fetched, err := artifact.Fetch(ctx, sourceURL, artifact.FetchPolicy{
			AllowedHosts:         service.config.AllowedDownloadHosts,
			MaxBytes:             service.config.MaxDownloadBytes,
			AllowPrivate:         true,
			RedirectAllowPrivate: true,
		})
		if err != nil {
			return ExtractResult{}, err
		}
		content, format = fetched.Data, fetched.Format
	} else {
		decoded, err := base64.StdEncoding.DecodeString(request.ContentBase64)
		if err != nil || len(decoded) == 0 {
			return ExtractResult{}, errors.New("url/excel_url or content_base64 is required")
		}
		content = decoded
	}
	rangeValue := request.Range
	if rangeValue == "" && request.StartCell != "" {
		rangeValue = request.StartCell
		if request.EndCell != "" {
			rangeValue += ":" + request.EndCell
		}
	}
	wb, err := spreadsheet.Read(bytes.NewReader(content), int64(len(content)), spreadsheet.Options{Format: format, FileName: request.FileName, Encoding: request.Encoding, SheetName: request.SheetName, Range: rangeValue, MaxInputBytes: service.config.MaxDownloadBytes, MaxExpandedBytes: service.config.MaxExpandedBytes, MaxRows: service.config.MaxRows, MaxColumns: service.config.MaxColumns})
	if err != nil {
		return ExtractResult{}, err
	}
	if len(wb.Sheets) == 0 {
		return ExtractResult{}, spreadsheet.ErrInvalidWorkbook
	}
	maxRows := request.MaxResultRows
	if maxRows <= 0 || maxRows > service.config.MaxRows {
		maxRows = service.config.MaxRows
	}
	table, err := tabular.Normalize(wb.Sheets[0].Rows, tabular.Options{HeaderRow: request.HeaderRow, MaxRows: maxRows})
	if err != nil {
		return ExtractResult{}, err
	}
	table.SheetNames = make([]string, len(wb.Sheets))
	for i, sheet := range wb.Sheets {
		table.SheetNames[i] = sheet.Name
	}
	table.SelectedSheet = wb.Sheets[0].Name
	table.Range = rangeValue
	jsonText, markdown, err := formatTable(table)
	if err != nil {
		return ExtractResult{}, err
	}
	return ExtractResult{Rows: table.Rows, JSON: jsonText, Markdown: markdown, Table: table}, nil
}
func (service *Service) BarChart(_ context.Context, request model.BarChartRequest) (ChartResult, error) {
	if len(request.Data) > service.config.MaxChartPoints {
		return ChartResult{}, fmt.Errorf("too many chart points")
	}
	svg, mapping, err := chart.Bar(request)
	if err != nil {
		return ChartResult{}, err
	}
	item, err := service.store.Write("svg", bytes.NewReader(svg))
	return chartResult(item, mapping, "柱形图"), err
}
func (service *Service) PieChart(_ context.Context, request model.PieChartRequest) (ChartResult, error) {
	if len(request.Data) > service.config.MaxChartPoints {
		return ChartResult{}, fmt.Errorf("too many chart points")
	}
	svg, mapping, err := chart.Pie(request)
	if err != nil {
		return ChartResult{}, err
	}
	item, err := service.store.Write("svg", bytes.NewReader(svg))
	return chartResult(item, mapping, "饼图"), err
}
func (service *Service) GenerateExcel(_ context.Context, request ExcelRequest) (ExcelResult, error) {
	var output bytes.Buffer
	summary, err := spreadsheet.WriteXLSX(&output, model.ExcelRequest{Data: request.Data, Columns: request.Columns, SheetName: request.SheetName, StartCell: request.StartCell, FreezeHeader: request.FreezeHeader, AutoFilter: request.AutoFilter})
	if err != nil {
		return ExcelResult{}, err
	}
	item, err := service.store.Write("xlsx", &output)
	return ExcelResult{Path: item.Path, FileURL: item.URL, FileURLMD: "[下载 Excel](" + item.URL + ")", Artifact: item, Summary: summary}, err
}

func chartResult(item artifact.Artifact, mapping model.ChartMapping, alt string) ChartResult {
	return ChartResult{
		Path:       item.Path,
		ImageURL:   item.URL,
		ImageURLMD: "![" + alt + "](" + item.URL + ")",
		Mapping:    mapping,
		Artifact:   item,
	}
}
func ContentType(id string) string {
	if strings.HasSuffix(id, ".svg") {
		return "image/svg+xml; charset=utf-8"
	}
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}
