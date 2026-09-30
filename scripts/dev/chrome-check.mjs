// Loads a page in headless Chrome and waits until an element has the expected
// text, optionally saving a screenshot. Used by scripts/dev/smoke-web.sh.
//   node scripts/dev/chrome-check.mjs <url> <selector> <text> [screenshot.png]
// Talks to Chrome over the DevTools protocol (Node's built-in WebSocket), because
// `--dump-dom` returns on the load event, before DDC has run main().
import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as sleep } from 'node:timers/promises';

const [url, selector, text, screenshot] = process.argv.slice(2);
if (!text) {
  console.error('usage: chrome-check.mjs <url> <selector> <text> [screenshot.png]');
  process.exit(2);
}
const deadline = Date.now() + 60_000;
const profile = mkdtempSync(join(tmpdir(), 'chrome-check-'));
const chrome = spawn(process.env.CHROME_EXECUTABLE ?? 'google-chrome', [
  '--headless', '--disable-gpu', '--remote-debugging-port=0', '--window-size=1024,640',
  `--user-data-dir=${profile}`, 'about:blank',
], { stdio: 'ignore' });

let exitCode = 1;
try {
  // Chrome writes its chosen port to DevToolsActivePort once it's listening.
  let port;
  while (!port) {
    if (Date.now() > deadline) throw new Error('Chrome did not start');
    try { port = readFileSync(join(profile, 'DevToolsActivePort'), 'utf8').split('\n')[0]; } catch { await sleep(100); }
  }
  const [page] = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json())
    .filter((t) => t.type === 'page');
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
  let nextId = 0;
  const pending = new Map();
  ws.onmessage = ({ data }) => {
    const msg = JSON.parse(data);
    pending.get(msg.id)?.(msg);
    pending.delete(msg.id);
  };
  const send = (method, params = {}) => new Promise((resolve) => {
    const id = ++nextId;
    pending.set(id, resolve);
    ws.send(JSON.stringify({ id, method, params }));
  });

  await send('Page.navigate', { url });
  const expression = `document.querySelector(${JSON.stringify(selector)})?.textContent ?? null`;
  let seen = null;
  while (Date.now() < deadline) {
    seen = (await send('Runtime.evaluate', { expression, returnByValue: true })).result?.result?.value ?? null;
    if (seen === text) break;
    await sleep(250);
  }
  if (seen !== text) throw new Error(`${selector} is ${JSON.stringify(seen)}, want ${JSON.stringify(text)}`);

  if (screenshot) {
    const shot = await send('Page.captureScreenshot', { format: 'png' });
    writeFileSync(screenshot, Buffer.from(shot.result.data, 'base64'));
  }
  ws.close();
  exitCode = 0;
} catch (err) {
  console.error(`chrome-check: ${err.message}`);
} finally {
  chrome.kill();
  await sleep(200);
  rmSync(profile, { recursive: true, force: true });
}
process.exit(exitCode);
