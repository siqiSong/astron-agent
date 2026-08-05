package encoding

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidGBK = errors.New("invalid GBK sequence")

var gbkRunes = []rune(gbkMapping)

func DecodeGBK(input []byte) (string, error) {
	var output strings.Builder
	output.Grow(len(input))
	for index := 0; index < len(input); {
		lead := input[index]
		if lead < utf8.RuneSelf {
			output.WriteByte(lead)
			index++
			continue
		}
		if lead < 0x81 || lead > 0xFE || index+1 >= len(input) {
			return "", ErrInvalidGBK
		}
		trail := input[index+1]
		trailIndex := -1
		switch {
		case trail >= 0x40 && trail <= 0x7E:
			trailIndex = int(trail - 0x40)
		case trail >= 0x80 && trail <= 0xFE:
			trailIndex = int(trail - 0x41)
		}
		if trailIndex < 0 {
			return "", ErrInvalidGBK
		}
		mappingIndex := int(lead-0x81)*190 + trailIndex
		if mappingIndex >= len(gbkRunes) || gbkRunes[mappingIndex] == utf8.RuneError {
			return "", ErrInvalidGBK
		}
		output.WriteRune(gbkRunes[mappingIndex])
		index += 2
	}
	return output.String(), nil
}
