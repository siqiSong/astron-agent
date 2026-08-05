package cellref

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	MaxRows    = 100_000
	MaxColumns = 1_024
)

var ErrInvalidReference = errors.New("invalid cell reference")

type Position struct {
	Row int
	Col int
}

type Range struct {
	Start Position
	End   Position
}

func ParseRange(value string) (Range, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 1 || len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
		return Range{}, ErrInvalidReference
	}
	start, err := parsePosition(parts[0])
	if err != nil {
		return Range{}, err
	}
	end := start
	if len(parts) == 2 {
		end, err = parsePosition(parts[1])
		if err != nil {
			return Range{}, err
		}
	}
	if end.Row < start.Row || end.Col < start.Col {
		return Range{}, ErrInvalidReference
	}
	return Range{Start: start, End: end}, nil
}

func parsePosition(value string) (Position, error) {
	value = strings.TrimSpace(value)
	letterEnd := 0
	for letterEnd < len(value) && unicode.IsLetter(rune(value[letterEnd])) && value[letterEnd] < unicode.MaxASCII {
		letterEnd++
	}
	if letterEnd == 0 || letterEnd == len(value) {
		return Position{}, ErrInvalidReference
	}
	column := 0
	for _, char := range strings.ToUpper(value[:letterEnd]) {
		if char < 'A' || char > 'Z' {
			return Position{}, ErrInvalidReference
		}
		column = column*26 + int(char-'A'+1)
	}
	row, err := strconv.Atoi(value[letterEnd:])
	if err != nil || row < 1 || row > MaxRows || column < 1 || column > MaxColumns {
		return Position{}, ErrInvalidReference
	}
	return Position{Row: row - 1, Col: column - 1}, nil
}

func (r Range) String() string {
	start := formatPosition(r.Start)
	if r.Start == r.End {
		return start
	}
	return start + ":" + formatPosition(r.End)
}

func formatPosition(position Position) string {
	column := position.Col + 1
	letters := ""
	for column > 0 {
		column--
		letters = string(rune('A'+column%26)) + letters
		column /= 26
	}
	return fmt.Sprintf("%s%d", letters, position.Row+1)
}
