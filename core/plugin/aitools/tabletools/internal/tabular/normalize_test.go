package tabular

import (
	"testing"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func textRows(values ...[]string) [][]model.Cell {
	rows := make([][]model.Cell, len(values))
	for rowIndex, row := range values {
		rows[rowIndex] = make([]model.Cell, len(row))
		for colIndex, value := range row {
			rows[rowIndex][colIndex] = model.Cell{Text: value}
		}
	}
	return rows
}

func TestNormalizeDuplicateHeadersAndTypes(t *testing.T) {
	rows := textRows(
		[]string{"姓名", "金额", "金额", "有效", "日期"},
		[]string{"张三", "12", "3.5", "true", "2026-08-05"},
		[]string{"李四", "8", "4", "false", "2026-08-06"},
	)

	got, err := Normalize(rows, Options{HeaderRow: 0, MaxRows: 100})
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"姓名", "金额", "金额_2", "有效", "日期"}
	wantTypes := []string{"string", "integer", "number", "boolean", "date"}
	for index := range wantNames {
		if got.Columns[index].Name != wantNames[index] || got.Columns[index].Type != wantTypes[index] {
			t.Fatalf("columns = %+v", got.Columns)
		}
	}
	if got.Rows[0]["金额"] != int64(12) || got.Rows[0]["金额_2"] != 3.5 || got.Rows[0]["有效"] != true {
		t.Fatalf("typed row = %#v", got.Rows[0])
	}
}

func TestNormalizeCreatesNamesForBlankHeadersAndTruncates(t *testing.T) {
	rows := textRows(
		[]string{"", ""},
		[]string{"a", "1"},
		[]string{"b", "2"},
		[]string{"c", "3"},
	)
	got, err := Normalize(rows, Options{HeaderRow: 0, MaxRows: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Columns[0].Name != "column_1" || got.Columns[1].Name != "column_2" {
		t.Fatalf("columns = %+v", got.Columns)
	}
	if got.RowCount != 2 || got.TotalRowCount != 3 || !got.Truncated {
		t.Fatalf("counts = %+v", got)
	}
}

func TestNormalizeRejectsInvalidHeaderRow(t *testing.T) {
	_, err := Normalize(textRows([]string{"a"}), Options{HeaderRow: 2})
	if err == nil {
		t.Fatal("expected invalid header row error")
	}
}
