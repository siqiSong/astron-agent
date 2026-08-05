package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/artifact"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/config"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func testService(t *testing.T) *Service {
	store, err := artifact.NewStore(t.TempDir(), "https://agent.example.com/aitools-files", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.AllowedDownloadHosts = nil
	return New(cfg, store)
}

func resultFields(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestExtractAllowsExactPrivateDownloadHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("姓名,年龄\n张三,20\n")) }))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	store, _ := artifact.NewStore(t.TempDir(), "/files", time.Hour)
	cfg := config.Default()
	cfg.AllowedDownloadHosts = []string{parsed.Hostname()}
	service := New(cfg, store)
	result, err := service.Extract(context.Background(), ExtractRequest{URL: server.URL + "/table.csv"})
	if err != nil || result.Table.RowCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestExtractBase64CSV(t *testing.T) {
	service := testService(t)
	content := base64.StdEncoding.EncodeToString([]byte("姓名,年龄\n张三,20\n"))
	result, err := service.Extract(context.Background(), ExtractRequest{ContentBase64: content, FileName: "a.csv"})
	if err != nil || result.Table.RowCount != 1 || result.Rows[0]["姓名"] != "张三" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestExtractReturnsRowsJSONMarkdownAndTableMetadata(t *testing.T) {
	service := testService(t)
	content := base64.StdEncoding.EncodeToString([]byte("姓名,备注\n张三,第一行|第二行\\完成\n"))
	result, err := service.Extract(context.Background(), ExtractRequest{ContentBase64: content, FileName: "a.csv"})
	if err != nil {
		t.Fatal(err)
	}
	fields := resultFields(t, result)
	if fields["json"] != `[{"备注":"第一行|第二行\\完成","姓名":"张三"}]` {
		t.Fatalf("json=%v", fields["json"])
	}
	wantMarkdown := "| 姓名 | 备注 |\n| --- | --- |\n| 张三 | 第一行\\|第二行\\\\完成 |"
	if fields["markdown"] != wantMarkdown {
		t.Fatalf("markdown=%q want=%q", fields["markdown"], wantMarkdown)
	}
	rows, ok := fields["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows=%#v", fields["rows"])
	}
	table, ok := fields["table"].(map[string]any)
	if !ok || table["row_count"] != float64(1) {
		t.Fatalf("table=%#v", fields["table"])
	}
}

func TestChartResultsExposeImageRepresentations(t *testing.T) {
	service := testService(t)
	tests := []struct {
		name string
		call func() (ChartResult, error)
		alt  string
	}{
		{name: "bar", alt: "柱形图", call: func() (ChartResult, error) {
			return service.BarChart(context.Background(), model.BarChartRequest{Data: []map[string]any{{"部门": "法学院", "人数": 12}}})
		}},
		{name: "pie", alt: "饼图", call: func() (ChartResult, error) {
			return service.PieChart(context.Background(), model.PieChartRequest{Data: []map[string]any{{"部门": "法学院", "人数": 12}}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.call()
			if err != nil {
				t.Fatal(err)
			}
			fields := resultFields(t, result)
			path, _ := fields["path"].(string)
			imageURL, _ := fields["image_url"].(string)
			if !strings.HasPrefix(path, "/aitools-files/") || imageURL != "https://agent.example.com"+path {
				t.Fatalf("fields=%#v", fields)
			}
			if fields["image_url_md"] != "!["+test.alt+"]("+imageURL+")" {
				t.Fatalf("fields=%#v", fields)
			}
		})
	}
}

func TestGenerateExcelCreatesArtifact(t *testing.T) {
	service := testService(t)
	result, err := service.GenerateExcel(context.Background(), ExcelRequest{Data: []map[string]any{{"姓名": "张三"}}, Columns: []string{"姓名"}})
	if err != nil || result.Artifact.ID == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestGenerateExcelExposesFileRepresentations(t *testing.T) {
	service := testService(t)
	result, err := service.GenerateExcel(context.Background(), ExcelRequest{Data: []map[string]any{{"姓名": "张三"}}, Columns: []string{"姓名"}})
	if err != nil {
		t.Fatal(err)
	}
	fields := resultFields(t, result)
	path, _ := fields["path"].(string)
	fileURL, _ := fields["file_url"].(string)
	if !strings.HasPrefix(path, "/aitools-files/") || fileURL != "https://agent.example.com"+path {
		t.Fatalf("fields=%#v", fields)
	}
	if fields["file_url_md"] != "[下载 Excel]("+fileURL+")" {
		t.Fatalf("fields=%#v", fields)
	}
}
