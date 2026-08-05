package chart

import (
	"bytes"
	"testing"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func chartRows() []map[string]any {
	return []map[string]any{{"部门": "法学院", "人数": 12, "预算": 2.5}, {"部门": "商学院", "人数": 8, "预算": 3.0}}
}

func TestBarInfersFieldsEscapesAndSupportsHorizontal(t *testing.T) {
	svg, mapping, err := Bar(model.BarChartRequest{Data: chartRows(), Title: `A&B <统计>`, Direction: "horizontal"})
	if err != nil {
		t.Fatal(err)
	}
	if mapping.CategoryField != "部门" || len(mapping.ValueFields) != 2 {
		t.Fatalf("mapping=%+v", mapping)
	}
	if !bytes.Contains(svg, []byte("A&amp;B &lt;统计&gt;")) || bytes.Contains(svg, []byte("<统计>")) {
		t.Fatalf("%s", svg)
	}
	if !bytes.Contains(svg, []byte("<rect")) {
		t.Fatalf("missing bars: %s", svg)
	}
}

func TestBarRejectsMissingNumericSeries(t *testing.T) {
	_, _, err := Bar(model.BarChartRequest{Data: []map[string]any{{"name": "x"}}})
	if err == nil {
		t.Fatal("expected error")
	}
}
