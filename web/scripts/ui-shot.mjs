// Dev-only UI screenshot harness. It uses the installed Edge/Chrome through
// playwright-core; no browser download and no production dependency.
//
//   npm run shots
//   npm run shots -- showcase wizard cannon
//
// Requires the server on :8080 (make backend / make build + bin/wizard-server).
// Outputs to web/screenshots/<fixture>/.

import { chromium } from 'playwright-core';
import { access, mkdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const base = process.env.BASE_URL ?? 'http://localhost:8080';
const outRoot = path.join(root, 'web', 'screenshots');
const CANVAS_SIZE = 72 * 8 + 46 * 2;
const CELL = 72;
const MARGIN = 46;

const browserCandidates = [
  process.env.EDGE_PATH,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
].filter(Boolean);

async function findBrowser() {
  for (const candidate of browserCandidates) {
    try {
      await access(candidate);
      return candidate;
    } catch {
      // try the next one
    }
  }
  throw new Error(
    'No Edge/Chrome found. Set EDGE_PATH to msedge.exe (or install playwright browsers).',
  );
}

async function loadSave(relativePath) {
  // Do not parse/stringify: the save has uint64 fields (stateHash, seed) that
  // JavaScript numbers cannot represent exactly.
  const raw = (await readFile(path.join(root, relativePath), 'utf8')).replace(
    /"ai":\s*\[[^\]]*\]/,
    '"ai": [false, false]',
  );
  const res = await fetch(`${base}/api/saves/load`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: raw,
  });
  if (!res.ok) throw new Error(`load ${relativePath}: ${res.status} ${await res.text()}`);
  return res.json();
}

function piece(view, action) {
  return view.state.pieces.find((p) => p.id === action.unit);
}

async function play(view, pred) {
  const action = view.legal.find(pred);
  if (!action) throw new Error(`no legal action for fixture (turn ${view.state.turn}, phase ${view.state.phase})`);
  const res = await fetch(`${base}/api/matches/${view.id}/actions`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ index: action.index }),
  });
  if (!res.ok) throw new Error(`play ${action.label}: ${res.status} ${await res.text()}`);
  return res.json();
}

function squarePoint(box, sq) {
  const f = sq.charCodeAt(0) - 97;
  const r = Number.parseInt(sq.slice(1), 10) - 1;
  const scale = box.width / CANVAS_SIZE;
  return {
    x: box.x + (MARGIN + f * CELL + CELL / 2) * scale,
    y: box.y + (MARGIN + (7 - r) * CELL + CELL / 2) * scale,
    scale,
  };
}

async function main() {
  const only = process.argv.slice(2);
  const wanted = (name) => only.length === 0 || only.includes(name);
  const exe = await findBrowser();
  const browser = await chromium.launch({ headless: true, executablePath: exe });
  const page = await browser.newPage({
    viewport: { width: 1500, height: 1000 },
    deviceScaleFactor: 2,
  });
  page.on('pageerror', (err) => console.error('PAGE ERROR:', err));
  page.on('console', (msg) => {
    if (msg.type() === 'error') console.error('CONSOLE ERROR:', msg.text());
  });
  page.on('requestfailed', (req) =>
    console.error('REQUEST FAILED:', req.url(), req.failure()?.errorText),
  );

  const open = async (view, dir) => {
    await mkdir(path.join(outRoot, dir), { recursive: true });
    // A hash-only navigation does not reload the SPA; force a fresh document
    // so the new match is loaded by start().
    await page.goto('about:blank');
    await page.goto(`${base}/?shot=${Date.now()}#m=${view.id}`, {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForSelector('canvas', { timeout: 15000 });
    await page.waitForSelector('#panel', { timeout: 15000 });
    await page.waitForTimeout(800);
  };

  const shot = async (dir, name) => {
    const basePath = path.join(outRoot, dir, name);
    await page.screenshot({ path: `${basePath}-full.png`, fullPage: true });
    const canvas = page.locator('canvas').first();
    await canvas.screenshot({ path: `${basePath}-board.png` });
    await page.locator('#panel').screenshot({ path: `${basePath}-panel.png` });
    return basePath;
  };

  const pieceShot = async (dir, name, sq) => {
    const box = await page.locator('canvas').first().boundingBox();
    if (!box) return;
    const p = squarePoint(box, sq);
    const size = CELL * 2.3 * p.scale;
    await page.screenshot({
      path: path.join(outRoot, dir, `${name}.png`),
      clip: {
        x: p.x - size / 2,
        y: p.y - size / 2,
        width: size,
        height: size,
      },
    });
  };

  const click = async (sq) => {
    const box = await page.locator('canvas').first().boundingBox();
    if (!box) throw new Error('board canvas not found');
    const p = squarePoint(box, sq);
    await page.mouse.click(p.x, p.y);
    await page.waitForTimeout(220);
  };

  if (wanted('showcase')) {
    // Pawn phase: the charged + buffed + nerfed B pawn on e4.
    const view = await loadSave('data/saves/effects_showcase_2026-10-27.json');
    await open(view, 'showcase');
    await click('e4');
    await shot('showcase', 'pawn');
    await pieceShot('showcase', 'pawn-e4', 'e4');
    console.log('showcase match:', `${base}/#m=${view.id}`);
  }

  if (wanted('wizard')) {
    // Wizard phase of the same showcase: buffed B wizard on b7 (portal icon).
    const view = await loadSave('data/saves/2026-10-27_10-36-07-738_1a171c0fc0bd.json');
    await open(view, 'wizard');
    await click('b7');
    await shot('wizard', 'wizard');
    await pieceShot('wizard', 'wizard-b7', 'b7');
    console.log('wizard match:', `${base}/#m=${view.id}`);
  }

  if (wanted('cannon')) {
    // B pawn move -> A's cannon phase: the buffed A cannon b1/f3 is selectable.
    let view = await loadSave('data/saves/effects_showcase_2026-10-27.json');
    view = await play(
      view,
      (a) =>
        a.kind === 'move' &&
        piece(view, a)?.owner === 'B' &&
        piece(view, a)?.type === 'pawn' &&
        piece(view, a)?.square !== 'e4',
    );
    await open(view, 'cannon-buff');
    await click('b1');
    await shot('cannon-buff', 'cannon');
    await pieceShot('cannon-buff', 'cannon-b1', 'b1');

    // A cannon move -> B's cannon phase: the nerfed B cannon c6 is selectable.
    view = await play(
      view,
      (a) =>
        a.kind === 'move' &&
        piece(view, a)?.owner === 'A' &&
        piece(view, a)?.type === 'cannon' &&
        !['b1', 'f3'].includes(piece(view, a)?.square ?? ''),
    );
    await open(view, 'cannon');
    await click('c6');
    await shot('cannon', 'cannon');
    await pieceShot('cannon', 'cannon-c6', 'c6');
    console.log('cannon match:', `${base}/#m=${view.id}`);
  }

  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
