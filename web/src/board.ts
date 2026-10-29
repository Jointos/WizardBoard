import Phaser from 'phaser';
import type { Highlights, PieceView, PlayerId, StateView } from './types';
import portalPng from './assets/portal.png?url';

const CELL = 72;
const MARGIN = 46;
const BOARD = CELL * 8;
// World size. Rendering happens at RENDER_SCALE times this resolution so the
// canvas is downscaled (crisp) instead of upscaled (pixelated) on screen.
export const CANVAS_SIZE = BOARD + MARGIN * 2;
export const RENDER_SCALE = 2;

const FILES = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

const GREEN = 0x57d98a;
const RED = 0xf06b6b;
const YELLOW = 0xf0d45e;
// Charge colour: deliberately not in the piece/buff/nerf/selection palette.
const VIOLET = 0xb388ff;

function fileOf(sq: string): number {
  return sq.charCodeAt(0) - 97;
}

function rankOf(sq: string): number {
  return Number.parseInt(sq.slice(1), 10) - 1;
}

function centerOf(sq: string): { x: number; y: number } {
  return {
    x: MARGIN + fileOf(sq) * CELL + CELL / 2,
    y: MARGIN + (7 - rankOf(sq)) * CELL + CELL / 2,
  };
}

function cellOf(sq: string): { x: number; y: number } {
  return {
    x: MARGIN + fileOf(sq) * CELL,
    y: MARGIN + (7 - rankOf(sq)) * CELL,
  };
}

function facingVector(facing: string): { x: number; y: number } {
  switch (facing) {
    case 'N':
      return { x: 0, y: -1 };
    case 'NE':
      return { x: 1, y: -1 };
    case 'E':
      return { x: 1, y: 0 };
    case 'SE':
      return { x: 1, y: 1 };
    case 'S':
      return { x: 0, y: 1 };
    case 'SW':
      return { x: -1, y: 1 };
    case 'W':
      return { x: -1, y: 0 };
    case 'NW':
      return { x: -1, y: -1 };
    default:
      return { x: 0, y: -1 };
  }
}

export type PickResult = { kind: 'dir'; dir: string } | { kind: 'center' } | { kind: 'cancel' };

export interface DirectionPickOptions {
  x: number;
  y: number;
  directions: string[];
  centerLabel: string;
  onResolve: (result: PickResult) => void;
}

interface ActivePick {
  x: number;
  y: number;
  startX: number;
  startY: number;
  dragging: boolean;
  sticky: boolean;
  hover: string | null;
  directions: string[];
  centerLabel: string;
  container: Phaser.GameObjects.Container | null;
  onResolve: (result: PickResult) => void;
}

const PICK_RADIUS = 98;

// Screen-space angle (0 = east, increasing clockwise) for each direction.
const DIR_ANGLE: Record<string, number> = {
  E: 0,
  SE: Math.PI / 4,
  S: Math.PI / 2,
  SW: (3 * Math.PI) / 4,
  W: Math.PI,
  NW: (5 * Math.PI) / 4,
  N: (3 * Math.PI) / 2,
  NE: (7 * Math.PI) / 4,
};

const DIR_GLYPH: Record<string, string> = {
  N: '↑',
  NE: '↗',
  E: '→',
  SE: '↘',
  S: '↓',
  SW: '↙',
  W: '←',
  NW: '↖',
};

const DIR_ORDER = ['E', 'SE', 'S', 'SW', 'W', 'NW', 'N', 'NE'];

export class BoardScene extends Phaser.Scene {
  private onPick: (sq: string, x: number, y: number) => void;
  private pick: ActivePick | null = null;
  private highlightG?: Phaser.GameObjects.Graphics;
  private pulseG?: Phaser.GameObjects.Graphics;
  private chargeG?: Phaser.GameObjects.Graphics;
  private effectG?: Phaser.GameObjects.Graphics;
  private pieceLayer?: Phaser.GameObjects.Container;
  private view: StateView | null = null;
  private hover: string | null = null;
  private highlights: Highlights = {
    move: [],
    target: [],
    ally: [],
    selectable: [],
    selected: '',
    last: [],
    rays: [],
    preview: [],
  };

  constructor(onPick: (sq: string, x: number, y: number) => void) {
    super({ key: 'board' });
    this.onPick = onPick;
  }

  preload(): void {
    this.load.image('fx-portal', portalPng);
  }

  create(): void {
    this.drawBoard();
    this.highlightG = this.add.graphics();
    this.chargeG = this.add.graphics();
    this.pulseG = this.add.graphics();
    this.effectG = this.add.graphics();
    this.pieceLayer = this.add.container(0, 0);

    // Everything is drawn in 660x660 world units; the camera renders them at
    // 2x so shapes and text stay sharp when the canvas is fitted to the panel.
    const cam = this.cameras.main;
    cam.setZoom(RENDER_SCALE);
    cam.centerOn(CANVAS_SIZE / 2, CANVAS_SIZE / 2);

    this.tweens.add({
      targets: this.pulseG,
      alpha: { from: 0.35, to: 1 },
      duration: 900,
      yoyo: true,
      repeat: -1,
      ease: 'Sine.easeInOut',
    });

    this.input.on('pointerdown', (pointer: Phaser.Input.Pointer) => {
      if (this.pick) {
        if (this.pick.sticky) {
          const pick = this.pick;
          this.pick = null;
          pick.container?.destroy(true);
          pick.onResolve(this.classify(pick, pointer.worldX, pointer.worldY));
        }
        return;
      }
      const sq = this.squareAt(pointer.worldX, pointer.worldY);
      if (sq) this.onPick(sq, pointer.worldX, pointer.worldY);
    });

    this.input.on('pointermove', (pointer: Phaser.Input.Pointer) => {
      if (this.pick) {
        const pick = this.pick;
        if (pointer.isDown && !pick.dragging && !pick.sticky) {
          const dx = pointer.worldX - pick.startX;
          const dy = pointer.worldY - pick.startY;
          if (Math.hypot(dx, dy) > 9) {
            pick.dragging = true;
            this.drawPick();
          }
        }
        if (pick.dragging || pick.sticky) {
          this.updatePickHover(pointer.worldX, pointer.worldY);
        }
        return;
      }
      const sq = this.squareAt(pointer.worldX, pointer.worldY);
      if (sq === this.hover) return;
      this.hover = sq;
      this.drawHighlights();
    });

    const up = (pointer: Phaser.Input.Pointer) =>
      this.onPointerUp(pointer.worldX, pointer.worldY);
    this.input.on('pointerup', up);
    this.input.on('pointerupoutside', up);

    this.redraw();
  }

  // Opens a radial facing picker at (x, y). A drag selects a direction and
  // resolves on release; a plain click leaves the wheel open until the player
  // picks a direction (or the centre) with the next click.
  beginDirectionPick(opts: DirectionPickOptions): void {
    this.cancelDirectionPick();
    this.pick = {
      x: opts.x,
      y: opts.y,
      startX: opts.x,
      startY: opts.y,
      dragging: false,
      sticky: false,
      hover: null,
      directions: opts.directions,
      centerLabel: opts.centerLabel,
      container: null,
      onResolve: opts.onResolve,
    };
    this.hover = null;
    this.drawHighlights();
  }

  cancelDirectionPick(): void {
    if (!this.pick) return;
    this.pick.container?.destroy(true);
    this.pick = null;
  }

  private onPointerUp(x: number, y: number): void {
    const pick = this.pick;
    if (!pick) return;
    if (pick.dragging) {
      this.pick = null;
      pick.container?.destroy(true);
      pick.onResolve(this.classify(pick, x, y));
      return;
    }
    if (!pick.sticky) {
      // Plain click: keep the wheel open so the player must choose a facing.
      pick.sticky = true;
      this.drawPick();
    }
  }

  private classify(pick: ActivePick, x: number, y: number): PickResult {
    const dx = x - pick.x;
    const dy = y - pick.y;
    const dist = Math.hypot(dx, dy);
    if (dist < 20) return { kind: 'center' };
    if (dist > PICK_RADIUS + 30) return { kind: 'cancel' };
    const deg = ((Math.atan2(dy, dx) * 180) / Math.PI + 360) % 360;
    const name = DIR_ORDER[Math.round(deg / 45) % 8];
    if (!pick.directions.includes(name)) return { kind: 'center' };
    return { kind: 'dir', dir: name };
  }

  private updatePickHover(x: number, y: number): void {
    const pick = this.pick;
    if (!pick) return;
    const dir = this.pickDirAt(pick, x, y);
    if (dir === pick.hover) return;
    pick.hover = dir;
    this.drawPick();
  }

  private pickDirAt(pick: ActivePick, x: number, y: number): string | null {
    const dx = x - pick.x;
    const dy = y - pick.y;
    if (Math.hypot(dx, dy) < 20) return null;
    const deg = ((Math.atan2(dy, dx) * 180) / Math.PI + 360) % 360;
    const name = DIR_ORDER[Math.round(deg / 45) % 8];
    return pick.directions.includes(name) ? name : null;
  }

  private drawPick(): void {
    const pick = this.pick;
    if (!pick) return;
    pick.container?.destroy(true);

    const c = this.add.container(0, 0).setDepth(1000);
    // Bridge the pick to the container so it can be destroyed from outside.
    pick.container = c;
    this.children.bringToTop(c);

    const R = PICK_RADIUS;
    const g = this.add.graphics();
    c.add(g);

    g.fillStyle(0x0b0e14, 0.93);
    g.fillCircle(pick.x, pick.y, R);
    g.lineStyle(2, 0x3a4a63, 1);
    g.strokeCircle(pick.x, pick.y, R);

    if (pick.hover) {
      const a = DIR_ANGLE[pick.hover];
      g.fillStyle(0xf0d45e, 0.2);
      g.slice(pick.x, pick.y, R - 5, a - Math.PI / 8, a + Math.PI / 8, false);
      g.fillPath();
    }

    g.lineStyle(1, 0x2a3547, 0.95);
    for (let i = 0; i < 8; i++) {
      const a = (i * Math.PI) / 4 + Math.PI / 8;
      g.beginPath();
      g.moveTo(pick.x + Math.cos(a) * 27, pick.y + Math.sin(a) * 27);
      g.lineTo(pick.x + Math.cos(a) * (R - 2), pick.y + Math.sin(a) * (R - 2));
      g.strokePath();
    }

    g.fillStyle(0x141a24, 1);
    g.fillCircle(pick.x, pick.y, 25);
    g.lineStyle(1, 0x3a4a63, 1);
    g.strokeCircle(pick.x, pick.y, 25);

    const center = this.add
      .text(pick.x, pick.y, pick.centerLabel, {
        fontFamily: 'ui-sans-serif, system-ui, sans-serif',
        fontSize: '10px',
        color: '#8593aa',
      })
      .setOrigin(0.5, 0.5)
      .setResolution(RENDER_SCALE);
    c.add(center);

    for (const name of pick.directions) {
      const a = DIR_ANGLE[name];
      const active = pick.hover === name;
      const t = this.add
        .text(pick.x + Math.cos(a) * 66, pick.y + Math.sin(a) * 66, DIR_GLYPH[name] ?? '?', {
          fontFamily: 'ui-sans-serif, system-ui, sans-serif',
          fontSize: active ? '30px' : '25px',
          color: active ? '#f0d45e' : '#dbe3f0',
        })
        .setOrigin(0.5, 0.5)
        .setResolution(RENDER_SCALE);
      c.add(t);
    }
  }

  setView(view: StateView | null, highlights: Highlights): void {
    this.cancelDirectionPick();
    this.view = view;
    this.highlights = highlights;
    this.redraw();
  }

  private squareAt(x: number, y: number): string | null {
    const f = Math.floor((x - MARGIN) / CELL);
    const screenRow = Math.floor((y - MARGIN) / CELL);
    if (f < 0 || f > 7 || screenRow < 0 || screenRow > 7) return null;
    return `${FILES[f]}${8 - screenRow}`;
  }

  private drawBoard(): void {
    const g = this.add.graphics();
    g.fillStyle(0x0e1219, 1);
    g.fillRect(0, 0, CANVAS_SIZE, CANVAS_SIZE);

    for (let file = 0; file < 8; file++) {
      for (let rank = 0; rank < 8; rank++) {
        const x = MARGIN + file * CELL;
        const y = MARGIN + (7 - rank) * CELL;
        const light = (file + rank) % 2 === 0;
        g.fillStyle(light ? 0x2c3648 : 0x232c3c, 1);
        g.fillRect(x, y, CELL, CELL);
      }
    }

    g.lineStyle(2, 0x0a0d13, 1);
    g.strokeRect(MARGIN, MARGIN, BOARD, BOARD);

    const labelStyle: Phaser.Types.GameObjects.Text.TextStyle = {
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: '13px',
      color: '#6f7d95',
    };
    for (let file = 0; file < 8; file++) {
      this.add
        .text(MARGIN + file * CELL + CELL / 2, MARGIN + BOARD + 14, FILES[file], labelStyle)
        .setOrigin(0.5, 0.5)
        .setResolution(RENDER_SCALE);
    }
    for (let rank = 0; rank < 8; rank++) {
      this.add
        .text(MARGIN - 18, MARGIN + (7 - rank) * CELL + CELL / 2, String(rank + 1), labelStyle)
        .setOrigin(0.5, 0.5)
        .setResolution(RENDER_SCALE);
    }
  }

  private redraw(): void {
    if (!this.highlightG || !this.pieceLayer) return;
    this.drawHighlights();
    this.drawCharges();
    this.drawEffects();
    this.pieceLayer.removeAll(true);
    if (!this.view) return;
    for (const piece of this.view.pieces) {
      this.pieceLayer.add(this.makePiece(piece));
    }
  }

  // Static (non-pulsing) violet glow under pawns whose next action is the
  // forward capture. Colour is deliberately outside the game palette
  // (blue/orange pieces, green buff, red nerf, yellow selection).
  private drawCharges(): void {
    const g = this.chargeG;
    if (!g) return;
    g.clear();
    if (!this.view) return;
    for (const piece of this.view.pieces) {
      if (piece.type !== 'pawn' || piece.acts < 2) continue;
      const c = centerOf(piece.square);
      g.fillStyle(VIOLET, 0.1);
      g.fillCircle(c.x, c.y, 34);
      g.fillStyle(VIOLET, 0.16);
      g.fillCircle(c.x, c.y, 27);
    }
  }

  // ---- highlights -------------------------------------------------------

  private drawHighlights(): void {
    const g = this.highlightG;
    if (!g) return;
    g.clear();

    // Faint dots along the selected wizard's 8 cast lines.
    for (const sq of this.highlights.rays) {
      const c = centerOf(sq);
      g.fillStyle(0x8fb4ff, 0.18);
      g.fillCircle(c.x, c.y, 2.2);
    }

    for (const sq of this.highlights.last) {
      const { x, y } = cellOf(sq);
      g.fillStyle(0xf0b45e, 0.14);
      g.fillRect(x, y, CELL, CELL);
    }

    // Squares the selected piece can move to.
    for (const sq of this.highlights.move) {
      const { x, y } = cellOf(sq);
      g.fillStyle(GREEN, 0.2);
      g.fillRect(x, y, CELL, CELL);
      g.lineStyle(2, GREEN, 0.75);
      g.strokeRect(x + 1, y + 1, CELL - 2, CELL - 2);
      const c = centerOf(sq);
      g.fillStyle(GREEN, 0.55);
      g.fillCircle(c.x, c.y, 4);
    }

    // Hostile targets (capture / shoot / nerf / cleanse an enemy).
    for (const sq of this.highlights.target) {
      const { x, y } = cellOf(sq);
      g.lineStyle(3, RED, 0.95);
      g.strokeRect(x + 4, y + 4, CELL - 8, CELL - 8);
    }

    // Friendly / support targets (buff / cleanse an ally).
    for (const sq of this.highlights.ally) {
      const { x, y } = cellOf(sq);
      g.lineStyle(3, GREEN, 0.95);
      g.strokeRect(x + 4, y + 4, CELL - 8, CELL - 8);
    }

    // AI-assist preview (white outline, does not mean "playable").
    for (const sq of this.highlights.preview) {
      const { x, y } = cellOf(sq);
      g.lineStyle(2, 0xffffff, 0.85);
      g.strokeRect(x + 5, y + 5, CELL - 10, CELL - 10);
    }

    this.drawSelectable();
    this.drawSelection(g);
    this.drawHover(g);
  }

  // Pulsing rings around every unit that can act this turn.
  private drawSelectable(): void {
    const g = this.pulseG;
    if (!g) return;
    g.clear();
    for (const sq of this.highlights.selectable) {
      if (sq === this.highlights.selected) continue;
      const c = centerOf(sq);
      g.fillStyle(YELLOW, 0.16);
      g.fillCircle(c.x, c.y, 33);
      g.lineStyle(3, YELLOW, 0.95);
      g.strokeCircle(c.x, c.y, 32);
    }
  }

  private drawSelection(g: Phaser.GameObjects.Graphics): void {
    const sq = this.highlights.selected;
    if (!sq) return;
    const cell = cellOf(sq);
    g.fillStyle(YELLOW, 0.12);
    g.fillRect(cell.x, cell.y, CELL, CELL);
    const c = centerOf(sq);
    g.lineStyle(3, YELLOW, 1);
    g.strokeCircle(c.x, c.y, 31);
    // Corner brackets make the selection readable even with pieces on top.
    const r = 12;
    g.lineStyle(3, YELLOW, 1);
    g.beginPath();
    g.moveTo(cell.x + 3, cell.y + 3 + r);
    g.lineTo(cell.x + 3, cell.y + 3);
    g.lineTo(cell.x + 3 + r, cell.y + 3);
    g.moveTo(cell.x + CELL - 3 - r, cell.y + 3);
    g.lineTo(cell.x + CELL - 3, cell.y + 3);
    g.lineTo(cell.x + CELL - 3, cell.y + 3 + r);
    g.moveTo(cell.x + CELL - 3, cell.y + CELL - 3 - r);
    g.lineTo(cell.x + CELL - 3, cell.y + CELL - 3);
    g.lineTo(cell.x + CELL - 3 - r, cell.y + CELL - 3);
    g.moveTo(cell.x + 3 + r, cell.y + CELL - 3);
    g.lineTo(cell.x + 3, cell.y + CELL - 3);
    g.lineTo(cell.x + 3, cell.y + CELL - 3 - r);
    g.strokePath();
  }

  private drawHover(g: Phaser.GameObjects.Graphics): void {
    const sq = this.hover;
    if (!sq) return;
    const actionable =
      this.highlights.move.includes(sq) ||
      this.highlights.target.includes(sq) ||
      this.highlights.ally.includes(sq) ||
      this.highlights.selectable.includes(sq);
    if (!actionable) return;
    const { x, y } = cellOf(sq);
    g.lineStyle(2, 0xffffff, 0.65);
    g.strokeRect(x + 2, y + 2, CELL - 4, CELL - 4);
  }

  // ---- magical links + caster badges ------------------------------------

  private drawEffects(): void {
    const g = this.effectG;
    if (!g) return;
    g.clear();
    if (!this.view) return;
    for (const piece of this.view.pieces) {
      if (piece.buffBy) this.drawLink(g, piece.buffBy, piece.square, GREEN);
      if (piece.nerfBy) this.drawLink(g, piece.nerfBy, piece.square, RED);
    }
  }

  private drawLink(
    g: Phaser.GameObjects.Graphics,
    fromSq: string,
    toSq: string,
    color: number,
  ): void {
    const a = centerOf(fromSq);
    const b = centerOf(toSq);
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const len = Math.hypot(dx, dy);
    if (len < 1) return;
    const ux = dx / len;
    const uy = dy / len;
    const x0 = a.x + ux * 20;
    const y0 = a.y + uy * 20;
    const x1 = b.x - ux * 27;
    const y1 = b.y - uy * 27;
    const dots = Math.max(3, Math.floor(Math.hypot(x1 - x0, y1 - y0) / 9));
    for (let i = 0; i <= dots; i++) {
      const t = i / dots;
      g.fillStyle(color, 0.5);
      g.fillCircle(x0 + (x1 - x0) * t, y0 + (y1 - y0) * t, 1.7);
    }
    g.fillStyle(color, 0.85);
    g.fillCircle(x1, y1, 3);
  }

  // ---- pieces ------------------------------------------------------------

  // Three small arrows under the pawn, fanning towards the enemy side. They
  // sit on a dark violet plate so the marker reads as an ability badge and
  // cannot be mistaken for the yellow selection ring.
  private drawForwardArrows(g: Phaser.GameObjects.Graphics, owner: PlayerId): void {
    g.fillStyle(0x241a36, 0.94);
    g.fillRoundedRect(-15, 19, 30, 14, 4);
    g.lineStyle(1.1, VIOLET, 0.95);
    g.strokeRoundedRect(-15, 19, 30, 14, 4);

    const base = owner === 'A' ? 0 : Math.PI;
    const xs = [-8.5, 0, 8.5];
    const splays = [-0.6, 0, 0.6];
    for (let i = 0; i < 3; i++) {
      this.drawArrow(g, xs[i], 26, base + splays[i]);
    }
  }

  // A proper arrow: a thin shaft plus a small head.
  private drawArrow(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    angle: number,
    color = 0xd9c2ff,
  ): void {
    const total = 11;
    const headLen = 4.6;
    const headHalf = 3.1;
    const dx = Math.sin(angle);
    const dy = -Math.cos(angle);
    const px = Math.cos(angle);
    const py = Math.sin(angle);

    const tipX = x + dx * (total / 2);
    const tipY = y + dy * (total / 2);
    const tailX = x - dx * (total / 2);
    const tailY = y - dy * (total / 2);
    const neckX = tipX - dx * headLen;
    const neckY = tipY - dy * headLen;

    g.lineStyle(2.2, color, 1);
    g.beginPath();
    g.moveTo(tailX, tailY);
    g.lineTo(neckX, neckY);
    g.strokePath();

    g.fillStyle(color, 1);
    g.fillTriangle(
      tipX,
      tipY,
      neckX + px * headHalf,
      neckY + py * headHalf,
      neckX - px * headHalf,
      neckY - py * headHalf,
    );
  }

  private drawSparkle(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    r: number,
    color: number,
  ): void {
    const w = r * 0.32;
    g.fillStyle(color, 1);
    g.fillTriangle(x, y - r, x + w, y, x - w, y);
    g.fillTriangle(x, y + r, x + w, y, x - w, y);
    g.fillTriangle(x - r, y, x, y - w, x, y + w);
    g.fillTriangle(x + r, y, x, y - w, x, y + w);
  }

  private makePiece(piece: PieceView): Phaser.GameObjects.Container {
    const { x, y } = centerOf(piece.square);
    const c = this.add.container(x, y);
    const g = this.add.graphics();

    const isA = piece.owner === 'A';
    const fill = isA ? 0x7aa2ff : 0xffb066;
    const edge = isA ? 0x2b3f74 : 0x7d4a17;
    const ink = isA ? 0x0b1428 : 0x2a1403;

    g.fillStyle(0x000000, 0.35);
    g.fillEllipse(0, 23, 44, 12);

    if (piece.type === 'pawn') {
      g.fillStyle(edge, 1);
      g.fillRoundedRect(-13, 10, 26, 10, 4);
      g.fillStyle(fill, 1);
      g.fillCircle(0, -3, 15);
      g.lineStyle(2, ink, 1);
      g.strokeCircle(0, -3, 15);
    } else if (piece.type === 'cannon') {
      g.fillStyle(edge, 1);
      g.fillRoundedRect(-18, -13, 36, 28, 7);
      g.fillStyle(fill, 1);
      g.fillRoundedRect(-15, -10, 30, 22, 6);
      const dir = facingVector(piece.facing);
      g.lineStyle(6, ink, 1);
      g.beginPath();
      g.moveTo(0, 0);
      g.lineTo(dir.x * 19, dir.y * 19);
      g.strokePath();
    } else {
      // Wizard: a robed figure with a pointed hat, a beard and a staff with a
      // glowing orb. It is the tallest piece so it reads as the main character.
      const skin = 0xf0d7b8;

      // Staff behind the body.
      g.lineStyle(3, ink, 1);
      g.beginPath();
      g.moveTo(17, 17);
      g.lineTo(17, -13);
      g.strokePath();
      g.fillStyle(0xf0d45e, 0.3);
      g.fillCircle(17, -17, 6.5);
      g.fillStyle(0xf0d45e, 1);
      g.fillCircle(17, -17, 3.4);
      g.fillStyle(0xfff6d0, 1);
      g.fillCircle(16.4, -18, 1.3);

      // Robe.
      const robe = [
        new Phaser.Geom.Point(-8, -4),
        new Phaser.Geom.Point(8, -4),
        new Phaser.Geom.Point(16, 18),
        new Phaser.Geom.Point(-16, 18),
      ];
      g.fillStyle(fill, 1);
      g.fillPoints(robe, true);
      g.lineStyle(2, ink, 1);
      g.strokePoints(robe, true);

      // Belt and golden hem.
      g.fillStyle(edge, 1);
      g.fillRect(-10, 5, 20, 3);
      g.fillStyle(0xf0d45e, 0.9);
      g.fillRect(-15, 15, 30, 2);

      // Head and beard.
      g.fillStyle(skin, 1);
      g.fillCircle(0, -9, 5.5);
      g.lineStyle(1.5, ink, 1);
      g.strokeCircle(0, -9, 5.5);
      g.fillStyle(0xe8eef8, 1);
      g.fillTriangle(-4, -4.5, 4, -4.5, 0, 2.5);

      // Pointed hat with a brim and a band.
      const hat = [
        new Phaser.Geom.Point(0, -26),
        new Phaser.Geom.Point(9, -12),
        new Phaser.Geom.Point(-9, -12),
      ];
      g.fillStyle(edge, 1);
      g.fillPoints(hat, true);
      g.lineStyle(2, ink, 1);
      g.strokePoints(hat, true);
      g.fillStyle(edge, 1);
      g.fillRoundedRect(-12, -13.5, 24, 4, 2);
      g.lineStyle(1.5, ink, 1);
      g.strokeRoundedRect(-12, -13.5, 24, 4, 2);
      g.fillStyle(0xf0d45e, 0.95);
      g.fillRect(-9, -15.5, 18, 2);

      // Sparkle on the robe.
      this.drawSparkle(g, 0, 10, 3.6, 0xf0d45e);
    }

    c.add(g);

    // Pawn charge indicator: two pips while the pawn builds up, three forward
    // arrows when the next action is the forward capture.
    if (piece.type === 'pawn') {
      const marks = this.add.graphics();
      if (piece.acts >= 2) {
        this.drawForwardArrows(marks, piece.owner);
      } else {
        for (let i = 0; i < 2; i++) {
          const filled = i < piece.acts;
          marks.fillStyle(filled ? 0xd7dfec : 0x55647e, 1);
          marks.fillCircle(-5 + i * 10, 26, 3);
        }
      }
      c.add(marks);
    }

    // Caster links already point at the wizard; the badge explains the effect
    // itself instead of repeating the caster square.
    if (piece.buffed && piece.nerfed) {
      this.drawEffectBadge(c, piece, 'buff', -15);
      this.drawEffectBadge(c, piece, 'nerf', 15);
    } else if (piece.buffed) {
      this.drawEffectBadge(c, piece, 'buff', 0);
    } else if (piece.nerfed) {
      this.drawEffectBadge(c, piece, 'nerf', 0);
    }

    // A nerfed pawn must retreat, so its side-capture buff and its ready
    // forward-capture charge are both unavailable. Strike them through.
    if (piece.type === 'pawn' && piece.nerfed) {
      const g = this.add.graphics();
      if (piece.buffed) this.drawSuppressedSlash(g, -15, -29);
      if (piece.acts >= 2) this.drawSuppressedSlash(g, 0, 26);
      c.add(g);
    }

    return c;
  }

  private drawEffectBadge(
    container: Phaser.GameObjects.Container,
    piece: PieceView,
    effect: 'buff' | 'nerf',
    x: number,
  ): void {
    const color = effect === 'buff' ? GREEN : RED;
    const light = effect === 'buff' ? 0xa9f0c4 : 0xffb3b3;
    const y = piece.type === 'wizard' ? -37 : -29;
    const g = this.add.graphics();
    g.fillStyle(0x0b0e14, 0.94);
    g.fillRoundedRect(x - 14, y - 7, 28, 14, 4);
    g.lineStyle(1.1, color, 0.95);
    g.strokeRoundedRect(x - 14, y - 7, 28, 14, 4);
    container.add(g);

    if (effect === 'buff') {
      if (piece.type === 'cannon') {
        this.drawRangeGlyph(g, x, y, light);
      } else if (piece.type === 'wizard') {
        const icon = this.add
          .image(x, y, 'fx-portal')
          .setDisplaySize(13, 13)
          .setTintFill(GREEN);
        container.add(icon);
      } else {
        this.drawSideArrows(g, x, y, light);
      }
      return;
    }
    if (piece.type === 'cannon') {
      this.drawNoTurnGlyph(g, x, y, light);
    } else if (piece.type === 'pawn') {
      this.drawRetreatGlyph(g, x, y, piece.owner, light);
    } else {
      this.drawMinusGlyph(g, x, y, light);
    }
  }

  private drawSideArrows(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    color: number,
  ): void {
    this.drawArrow(g, x - 6, y, -Math.PI / 2, color);
    this.drawArrow(g, x + 6, y, Math.PI / 2, color);
  }

  private drawRetreatGlyph(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    owner: PlayerId,
    color: number,
  ): void {
    // Backwards is down for A and up for B.
    this.drawArrow(g, x, y, owner === 'A' ? Math.PI : 0, color);
  }

  private drawRangeGlyph(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    color: number,
  ): void {
    g.lineStyle(1.3, color, 1);
    g.strokeCircle(x - 6, y, 3.1);
    g.beginPath();
    g.moveTo(x - 10, y);
    g.lineTo(x - 2, y);
    g.moveTo(x - 6, y - 4);
    g.lineTo(x - 6, y + 4);
    g.strokePath();
    g.fillStyle(color, 1);
    g.fillCircle(x + 2.5, y, 1.05);
    g.fillCircle(x + 5.5, y, 1.05);
    g.fillCircle(x + 8.5, y, 1.05);
  }

  private drawNoTurnGlyph(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    color: number,
  ): void {
    // Draw the thin slash first; the rotation arrow must stay readable on top.
    g.lineStyle(1.2, color, 0.95);
    g.beginPath();
    g.moveTo(x - 4.6, y - 4.6);
    g.lineTo(x + 4.6, y + 4.6);
    g.strokePath();

    const r = 3.8;
    const end = 5.0;
    g.lineStyle(1.5, color, 1);
    g.beginPath();
    g.arc(x, y, r, 0.6, end, false);
    g.strokePath();

    // Proper arrowhead at the end of the rotation arc.
    const ex = x + Math.cos(end) * r;
    const ey = y + Math.sin(end) * r;
    const tx = -Math.sin(end);
    const ty = Math.cos(end);
    const px = -ty;
    const py = tx;
    const tipX = ex + tx * 3.2;
    const tipY = ey + ty * 3.2;
    g.fillStyle(color, 1);
    g.fillTriangle(
      tipX,
      tipY,
      ex + px * 1.8,
      ey + py * 1.8,
      ex - px * 1.8,
      ey - py * 1.8,
    );
  }

  private drawSuppressedSlash(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
  ): void {
    g.lineStyle(2.8, 0x2a0b0b, 0.85);
    g.beginPath();
    g.moveTo(x - 11, y - 5);
    g.lineTo(x + 11, y + 5);
    g.strokePath();
    g.lineStyle(1.7, 0xff7b7b, 1);
    g.beginPath();
    g.moveTo(x - 10.5, y - 4.5);
    g.lineTo(x + 10.5, y + 4.5);
    g.strokePath();
  }

  private drawMinusGlyph(
    g: Phaser.GameObjects.Graphics,
    x: number,
    y: number,
    color: number,
  ): void {
    g.lineStyle(2, color, 1);
    g.beginPath();
    g.moveTo(x - 4, y);
    g.lineTo(x + 4, y);
    g.strokePath();
  }
}
