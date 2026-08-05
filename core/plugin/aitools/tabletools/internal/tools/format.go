package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iflytek/astron-agent/core/plugin/aitools/tabletools/internal/model"
)

func formatTable(table model.Table) (string, string, error) {
	encoded, err := json.Marshal(table.Rows)
	if err != nil {
		return "", "", err
	}
	if len(table.Columns) == 0 {
		return string(encoded), "", nil
	}

	var markdown strings.Builder
	markdown.WriteString("| ")
	for index, column := range table.Columns {
		if index > 0 {
			markdown.WriteString(" | ")
		}
		markdown.WriteString(escapeMarkdownCell(column.Name))
	}
	markdown.WriteString(" |\n| ")
	for index := range table.Columns {
		if index > 0 {
			markdown.WriteString(" | ")
		}
		markdown.WriteString("---")
	}
	markdown.WriteString(" |")
	for _, row := range table.Rows {
		markdown.WriteString("\n| ")
		for index, column := range table.Columns {
			if index > 0 {
				markdown.WriteString(" | ")
			}
			markdown.WriteString(escapeMarkdownCell(valueText(row[column.Name])))
		}
		markdown.WriteString(" |")
	}
	return string(encoded), markdown.String(), nil
}

func valueText(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func escapeMarkdownCell(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\r", "<br>")
	return strings.ReplaceAll(value, "\n", "<br>")
}
