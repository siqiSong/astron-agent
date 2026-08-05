package spreadsheet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func biffRecord(id uint16, payload []byte) []byte {
	result := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(result, id)
	binary.LittleEndian.PutUint16(result[2:], uint16(len(payload)))
	copy(result[4:], payload)
	return result
}

func biffBOF(kind uint16) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload, 0x0600)
	binary.LittleEndian.PutUint16(payload[2:], kind)
	return biffRecord(0x0809, payload)
}

func biff8String(value string) []byte {
	runes := []rune(value)
	payload := make([]byte, 3+len(runes)*2)
	binary.LittleEndian.PutUint16(payload, uint16(len(runes)))
	payload[2] = 1
	for index, r := range runes {
		binary.LittleEndian.PutUint16(payload[3+index*2:], uint16(r))
	}
	return payload
}

func buildBIFF8Workbook() []byte {
	globals := bytes.NewBuffer(nil)
	globals.Write(biffBOF(0x0005))
	globals.Write(biffRecord(0x0042, []byte{0xA4, 0x03})) // UTF-8 codepage marker.
	sst := make([]byte, 8)
	binary.LittleEndian.PutUint32(sst, 1)
	binary.LittleEndian.PutUint32(sst[4:], 1)
	sst = append(sst, biff8String("张三")...)
	globals.Write(biffRecord(0x00FC, sst))

	bound1 := append(make([]byte, 6), byte(len([]rune("数据"))), 1)
	for _, r := range []rune("数据") {
		bound1 = binary.LittleEndian.AppendUint16(bound1, uint16(r))
	}
	bound2 := append(make([]byte, 6), byte(len([]rune("第二页"))), 1)
	for _, r := range []rune("第二页") {
		bound2 = binary.LittleEndian.AppendUint16(bound2, uint16(r))
	}
	globals.Write(biffRecord(0x0085, bound1))
	globals.Write(biffRecord(0x0085, bound2))
	globals.Write(biffRecord(0x000A, nil))

	sheet1 := bytes.NewBuffer(nil)
	sheet1.Write(biffBOF(0x0010))
	label := make([]byte, 10)
	binary.LittleEndian.PutUint16(label, 1)
	binary.LittleEndian.PutUint32(label[6:], 0)
	sheet1.Write(biffRecord(0x00FD, label))
	number := make([]byte, 14)
	binary.LittleEndian.PutUint16(number, 1)
	binary.LittleEndian.PutUint16(number[2:], 1)
	binary.LittleEndian.PutUint64(number[6:], math.Float64bits(20.5))
	sheet1.Write(biffRecord(0x0203, number))
	sheet1.Write(biffRecord(0x000A, nil))

	sheet2 := bytes.NewBuffer(nil)
	sheet2.Write(biffBOF(0x0010))
	boolean := make([]byte, 8)
	boolean[6] = 1
	sheet2.Write(biffRecord(0x0205, boolean))
	sheet2.Write(biffRecord(0x000A, nil))

	workbook := globals.Bytes()
	offset1 := uint32(len(workbook))
	offset2 := offset1 + uint32(sheet1.Len())
	// Patch BOUNDSHEET offsets in the two global records.
	first := bytes.Index(workbook, []byte{0x85, 0x00})
	binary.LittleEndian.PutUint32(workbook[first+4:], offset1)
	secondRel := bytes.Index(workbook[first+8:], []byte{0x85, 0x00})
	second := first + 8 + secondRel
	binary.LittleEndian.PutUint32(workbook[second+4:], offset2)
	return append(append(workbook, sheet1.Bytes()...), sheet2.Bytes()...)
}

func TestReadBIFF8SharedStringsNumbersAndSheets(t *testing.T) {
	wb, err := parseBIFF(buildBIFF8Workbook(), Options{}.defaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 || wb.Sheets[0].Name != "数据" || wb.Sheets[1].Name != "第二页" {
		t.Fatalf("sheets=%+v", wb.Sheets)
	}
	if got := wb.Sheets[0].Rows[1][0].Text; got != "张三" {
		t.Fatalf("shared string=%q", got)
	}
	if got := wb.Sheets[0].Rows[1][1].Value; got != 20.5 {
		t.Fatalf("number=%v", got)
	}
	if got := wb.Sheets[1].Rows[0][0].Value; got != true {
		t.Fatalf("boolean=%v", got)
	}
}

func TestEncryptedWorkbookIsExplicit(t *testing.T) {
	data := append(biffBOF(0x0005), biffRecord(0x002F, []byte{1, 0})...)
	_, err := parseBIFF(data, Options{}.defaults())
	if !errors.Is(err, ErrEncryptedWorkbook) {
		t.Fatalf("err=%v", err)
	}
}

func TestBIFFRejectsTruncatedRecord(t *testing.T) {
	_, err := parseBIFF([]byte{0x09, 0x08, 0x10, 0x00, 1}, Options{}.defaults())
	if !errors.Is(err, ErrInvalidWorkbook) {
		t.Fatalf("err=%v", err)
	}
}

func buildBIFF5Workbook() []byte {
	globals := bytes.NewBuffer(nil)
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload, 0x0500)
	binary.LittleEndian.PutUint16(payload[2:], 0x0005)
	globals.Write(biffRecord(0x0809, payload))
	globals.Write(biffRecord(0x0042, []byte{0xA8, 0x03})) // CP936
	bound := append(make([]byte, 6), byte(len("Sheet1")))
	bound = append(bound, "Sheet1"...)
	globals.Write(biffRecord(0x0085, bound))
	globals.Write(biffRecord(0x000A, nil))
	sheet := bytes.NewBuffer(nil)
	binary.LittleEndian.PutUint16(payload, 0x0500)
	binary.LittleEndian.PutUint16(payload[2:], 0x0010)
	sheet.Write(biffRecord(0x0809, payload))
	label := make([]byte, 7)
	label = append(label, 0xD5, 0xC5, 0xC8, 0xFD) // 张三 in GBK
	label[6] = 4
	sheet.Write(biffRecord(0x0204, label))
	sheet.Write(biffRecord(0x000A, nil))
	data := globals.Bytes()
	index := bytes.Index(data, []byte{0x85, 0x00})
	binary.LittleEndian.PutUint32(data[index+4:], uint32(len(data)))
	return append(data, sheet.Bytes()...)
}

func TestReadBIFF5GBKLabel(t *testing.T) {
	wb, err := parseBIFF(buildBIFF5Workbook(), Options{}.defaults())
	if err != nil {
		t.Fatal(err)
	}
	if got := wb.Sheets[0].Rows[0][0].Text; got != "张三" {
		t.Fatalf("label=%q", got)
	}
}

func TestBIFFDateStyleAndFormulaCache(t *testing.T) {
	data := buildBIFF8Workbook()
	// Insert an XF using built-in date format 14 before the globals EOF.
	xf := make([]byte, 4)
	binary.LittleEndian.PutUint16(xf[2:], 14)
	eof := bytes.Index(data, []byte{0x0A, 0x00, 0x00, 0x00})
	insert := biffRecord(0x00E0, xf)
	data = append(data[:eof], append(insert, data[eof:]...)...)
	// Adjust both worksheet offsets after insertion.
	first := bytes.Index(data, []byte{0x85, 0x00})
	secondRel := bytes.Index(data[first+8:], []byte{0x85, 0x00})
	second := first + 8 + secondRel
	binary.LittleEndian.PutUint32(data[first+4:], binary.LittleEndian.Uint32(data[first+4:])+uint32(len(insert)))
	binary.LittleEndian.PutUint32(data[second+4:], binary.LittleEndian.Uint32(data[second+4:])+uint32(len(insert)))
	firstSheet := int(binary.LittleEndian.Uint32(data[first+4:]))
	formula := make([]byte, 14)
	binary.LittleEndian.PutUint16(formula, 2)
	binary.LittleEndian.PutUint64(formula[6:], math.Float64bits(46239))
	formulaRecord := biffRecord(0x0006, formula)
	sheetEOF := firstSheet + bytes.Index(data[firstSheet:], []byte{0x0A, 0x00, 0x00, 0x00})
	data = append(data[:sheetEOF], append(formulaRecord, data[sheetEOF:]...)...)
	binary.LittleEndian.PutUint32(data[second+4:], binary.LittleEndian.Uint32(data[second+4:])+uint32(len(formulaRecord)))
	wb, err := parseBIFF(data, Options{}.defaults())
	if err != nil {
		t.Fatal(err)
	}
	if got := wb.Sheets[0].Rows[2][0].Text; got != "2026-08-05" {
		t.Fatalf("date formula=%q", got)
	}
}
