package spreadsheet

import (
	"bytes"
	"testing"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func TestWriteXLSXRoundTrips(t *testing.T) {
	rows := []map[string]any{{"姓名": "张三", "年龄": 20, "在校": true}, {"姓名": "李四", "年龄": 21, "在校": false}}
	var output bytes.Buffer
	summary, err := WriteXLSX(&output, model.ExcelRequest{Data: rows, Columns: []string{"姓名", "年龄", "在校"}, SheetName: "学生/名单", StartCell: "B2", FreezeHeader: true, AutoFilter: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Range != "B2:D4" || summary.SheetName != "学生 名单" {
		t.Fatalf("summary=%+v", summary)
	}
	wb, err := Read(bytes.NewReader(output.Bytes()), int64(output.Len()), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := wb.Sheets[0].Rows[2][1].Text; got != "张三" {
		t.Fatalf("cell=%q", got)
	}
}

func TestWriteXLSXIsDeterministic(t *testing.T) {
	request := model.ExcelRequest{Data: []map[string]any{{"a": 1}}, Columns: []string{"a"}}
	var first, second bytes.Buffer
	WriteXLSX(&first, request)
	WriteXLSX(&second, request)
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("output differs")
	}
}
