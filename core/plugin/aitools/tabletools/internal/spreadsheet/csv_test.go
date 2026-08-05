package spreadsheet

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadCSVAutoDetectsGBK(t *testing.T) {
	data := []byte{0xD0, 0xD5, 0xC3, 0xFB, ',', 0xC4, 0xEA, 0xC1, 0xE4, '\n', 0xD5, 0xC5, 0xC8, 0xFD, ',', '2', '0', '\n'}
	wb, err := Read(bytes.NewReader(data), int64(len(data)), Options{Encoding: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 1 || wb.Sheets[0].Rows[0][0].Text != "姓名" || wb.Sheets[0].Rows[1][0].Text != "张三" {
		t.Fatalf("workbook = %+v", wb)
	}
}

func TestReadCSVHandlesBOMQuotedNewlineAndRange(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("姓名,备注,值\n张三,\"两行\n内容\",1\n李四,普通,2\n")...)
	wb, err := Read(bytes.NewReader(data), int64(len(data)), Options{Range: "B1:C2"})
	if err != nil {
		t.Fatal(err)
	}
	rows := wb.Sheets[0].Rows
	if len(rows) != 2 || len(rows[0]) != 2 || rows[1][0].Text != "两行\n内容" || rows[1][1].Text != "1" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestReadCSVRejectsUnknownEncodingAndColumnLimit(t *testing.T) {
	data := []byte("a,b,c\n1,2,3\n")
	if _, err := Read(bytes.NewReader(data), int64(len(data)), Options{Encoding: "big5"}); !errors.Is(err, ErrUnsupportedEncoding) {
		t.Fatalf("encoding error = %v", err)
	}
	if _, err := Read(bytes.NewReader(data), int64(len(data)), Options{MaxColumns: 2}); !errors.Is(err, ErrColumnLimit) {
		t.Fatalf("column error = %v", err)
	}
}
