import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { supportsReAnswer } from '../src/pages/chat-page/re-answer-policy.ts';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

test('workflow re-answer stays hidden for draft, named, and published routes', () => {
  for (const routeVersion of ['debugger', 'release-1', undefined]) {
    assert.equal(
      supportsReAnswer(3),
      false,
      `workflow action should be hidden for ${routeVersion ?? 'published'} route`
    );
  }
});

test('talk re-answer stays hidden and ordinary agent re-answer remains available', () => {
  assert.equal(supportsReAnswer(4), false);
  assert.equal(supportsReAnswer(1), true);
});

test('response controls gate the re-answer action through the shared policy', () => {
  const source = readFileSync(
    resolve(
      frontendRoot,
      'src/pages/chat-page/components/resq-bottom-buttons.tsx'
    ),
    'utf8'
  );

  assert.match(source, /supportsReAnswer\(botInfo\.version\)/);
  assert.match(source, /isLastMessage && supportsReAnswer/);
});
