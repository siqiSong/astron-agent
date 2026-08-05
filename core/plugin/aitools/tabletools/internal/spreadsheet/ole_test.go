package spreadsheet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func putDirectoryName(entry []byte, name string) {
	units := []rune(name)
	for index, value := range units {
		binary.LittleEndian.PutUint16(entry[index*2:], uint16(value))
	}
	binary.LittleEndian.PutUint16(entry[len(units)*2:], 0)
	binary.LittleEndian.PutUint16(entry[64:], uint16((len(units)+1)*2))
}

func wrapWorkbookInCFB(workbook []byte) []byte {
	const sectorSize = 512
	padded := make([]byte, 4096)
	copy(padded, workbook)
	data := make([]byte, sectorSize+10*sectorSize)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	binary.LittleEndian.PutUint16(data[24:], 0x003E)
	binary.LittleEndian.PutUint16(data[26:], 3)
	binary.LittleEndian.PutUint16(data[28:], 0xFFFE)
	binary.LittleEndian.PutUint16(data[30:], 9)
	binary.LittleEndian.PutUint16(data[32:], 6)
	binary.LittleEndian.PutUint32(data[44:], 1)
	binary.LittleEndian.PutUint32(data[48:], 8)
	// Force the regular stream path while keeping the declared stream length exact.
	binary.LittleEndian.PutUint32(data[56:], 1)
	binary.LittleEndian.PutUint32(data[60:], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(data[68:], 0xFFFFFFFE)
	for offset := 76; offset < 512; offset += 4 {
		binary.LittleEndian.PutUint32(data[offset:], 0xFFFFFFFF)
	}
	binary.LittleEndian.PutUint32(data[76:], 9)
	copy(data[512:], padded)
	directory := data[512+8*512 : 512+9*512]
	putDirectoryName(directory, "Root Entry")
	directory[66] = 5
	binary.LittleEndian.PutUint32(directory[116:], 0xFFFFFFFE)
	book := directory[128:256]
	putDirectoryName(book, "Workbook")
	book[66] = 2
	binary.LittleEndian.PutUint32(book[116:], 0)
	binary.LittleEndian.PutUint64(book[120:], uint64(len(workbook)))
	fat := data[512+9*512:]
	for index := range fat {
		fat[index] = 0xFF
	}
	for id := 0; id < 7; id++ {
		binary.LittleEndian.PutUint32(fat[id*4:], uint32(id+1))
	}
	binary.LittleEndian.PutUint32(fat[7*4:], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(fat[8*4:], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(fat[9*4:], 0xFFFFFFFD)
	return data
}

func TestOLERejectsBadSignature(t *testing.T) {
	_, err := extractWorkbookStream(make([]byte, 512), Options{}.defaults())
	if !errors.Is(err, ErrInvalidWorkbook) {
		t.Fatalf("err=%v", err)
	}
}

func TestOLERejectsCyclicFATChain(t *testing.T) {
	data := make([]byte, 512+3*512)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	binary.LittleEndian.PutUint16(data[24:], 0x003E)
	binary.LittleEndian.PutUint16(data[26:], 3)
	binary.LittleEndian.PutUint16(data[28:], 0xFFFE)
	binary.LittleEndian.PutUint16(data[30:], 9)
	binary.LittleEndian.PutUint16(data[32:], 6)
	binary.LittleEndian.PutUint32(data[44:], 1)
	binary.LittleEndian.PutUint32(data[48:], 1) // directory sector
	binary.LittleEndian.PutUint32(data[56:], 4096)
	binary.LittleEndian.PutUint32(data[60:], 0xFFFFFFFE)
	binary.LittleEndian.PutUint32(data[68:], 0xFFFFFFFE)
	for offset := 76; offset < 512; offset += 4 {
		binary.LittleEndian.PutUint32(data[offset:], 0xFFFFFFFF)
	}
	binary.LittleEndian.PutUint32(data[76:], 0) // FAT sector
	// FAT: sector 0 is FAT, directory sector 1 points to itself.
	fat := data[512:1024]
	for offset := range fat {
		fat[offset] = 0xFF
	}
	binary.LittleEndian.PutUint32(fat, 0xFFFFFFFD)
	binary.LittleEndian.PutUint32(fat[4:], 1)
	_, err := extractWorkbookStream(data, Options{}.defaults())
	if !errors.Is(err, ErrInvalidWorkbook) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadXLSFromOLEWorkbookStream(t *testing.T) {
	data := wrapWorkbookInCFB(buildBIFF8Workbook())
	wb, err := Read(bytes.NewReader(data), int64(len(data)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := wb.Sheets[0].Rows[1][0].Text; got != "张三" {
		t.Fatalf("value=%q", got)
	}
}
