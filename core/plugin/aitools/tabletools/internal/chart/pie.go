package chart

import (
	"fmt"
	"math"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func Pie(request model.PieChartRequest) ([]byte, model.ChartMapping, error) {
	values := []string{}
	if request.ValueField != "" {
		values = []string{request.ValueField}
	}
	mapping, err := infer(request.Data, request.CategoryField, values)
	if err != nil {
		return nil, mapping, err
	}
	mapping.ValueFields = mapping.ValueFields[:1]
	aggregated := map[string]float64{}
	total := 0.0
	for _, row := range request.Data {
		value, ok := number(row[mapping.ValueFields[0]])
		if !ok || value < 0 {
			return nil, mapping, ErrInvalidChartData
		}
		aggregated[category(row[mapping.CategoryField])] += value
		total += value
	}
	if total <= 0 {
		return nil, mapping, ErrInvalidChartData
	}
	width, height := dimension(request.Width, 600), dimension(request.Height, 400)
	cx, cy := float64(width)*.38, float64(height)*.56
	radius := math.Min(float64(width), float64(height)) * .3
	keys := sortedCategories(aggregated)
	var out strings.Builder
	out.WriteString(svgStart(width, height, request.Title))
	angle := -math.Pi / 2
	for index, key := range keys {
		value := aggregated[key]
		if value == 0 {
			continue
		}
		next := angle + value/total*2*math.Pi
		out.WriteString(fmt.Sprintf(`<path d="%s" fill="%s"><title>%s: %s</title></path>`, arcPath(cx, cy, radius, angle, next), palette[index%len(palette)], escape(key), escape(percent(value, total))))
		ly := 75 + index*24
		out.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="12" height="12" fill="%s"/><text x="%d" y="%d" font-family="sans-serif" font-size="12">%s %s</text>`, int(float64(width)*.72), ly, palette[index%len(palette)], int(float64(width)*.72)+18, ly+11, escape(key), percent(value, total)))
		angle = next
	}
	if request.Donut {
		out.WriteString(fmt.Sprintf(`<circle cx="%s" cy="%s" r="%s" fill="white"/>`, f(cx), f(cy), f(radius*.52)))
	}
	closeSVG(&out)
	return []byte(out.String()), mapping, nil
}
