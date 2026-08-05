package com.iflytek.astron.console.toolkit.service.workflow;

import com.alibaba.fastjson2.JSON;
import com.alibaba.fastjson2.JSONObject;
import com.iflytek.astron.console.commons.exception.BusinessException;
import com.iflytek.astron.console.toolkit.entity.biz.workflow.BizWorkflowData;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class WorkflowServiceReferenceIntegrityTest {

    private final WorkflowService workflowService = new WorkflowService();

    @Test
    void repairsStaleReferenceFromOnlyDirectPredecessorWithMatchingOutput() {
        BizWorkflowData data = workflowData(
                "[{\"id\":\"agent::current\",\"data\":{\"outputs\":[{\"id\":\"new-output-id\",\"name\":\"output\"}]}},"
                        + "{\"id\":\"node-end::end\",\"data\":{\"inputs\":[{\"name\":\"output\",\"schema\":{\"value\":{\"type\":\"ref\",\"content\":{\"nodeId\":\"agent::deleted\",\"id\":\"old-output-id\",\"name\":\"output\"}}}}]}}]",
                "[{\"source\":\"agent::current\",\"target\":\"node-end::end\"}]");

        ReflectionTestUtils.invokeMethod(workflowService, "validateAndRepairReferences", data);

        JSONObject content = (JSONObject) data.getNodes().get(1).getData().getInputs().get(0)
                .getSchema().getValue().getContent();
        assertThat(content.getString("nodeId")).isEqualTo("agent::current");
        assertThat(content.getString("id")).isEqualTo("new-output-id");
        assertThat(content.getString("name")).isEqualTo("output");
    }

    @Test
    void rejectsStaleReferenceWhenRepairWouldBeAmbiguous() {
        BizWorkflowData data = workflowData(
                "[{\"id\":\"agent::one\",\"data\":{\"outputs\":[{\"id\":\"one-output\",\"name\":\"output\"}]}},"
                        + "{\"id\":\"agent::two\",\"data\":{\"outputs\":[{\"id\":\"two-output\",\"name\":\"output\"}]}},"
                        + "{\"id\":\"node-end::end\",\"data\":{\"inputs\":[{\"name\":\"output\",\"schema\":{\"value\":{\"type\":\"ref\",\"content\":{\"nodeId\":\"agent::deleted\",\"name\":\"output\"}}}}]}}]",
                "[{\"source\":\"agent::one\",\"target\":\"node-end::end\"},{\"source\":\"agent::two\",\"target\":\"node-end::end\"}]");

        assertThatThrownBy(() -> ReflectionTestUtils.invokeMethod(
                workflowService, "validateAndRepairReferences", data))
                .isInstanceOf(BusinessException.class)
                .satisfies(error -> assertThat(((BusinessException) error).getArgs()[0])
                        .contains("node-end::end")
                        .contains("reconnect"));
    }

    private BizWorkflowData workflowData(String nodes, String edges) {
        return JSON.parseObject("{\"nodes\":" + nodes + ",\"edges\":" + edges + "}", BizWorkflowData.class);
    }
}
