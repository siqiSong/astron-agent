package model

type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	SID     string `json:"sid"`
	Data    any    `json:"data"`
}

type Cell struct {
	Text  string
	Value any
	Kind  string
}

type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SourceIndex int    `json:"source_index"`
}

type Table struct {
	Columns       []Column         `json:"columns"`
	Rows          []map[string]any `json:"rows"`
	RowCount      int              `json:"row_count"`
	TotalRowCount int              `json:"total_row_count"`
	SheetNames    []string         `json:"sheet_names,omitempty"`
	SelectedSheet string           `json:"selected_sheet,omitempty"`
	Range         string           `json:"range,omitempty"`
	Truncated     bool             `json:"truncated"`
}

type Sheet struct {
	Name string
	Rows [][]Cell
}

type Workbook struct {
	Sheets []Sheet
}

type ChartMapping struct {
	CategoryField string   `json:"category_field"`
	ValueFields   []string `json:"value_fields"`
}

type BarChartRequest struct {
	Data          []map[string]any `json:"data"`
	CategoryField string           `json:"category_field,omitempty"`
	ValueFields   []string         `json:"value_fields,omitempty"`
	Title         string           `json:"title,omitempty"`
	Width         int              `json:"width,omitempty"`
	Height        int              `json:"height,omitempty"`
	Direction     string           `json:"direction,omitempty"`
	Layout        string           `json:"layout,omitempty"`
}

type PieChartRequest struct {
	Data           []map[string]any `json:"data"`
	CategoryField  string           `json:"category_field,omitempty"`
	ValueField     string           `json:"value_field,omitempty"`
	Title          string           `json:"title,omitempty"`
	Width          int              `json:"width,omitempty"`
	Height         int              `json:"height,omitempty"`
	Donut          bool             `json:"donut,omitempty"`
	OtherThreshold float64          `json:"other_threshold,omitempty"`
}

type ExcelRequest struct {
	Data         []map[string]any `json:"data"`
	Columns      []string         `json:"columns,omitempty"`
	SheetName    string           `json:"sheet_name,omitempty"`
	StartCell    string           `json:"start_cell,omitempty"`
	FreezeHeader bool             `json:"freeze_header,omitempty"`
	AutoFilter   bool             `json:"auto_filter,omitempty"`
}

type ExcelSummary struct {
	Rows      int    `json:"rows"`
	Columns   int    `json:"columns"`
	SheetName string `json:"sheet_name"`
	Range     string `json:"range"`
}
