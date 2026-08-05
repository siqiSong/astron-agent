package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/artifact"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/config"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/mcp"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
	toolservice "github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/tools"
)

func NewServer(cfg config.Config) http.Handler {
	store, err := artifact.NewStore(cfg.ArtifactDir, cfg.PublicBaseURL, cfg.ArtifactTTL)
	if err != nil {
		panic(err)
	}
	service := toolservice.New(cfg, store)
	mcpServer := mcp.New(service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})
	handleJSON(mux, "POST /aitools/v1/table_extract", func(r *http.Request) (any, error) {
		var request toolservice.ExtractRequest
		if err := decodeJSON(r, &request); err != nil {
			return nil, err
		}
		return service.Extract(r.Context(), request)
	})
	handleJSON(mux, "POST /aitools/v1/bar_chart", func(r *http.Request) (any, error) {
		var request model.BarChartRequest
		if err := decodeJSON(r, &request); err != nil {
			return nil, err
		}
		return service.BarChart(r.Context(), request)
	})
	handleJSON(mux, "POST /aitools/v1/pie_chart", func(r *http.Request) (any, error) {
		var request model.PieChartRequest
		if err := decodeJSON(r, &request); err != nil {
			return nil, err
		}
		return service.PieChart(r.Context(), request)
	})
	handleJSON(mux, "POST /aitools/v1/excel_generate", func(r *http.Request) (any, error) {
		var request toolservice.ExcelRequest
		if err := decodeJSON(r, &request); err != nil {
			return nil, err
		}
		return service.GenerateExcel(r.Context(), request)
	})
	mux.HandleFunc("GET /aitools-files/icons/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, ok := artifact.Icons[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("GET /aitools-files/{name}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("name")
		file, err := store.Open(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", toolservice.ContentType(id))
		if len(id) > 5 && id[len(id)-5:] == ".xlsx" {
			w.Header().Set("Content-Disposition", `attachment; filename="table.xlsx"`)
		}
		io.Copy(w, file)
	})
	mux.HandleFunc("POST /mcp", mcpServer.HandleStreamable)
	mux.HandleFunc("GET /mcp/sse", mcpServer.HandleSSE)
	mux.HandleFunc("POST /mcp/messages", mcpServer.HandleMessages)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(model.Envelope{Code: 40400, Message: "not found", Data: map[string]any{}})
	})
	return mux
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func handleJSON(mux *http.ServeMux, pattern string, handler func(*http.Request) (any, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		sidBytes := make([]byte, 8)
		rand.Read(sidBytes)
		sid := hex.EncodeToString(sidBytes)
		data, err := handler(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(model.Envelope{Code: 40001, Message: err.Error(), SID: sid, Data: map[string]any{}})
			return
		}
		json.NewEncoder(w).Encode(model.Envelope{Code: 0, Message: "success", SID: sid, Data: data})
	})
}
