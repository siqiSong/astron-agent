package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/artifact"
	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/config"
	toolservice "github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/tools"
)

func testMCP(t *testing.T) *Server {
	store, err := artifact.NewStore(t.TempDir(), "/files", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return New(toolservice.New(config.Default(), store))
}

func TestMCPListsExactlyFourTools(t *testing.T) {
	response, _ := testMCP(t).Dispatch([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	var decoded struct {
		Result struct {
			Tools []any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Result.Tools) != 4 {
		t.Fatalf("response=%s", response)
	}
}

func TestMCPToolDefinitionsDescribePromptSelectableOutputs(t *testing.T) {
	response, _ := testMCP(t).Dispatch([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	var decoded struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response, &decoded); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"table_extract":  {"rows", "json", "markdown", "table"},
		"bar_chart":      {"path", "image_url", "image_url_md", "mapping"},
		"pie_chart":      {"path", "image_url", "image_url_md", "mapping"},
		"excel_generate": {"path", "file_url", "file_url_md", "summary"},
	}
	for _, tool := range decoded.Result.Tools {
		name, _ := tool["name"].(string)
		schema, ok := tool["outputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("tool %s has no outputSchema: %#v", name, tool)
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, field := range want[name] {
			if _, ok := properties[field]; !ok {
				t.Fatalf("tool %s missing output %s: %#v", name, field, properties)
			}
		}
	}
}

func TestMCPInitializeAndToolCall(t *testing.T) {
	server := testMCP(t)
	response, _ := server.Dispatch([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1"}}}`))
	if !json.Valid(response) {
		t.Fatalf("%s", response)
	}
	response, _ = server.Dispatch([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"bar_chart","arguments":{"data":[{"部门":"法学院","人数":12}]}}}`))
	if !json.Valid(response) || !contains(response, `"isError":false`) {
		t.Fatalf("%s", response)
	}
}

func contains(data []byte, value string) bool {
	return string(data) != "" && len(data) >= len(value) && json.Valid(data) && stringContains(string(data), value)
}
func stringContains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
