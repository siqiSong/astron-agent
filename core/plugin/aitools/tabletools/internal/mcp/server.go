package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
	toolservice "github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/tools"
)

type Server struct {
	service  *toolservice.Service
	sessions *sessionStore
}

func New(service *toolservice.Service) *Server {
	return &Server{service: service, sessions: newSessionStore()}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (server *Server) Dispatch(input []byte) ([]byte, bool) {
	var request rpcRequest
	if err := json.Unmarshal(input, &request); err != nil {
		return marshalRPC(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}}), false
	}
	if request.Method == "notifications/initialized" || request.ID == nil {
		return nil, true
	}
	response := rpcResponse{JSONRPC: "2.0", ID: request.ID}
	switch request.Method {
	case "initialize":
		response.Result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "astron-table-tools", "version": "1.0.0"}}
	case "ping":
		response.Result = map[string]any{}
	case "tools/list":
		response.Result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			response.Error = &rpcError{Code: -32602, Message: "invalid params"}
			break
		}
		result, err := server.call(context.Background(), params.Name, params.Arguments)
		if err != nil {
			response.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": err.Error()}}, "isError": true}
			break
		}
		encoded, _ := json.Marshal(result)
		response.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": string(encoded)}}, "structuredContent": result, "isError": false}
	default:
		response.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return marshalRPC(response), false
}
func (server *Server) call(ctx context.Context, name string, arguments json.RawMessage) (any, error) {
	switch name {
	case "table_extract":
		var req toolservice.ExtractRequest
		if err := decodeArgs(arguments, &req); err != nil {
			return nil, err
		}
		return server.service.Extract(ctx, req)
	case "bar_chart":
		var req model.BarChartRequest
		if err := decodeArgs(arguments, &req); err != nil {
			return nil, err
		}
		return server.service.BarChart(ctx, req)
	case "pie_chart":
		var req model.PieChartRequest
		if err := decodeArgs(arguments, &req); err != nil {
			return nil, err
		}
		return server.service.PieChart(ctx, req)
	case "excel_generate":
		var req toolservice.ExcelRequest
		if err := decodeArgs(arguments, &req); err != nil {
			return nil, err
		}
		return server.service.GenerateExcel(ctx, req)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}
func decodeArgs(data []byte, target any) error {
	if len(data) == 0 {
		data = []byte(`{}`)
	}
	return json.Unmarshal(data, target)
}
func marshalRPC(value rpcResponse) []byte { data, _ := json.Marshal(value); return data }
func toolDefinitions() []map[string]any {
	return []map[string]any{
		{"name": "table_extract", "description": "读取 XLS、XLSX 或 CSV，并同时返回结构化行、JSON 和 Markdown 表格", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"url": map[string]string{"type": "string"}, "excel_url": map[string]string{"type": "string"}, "content_base64": map[string]string{"type": "string"}, "file_name": map[string]string{"type": "string"}, "sheet_name": map[string]string{"type": "string"}, "range": map[string]string{"type": "string"}}}, "outputSchema": extractOutputSchema()},
		{"name": "bar_chart", "description": "根据表格行数据生成柱形图 SVG，并返回图片地址与 Markdown", "inputSchema": chartSchema(false), "outputSchema": chartOutputSchema()},
		{"name": "pie_chart", "description": "根据表格行数据生成饼图或圆环图 SVG，并返回图片地址与 Markdown", "inputSchema": chartSchema(true), "outputSchema": chartOutputSchema()},
		{"name": "excel_generate", "description": "根据结构化行数据生成 XLSX 文件，并返回下载地址与 Markdown", "inputSchema": map[string]any{"type": "object", "required": []string{"data"}, "properties": map[string]any{"data": map[string]string{"type": "array"}, "columns": map[string]string{"type": "array"}, "sheet_name": map[string]string{"type": "string"}, "start_cell": map[string]string{"type": "string"}}}, "outputSchema": excelOutputSchema()}}
}
func extractOutputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"rows", "json", "markdown", "table"},
		"properties": map[string]any{
			"rows":     map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}, "description": "结构化表格行，供后续工具继续处理"},
			"json":     map[string]string{"type": "string", "description": "结构化表格行的 JSON 字符串"},
			"markdown": map[string]string{"type": "string", "description": "可直接输出到对话的 Markdown 表格"},
			"table":    map[string]string{"type": "object", "description": "列定义、行数、工作表和范围等元数据"},
		},
	}
}

func chartOutputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"path", "image_url", "image_url_md", "mapping"},
		"properties": map[string]any{
			"path":         map[string]string{"type": "string", "description": "同源相对路径"},
			"image_url":    map[string]string{"type": "string", "description": "图片下载地址"},
			"image_url_md": map[string]string{"type": "string", "description": "可直接渲染图片的 Markdown"},
			"mapping":      map[string]string{"type": "object", "description": "图表字段映射"},
			"artifact":     map[string]string{"type": "object", "description": "兼容旧调用方的文件信息"},
		},
	}
}

func excelOutputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"path", "file_url", "file_url_md", "summary"},
		"properties": map[string]any{
			"path":        map[string]string{"type": "string", "description": "同源相对路径"},
			"file_url":    map[string]string{"type": "string", "description": "Excel 下载地址"},
			"file_url_md": map[string]string{"type": "string", "description": "可直接输出的 Excel 下载 Markdown"},
			"summary":     map[string]string{"type": "object", "description": "生成行列数、工作表和范围"},
			"artifact":    map[string]string{"type": "object", "description": "兼容旧调用方的文件信息"},
		},
	}
}

func chartSchema(pie bool) map[string]any {
	properties := map[string]any{"data": map[string]string{"type": "array"}, "category_field": map[string]string{"type": "string"}, "title": map[string]string{"type": "string"}, "width": map[string]string{"type": "integer"}, "height": map[string]string{"type": "integer"}}
	if pie {
		properties["value_field"] = map[string]string{"type": "string"}
	} else {
		properties["value_fields"] = map[string]string{"type": "array"}
	}
	return map[string]any{"type": "object", "required": []string{"data"}, "properties": properties}
}
