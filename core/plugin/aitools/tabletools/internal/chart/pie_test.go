package chart

import (
	"bytes"
	"testing"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func TestPieAggregatesDuplicatesAndRendersDonut(t *testing.T) {
	rows := []map[string]any{{"分类": "A", "值": 2}, {"分类": "A", "值": 3}, {"分类": "B", "值": 5}}
	svg, mapping, err := Pie(model.PieChartRequest{Data: rows, Donut: true, Title: "占比"})
	if err != nil {
		t.Fatal(err)
	}
	if mapping.CategoryField != "分类" || mapping.ValueFields[0] != "值" {
		t.Fatalf("mapping=%+v", mapping)
	}
	if bytes.Count(svg, []byte("<path")) != 2 || !bytes.Contains(svg, []byte("<circle")) {
		t.Fatalf("%s", svg)
	}
}

func TestPieRejectsNegativeAndAllZero(t *testing.T) {
	for _, value := range []float64{-1, 0} {
		_, _, err := Pie(model.PieChartRequest{Data: []map[string]any{{"x": "A", "y": value}}})
		if err == nil {
			t.Fatalf("value=%v", value)
		}
	}
}
