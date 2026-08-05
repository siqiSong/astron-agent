package chart

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

var ErrInvalidChartData = errors.New("invalid chart data")
var palette = []string{"#1677ff", "#ff5f91", "#19c2c9", "#f6bd16", "#7262fd", "#78d3f8", "#9661bc", "#f6903d"}

func escape(value string) string {
	var output bytes.Buffer
	xml.EscapeText(&output, []byte(value))
	return output.String()
}
func dimension(value, fallback int) int {
	if value < 240 {
		return fallback
	}
	if value > 2000 {
		return 2000
	}
	return value
}
func number(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case float32:
		return float64(v), true
	case jsonNumber:
		n, e := strconv.ParseFloat(string(v), 64)
		return n, e == nil
	}
	return 0, false
}

type jsonNumber string

func infer(data []map[string]any, category string, values []string) (model.ChartMapping, error) {
	if len(data) == 0 {
		return model.ChartMapping{}, ErrInvalidChartData
	}
	keys := make([]string, 0, len(data[0]))
	for key := range data[0] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if category == "" {
		for _, key := range keys {
			if _, ok := number(data[0][key]); !ok {
				category = key
				break
			}
		}
	}
	if category == "" {
		category = keys[0]
	}
	if len(values) == 0 {
		for _, key := range keys {
			if key == category {
				continue
			}
			valid := false
			for _, row := range data {
				if _, ok := number(row[key]); ok {
					valid = true
					break
				}
			}
			if valid {
				values = append(values, key)
			}
		}
	}
	if len(values) == 0 {
		return model.ChartMapping{}, ErrInvalidChartData
	}
	return model.ChartMapping{CategoryField: category, ValueFields: values}, nil
}

func svgStart(width, height int, title string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img"><rect width="100%%" height="100%%" fill="white"/><text x="%d" y="30" text-anchor="middle" font-family="sans-serif" font-size="20" font-weight="600">%s</text>`, width, height, width, height, width/2, escape(title))
}
func f(value float64) string { return strconv.FormatFloat(value, 'f', 2, 64) }
func category(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func maxAbs(values []float64) float64 {
	result := 0.0
	for _, v := range values {
		if math.Abs(v) > result {
			result = math.Abs(v)
		}
	}
	if result == 0 {
		return 1
	}
	return result
}
func sortedCategories(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func percent(value, total float64) string {
	return strconv.FormatFloat(value/total*100, 'f', 1, 64) + "%"
}
func point(cx, cy, radius, angle float64) (float64, float64) {
	return cx + radius*math.Cos(angle), cy + radius*math.Sin(angle)
}
func arcPath(cx, cy, radius, start, end float64) string {
	x1, y1 := point(cx, cy, radius, start)
	x2, y2 := point(cx, cy, radius, end)
	large := 0
	if end-start > math.Pi {
		large = 1
	}
	return fmt.Sprintf("M%s %s L%s %s A%s %s 0 %d 1 %s %s Z", f(cx), f(cy), f(x1), f(y1), f(radius), f(radius), large, f(x2), f(y2))
}
func closeSVG(builder *strings.Builder) { builder.WriteString(`</svg>`) }
