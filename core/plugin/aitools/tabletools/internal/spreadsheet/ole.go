package spreadsheet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

const (
	cfbFree       = uint32(0xFFFFFFFF)
	cfbEnd        = uint32(0xFFFFFFFE)
	cfbFAT        = uint32(0xFFFFFFFD)
	cfbDIF        = uint32(0xFFFFFFFC)
	maxCFBSectors = 1_000_000
)

type cfbDirectory struct {
	name   string
	typeID byte
	start  uint32
	size   uint64
}

func extractWorkbookStream(data []byte, options Options) ([]byte, error) {
	if len(data) < 512 || !bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return nil, ErrInvalidWorkbook
	}
	if binary.LittleEndian.Uint16(data[28:]) != 0xFFFE {
		return nil, ErrInvalidWorkbook
	}
	version := binary.LittleEndian.Uint16(data[26:])
	sectorShift := binary.LittleEndian.Uint16(data[30:])
	miniShift := binary.LittleEndian.Uint16(data[32:])
	if (version != 3 && version != 4) || (sectorShift != 9 && sectorShift != 12) || miniShift != 6 {
		return nil, ErrInvalidWorkbook
	}
	sectorSize, miniSize := 1<<sectorShift, 1<<miniShift
	sectorCount := (len(data) - sectorSize) / sectorSize
	if len(data) < sectorSize || sectorCount < 1 || sectorCount > maxCFBSectors {
		return nil, ErrInvalidWorkbook
	}
	sector := func(id uint32) ([]byte, error) {
		if id >= uint32(sectorCount) {
			return nil, ErrInvalidWorkbook
		}
		start := sectorSize + int(id)*sectorSize
		return data[start : start+sectorSize], nil
	}

	numFAT := binary.LittleEndian.Uint32(data[44:])
	if numFAT == 0 || numFAT > uint32(sectorCount) {
		return nil, ErrInvalidWorkbook
	}
	difat := make([]uint32, 0, numFAT)
	for offset := 76; offset+4 <= 512 && len(difat) < int(numFAT); offset += 4 {
		id := binary.LittleEndian.Uint32(data[offset:])
		if id != cfbFree {
			difat = append(difat, id)
		}
	}
	nextDIF := binary.LittleEndian.Uint32(data[68:])
	numDIF := binary.LittleEndian.Uint32(data[72:])
	seenDIF := map[uint32]bool{}
	for count := uint32(0); count < numDIF && nextDIF != cfbEnd; count++ {
		if seenDIF[nextDIF] {
			return nil, ErrInvalidWorkbook
		}
		seenDIF[nextDIF] = true
		block, err := sector(nextDIF)
		if err != nil {
			return nil, err
		}
		for offset := 0; offset < sectorSize-4 && len(difat) < int(numFAT); offset += 4 {
			id := binary.LittleEndian.Uint32(block[offset:])
			if id != cfbFree {
				difat = append(difat, id)
			}
		}
		nextDIF = binary.LittleEndian.Uint32(block[sectorSize-4:])
	}
	if len(difat) != int(numFAT) {
		return nil, ErrInvalidWorkbook
	}
	fat := make([]uint32, 0, len(difat)*sectorSize/4)
	for _, id := range difat {
		block, err := sector(id)
		if err != nil {
			return nil, err
		}
		for offset := 0; offset < sectorSize; offset += 4 {
			fat = append(fat, binary.LittleEndian.Uint32(block[offset:]))
		}
	}
	readChain := func(start uint32, table []uint32, load func(uint32) ([]byte, error), limit int64) ([]byte, error) {
		var output []byte
		seen := map[uint32]bool{}
		for id := start; id != cfbEnd; {
			if id == cfbFree || id == cfbFAT || id == cfbDIF || int(id) >= len(table) || seen[id] {
				return nil, ErrInvalidWorkbook
			}
			seen[id] = true
			block, err := load(id)
			if err != nil {
				return nil, err
			}
			if int64(len(output))+int64(len(block)) > limit {
				return nil, ErrExpandedLimit
			}
			output = append(output, block...)
			id = table[id]
		}
		return output, nil
	}
	directoryBytes, err := readChain(binary.LittleEndian.Uint32(data[48:]), fat, sector, options.MaxExpandedBytes)
	if err != nil {
		return nil, err
	}
	entries := make([]cfbDirectory, 0, len(directoryBytes)/128)
	for offset := 0; offset+128 <= len(directoryBytes); offset += 128 {
		entry := directoryBytes[offset : offset+128]
		nameLength := int(binary.LittleEndian.Uint16(entry[64:]))
		if nameLength < 2 || nameLength > 64 || nameLength%2 != 0 {
			continue
		}
		units := make([]uint16, 0, nameLength/2-1)
		for pos := 0; pos < nameLength-2; pos += 2 {
			units = append(units, binary.LittleEndian.Uint16(entry[pos:]))
		}
		size := binary.LittleEndian.Uint64(entry[120:])
		if version == 3 {
			size &= 0xFFFFFFFF
		}
		entries = append(entries, cfbDirectory{name: string(utf16.Decode(units)), typeID: entry[66], start: binary.LittleEndian.Uint32(entry[116:]), size: size})
	}
	var root, workbook *cfbDirectory
	for index := range entries {
		entry := &entries[index]
		if entry.typeID == 5 {
			root = entry
		}
		if entry.typeID == 2 && (entry.name == "Workbook" || entry.name == "Book") {
			workbook = entry
		}
	}
	if root == nil || workbook == nil || workbook.size > uint64(options.MaxExpandedBytes) {
		return nil, ErrInvalidWorkbook
	}
	cutoff := binary.LittleEndian.Uint32(data[56:])
	var stream []byte
	if workbook.size >= uint64(cutoff) {
		stream, err = readChain(workbook.start, fat, sector, options.MaxExpandedBytes)
	} else {
		miniFATBytes, miniErr := readChain(binary.LittleEndian.Uint32(data[60:]), fat, sector, options.MaxExpandedBytes)
		if miniErr != nil {
			return nil, miniErr
		}
		miniFAT := make([]uint32, len(miniFATBytes)/4)
		for index := range miniFAT {
			miniFAT[index] = binary.LittleEndian.Uint32(miniFATBytes[index*4:])
		}
		miniStream, miniErr := readChain(root.start, fat, sector, options.MaxExpandedBytes)
		if miniErr != nil {
			return nil, miniErr
		}
		loadMini := func(id uint32) ([]byte, error) {
			start := int(id) * miniSize
			if start < 0 || start+miniSize > len(miniStream) {
				return nil, ErrInvalidWorkbook
			}
			return miniStream[start : start+miniSize], nil
		}
		stream, err = readChain(workbook.start, miniFAT, loadMini, options.MaxExpandedBytes)
	}
	if err != nil || uint64(len(stream)) < workbook.size {
		return nil, fmt.Errorf("%w: workbook stream", ErrInvalidWorkbook)
	}
	return stream[:workbook.size], nil
}
