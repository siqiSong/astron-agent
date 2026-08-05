package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/config"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func TestHTTPBarChartEnvelope(t *testing.T) {
	cfg := config.Default()
	cfg.ArtifactDir = t.TempDir()
	handler := NewServer(cfg)
	request := httptest.NewRequest(http.MethodPost, "/aitools/v1/bar_chart", strings.NewReader(`{"data":[{"部门":"法学院","人数":12}]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var envelope model.Envelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 200 || envelope.Code != 0 {
		t.Fatalf("status=%d envelope=%+v", recorder.Code, envelope)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok || data["image_url"] == nil || data["image_url_md"] == nil || data["path"] == nil {
		t.Fatalf("data=%#v", envelope.Data)
	}
}

func TestHTTPExtractEnvelopeOffersRowsJSONAndMarkdown(t *testing.T) {
	cfg := config.Default()
	cfg.ArtifactDir = t.TempDir()
	handler := NewServer(cfg)
	content := base64.StdEncoding.EncodeToString([]byte("姓名,年龄\n张三,20\n"))
	body := `{"content_base64":"` + content + `","file_name":"a.csv"}`
	request := httptest.NewRequest(http.MethodPost, "/aitools/v1/table_extract", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var envelope model.Envelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	data, ok := envelope.Data.(map[string]any)
	if recorder.Code != 200 || !ok || data["rows"] == nil || data["json"] == nil || data["markdown"] == nil {
		t.Fatalf("status=%d data=%#v", recorder.Code, envelope.Data)
	}
}

func TestHTTPExcelEnvelopeOffersDownloadRepresentations(t *testing.T) {
	cfg := config.Default()
	cfg.ArtifactDir = t.TempDir()
	handler := NewServer(cfg)
	request := httptest.NewRequest(http.MethodPost, "/aitools/v1/excel_generate", strings.NewReader(`{"data":[{"姓名":"张三"}],"columns":["姓名"]}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	var envelope model.Envelope
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	data, ok := envelope.Data.(map[string]any)
	if recorder.Code != 200 || !ok || data["file_url"] == nil || data["file_url_md"] == nil || data["path"] == nil {
		t.Fatalf("status=%d data=%#v", recorder.Code, envelope.Data)
	}
}

func TestHTTPRejectsUnknownJSONField(t *testing.T) {
	cfg := config.Default()
	cfg.ArtifactDir = t.TempDir()
	handler := NewServer(cfg)
	request := httptest.NewRequest(http.MethodPost, "/aitools/v1/pie_chart", strings.NewReader(`{"data":[],"surprise":true}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
