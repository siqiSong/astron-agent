package chart

import (
	"fmt"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func Bar(request model.BarChartRequest) ([]byte, model.ChartMapping, error) {
	mapping, err := infer(request.Data, request.CategoryField, request.ValueFields)
	if err != nil {
		return nil, mapping, err
	}
	width, height := dimension(request.Width, 600), dimension(request.Height, 400)
	horizontal := request.Direction == "horizontal"
	var all []float64
	for _, row := range request.Data {
		for _, field := range mapping.ValueFields {
			if value, ok := number(row[field]); ok {
				all = append(all, value)
			}
		}
	}
	scale := maxAbs(all)
	var out strings.Builder
	out.WriteString(svgStart(width, height, request.Title))
	left, top, right, bottom := 70.0, 55.0, float64(width-25), float64(height-55)
	plotW, plotH := right-left, bottom-top
	out.WriteString(fmt.Sprintf(`<path d="M%s %sV%sH%s" fill="none" stroke="#8c8c8c"/>`, f(left), f(top), f(bottom), f(right)))
	groups := float64(len(request.Data))
	series := float64(len(mapping.ValueFields))
	if groups == 0 {
		groups = 1
	}
	for rowIndex, row := range request.Data {
		label := escape(category(row[mapping.CategoryField]))
		for seriesIndex, field := range mapping.ValueFields {
			value, _ := number(row[field])
			color := palette[seriesIndex%len(palette)]
			if horizontal {
				slot := plotH / groups
				barH := slot / (series + 1)
				y := top + float64(rowIndex)*slot + float64(seriesIndex)*barH + 4
				barW := value / scale * plotW * .9
				x := left
				if barW < 0 {
					x += barW
					barW = -barW
				}
				out.WriteString(fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`, f(x), f(y), f(barW), f(barH-2), color))
				if seriesIndex == 0 {
					out.WriteString(fmt.Sprintf(`<text x="%s" y="%s" text-anchor="end" font-family="sans-serif" font-size="11">%s</text>`, f(left-6), f(y+barH*.7), label))
				}
			} else {
				slot := plotW / groups
				barW := slot / (series + 1)
				x := left + float64(rowIndex)*slot + float64(seriesIndex)*barW + 4
				barH := value / scale * plotH * .85
				y := bottom - barH
				if barH < 0 {
					y = bottom
					barH = -barH
				}
				out.WriteString(fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`, f(x), f(y), f(barW-2), f(barH), color))
				if seriesIndex == 0 {
					out.WriteString(fmt.Sprintf(`<text x="%s" y="%s" text-anchor="middle" font-family="sans-serif" font-size="11">%s</text>`, f(left+(float64(rowIndex)+.45)*slot), f(bottom+18), label))
				}
			}
		}
	}
	for index, field := range mapping.ValueFields {
		out.WriteString(fmt.Sprintf(`<rect x="%d" y="38" width="10" height="10" fill="%s"/><text x="%d" y="47" font-family="sans-serif" font-size="11">%s</text>`, 80+index*100, palette[index%len(palette)], 94+index*100, escape(field)))
	}
	closeSVG(&out)
	return []byte(out.String()), mapping, nil
}
