package com.iflytek.astron.console.toolkit.service.tool;

import com.alibaba.fastjson2.JSONArray;
import com.alibaba.fastjson2.JSONObject;
import com.iflytek.astron.console.toolkit.entity.tool.WebSchemaItem;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ToolBoxServiceCompositeInputTest {

    private final ToolBoxService service = new ToolBoxService();

    @Test
    void extractsArrayObjectJsonEnteredByPluginDebugger() {
        WebSchemaItem data = item("data", "array-object", "[{\"部门\":\"法学院\",\"人数\":12}]");

        JSONObject result = service.extractToolRunParams(List.of(data));

        JSONArray rows = result.getJSONArray("data");
        assertThat(rows).hasSize(1);
        assertThat(rows.getJSONObject(0).getString("部门")).isEqualTo("法学院");
        assertThat(rows.getJSONObject(0).getIntValue("人数")).isEqualTo(12);
    }

    @Test
    void extractsArrayStringJsonEnteredByPluginDebugger() {
        WebSchemaItem columns = item("columns", "array-string", "[\"部门\",\"人数\"]");

        JSONObject result = service.extractToolRunParams(List.of(columns));

        assertThat(result.getJSONArray("columns")).containsExactly("部门", "人数");
    }

    private WebSchemaItem item(String name, String type, Object value) {
        WebSchemaItem item = new WebSchemaItem();
        item.setName(name);
        item.setType(type);
        item.setDft(value);
        return item;
    }
}
