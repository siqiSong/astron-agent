const WORKFLOW_BOT_VERSION = 3;
const TALK_BOT_VERSION = 4;

/**
 * Re-answer currently runs only through the standalone-agent runtime.
 * Workflow and talk bots must not expose that action until their runtime
 * provides an equivalent regeneration operation.
 */
export const supportsReAnswer = (botVersion: number): boolean =>
  botVersion !== WORKFLOW_BOT_VERSION && botVersion !== TALK_BOT_VERSION;
