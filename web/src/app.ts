import { api } from './api';
import { AudioFX } from './audio';
import { buffMeaning, effectIcon, nerfMeaning, spellIcon, spellNote } from './icons';
import type { ActionView, AIInfo, AnalysisMove, AnalysisView, Highlights, MatchView, PieceView } from './types';
import type { BoardScene } from './board';

type Mode = 'idle' | 'teleport' | 'swap';

interface TargetPick {
  sq: string;
  actions: ActionView[];
}

const SAVE_ICON = `<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>`;

const LOAD_ICON = `<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>`;

function cap(s: string): string {
  return s.length ? s[0].toUpperCase() + s.slice(1) : s;
}

function fmtScore(n: number): string {
  return n > 0 ? `+${n}` : `${n}`;
}

function esc(s: string): string {
  return s.replace(/[&<>"']/g, (c) => {
    switch (c) {
      case '&':
        return '&amp;';
      case '<':
        return '&lt;';
      case '>':
        return '&gt;';
      case '"':
        return '&quot;';
      default:
        return '&#39;';
    }
  });
}

export class App {
  private view: MatchView | null = null;
  private board: BoardScene | null = null;
  private panel: HTMLElement;
  private selected: number | null = null;
  private mode: Mode = 'idle';
  private targetPick: TargetPick | null = null;
  private error = '';
  private busy = false;
  private aiThinking = false;
  private opSeq = 0;
  private toast = '';
  private toastErr = false;
  private toastTimer = 0;
  private assist = (() => {
    try {
      return localStorage.getItem('wb-assist') === '1';
    } catch {
      return false;
    }
  })();
  private analysis: AnalysisView | null = null;
  private analysisError = '';
  private analysisBusy = false;
  private analysisSeq = 0;
  private preview: string[] = [];
  private sound = new AudioFX();
  private silentNext = false;
  private ais: AIInfo[] = [];
  private settings = {
    opponent: 'aiB' as 'human' | 'aiA' | 'aiB',
    engine: 'ml-r2-v2',
    strength: 'strong' as 'fast' | 'strong' | 'max',
  };

  constructor() {
    const panel = document.getElementById('panel');
    if (!panel) throw new Error('#panel missing');
    this.panel = panel;
    this.panel.addEventListener('click', (ev) => this.onPanelClick(ev));
    this.panel.addEventListener('change', (ev) => this.onPanelChange(ev));
    const unlock = () => this.sound.unlock();
    window.addEventListener('pointerdown', unlock, { passive: true });
    window.addEventListener('keydown', unlock);
    window.addEventListener('keydown', (ev) => {
      if (ev.key === 'Escape') {
        this.board?.cancelDirectionPick();
        this.selected = null;
        this.mode = 'idle';
        this.targetPick = null;
        this.render();
      }
    });
    this.render();
  }

  attachBoard(board: BoardScene): void {
    this.board = board;
    this.render();
  }

  async start(): Promise<void> {
    try {
      this.ais = await api.listAI();
      if (this.ais.length > 0 && !this.ais.some((a) => a.id === this.settings.engine)) {
        this.settings.engine = this.ais[0].id;
      }
    } catch {
      // Older server or offline: keep the built-in default.
    }
    this.render();
    const fromHash = /^#m=([a-z0-9]+)$/i.exec(window.location.hash);
    if (fromHash) {
      await this.run(() => api.get(fromHash[1]));
      if (this.view) return;
    }
    await this.newGame();
  }

  async newGame(): Promise<void> {
    this.opSeq++;
    this.selected = null;
    this.mode = 'idle';
    this.targetPick = null;
    this.error = '';
    this.aiThinking = false;
    const opponent = this.settings.opponent;
    const opts = {
      ai: { A: opponent === 'aiA', B: opponent === 'aiB' },
      engine: this.settings.engine,
      difficulty: this.settings.strength,
    };
    await this.run(() => api.create(opts));
  }

  private async runAI(id: string): Promise<void> {
    if (this.busy || !this.view || this.view.id !== id || !this.view.pendingAI) return;
    const seq = this.opSeq;
    this.busy = true;
    this.aiThinking = true;
    this.error = '';
    this.render();
    try {
      const view = await api.ai(id);
      if (seq !== this.opSeq) return; // undone or restarted meanwhile
      this.aiThinking = false;
      this.acceptView(view);
    } catch (err) {
      if (seq !== this.opSeq) return;
      this.aiThinking = false;
      this.error = err instanceof Error ? err.message : String(err);
      this.sound.error();
    } finally {
      if (seq === this.opSeq) {
        this.busy = false;
        this.render();
      }
    }
  }

  private async undo(): Promise<void> {
    const view = this.view;
    if (!view || !view.canUndo) return;
    const id = view.id;
    const seq = ++this.opSeq;
    // Cancels any in-flight AI move: its reply is dropped by the sequence check.
    this.aiThinking = false;
    this.selected = null;
    this.mode = 'idle';
    this.targetPick = null;
    this.error = '';
    this.busy = true;
    this.render();
    try {
      const next = await api.undo(id);
      if (seq !== this.opSeq) return;
      this.busy = false;
      this.acceptView(next);
    } catch (err) {
      if (seq !== this.opSeq) return;
      this.error = err instanceof Error ? err.message : String(err);
      this.sound.error();
    } finally {
      if (seq === this.opSeq) {
        this.busy = false;
        this.render();
      }
    }
  }

  private async submit(index: number): Promise<void> {
    if (!this.view) return;
    const id = this.view.id;
    await this.run(() => api.play(id, index));
  }

  private async run(fn: () => Promise<MatchView>): Promise<void> {
    if (this.busy) return;
    this.busy = true;
    this.error = '';
    this.render();
    try {
      const view = await fn();
      this.acceptView(view);
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      this.sound.error();
    } finally {
      this.busy = false;
      this.render();
    }
  }

  private resultKind(view: MatchView): 'win' | 'lose' | 'neutral' {
    const winner = view.state.winner;
    if (winner !== 'A' && winner !== 'B') return 'neutral';
    const humanA = !view.ai.A;
    const humanB = !view.ai.B;
    if (humanA === humanB) return 'neutral';
    const human = humanA ? 'A' : 'B';
    return winner === human ? 'win' : 'lose';
  }

  private acceptView(view: MatchView): void {
    const prev = this.view;
    const silent = this.silentNext;
    this.silentNext = false;
    this.view = view;
    window.history.replaceState(null, '', `#m=${view.id}`);
    // Any previous analysis belongs to a different position.
    this.analysisSeq++;
    this.analysis = null;
    this.analysisError = '';
    this.analysisBusy = false;
    this.preview = [];
    const st = view.state;
    if (!silent && prev && prev.id === view.id) {
      const delta = view.state.ply - prev.state.ply;
      if (delta > 0 && view.lastAction) {
        this.sound.action(view.lastAction);
        if (view.state.over) {
          window.setTimeout(() => this.sound.result(this.resultKind(view)), 340);
        }
      } else if (delta < 0) {
        this.sound.undo();
      }
    }
    if (st.over) {
      this.selected = null;
      this.mode = 'idle';
      this.targetPick = null;
      this.aiThinking = false;
      return;
    }
    if (view.pendingAI) {
      // The human cannot act: clear local selection and let the AI move.
      this.selected = null;
      this.mode = 'idle';
      this.targetPick = null;
      this.aiThinking = true;
      const id = view.id;
      window.setTimeout(() => {
        if (this.view?.id === id) void this.runAI(id);
      }, 220);
      return;
    }
    this.aiThinking = false;
    if (st.pending >= 0) {
      this.selected = st.pending;
      this.mode = 'idle';
      this.targetPick = null;
      return;
    }
    if (this.selected != null) {
      const piece = st.pieces.find((p) => p.id === this.selected);
      if (!piece || piece.owner !== st.turn || piece.type !== st.phase) {
        this.selected = null;
      }
    }
    this.mode = 'idle';
    this.targetPick = null;
    this.maybeAnalyze();
  }

  private maybeAnalyze(): void {
    this.analysisSeq++;
    this.analysis = null;
    this.analysisError = '';
    this.preview = [];
    const view = this.view;
    if (!this.assist || !view || view.state.over || view.pendingAI) {
      this.analysisBusy = false;
      return;
    }
    const seq = this.analysisSeq;
    const id = view.id;
    const ply = view.state.ply;
    this.analysisBusy = true;
    this.render();
    void this.runAnalysis(id, ply, seq);
  }

  private async runAnalysis(id: string, ply: number, seq: number): Promise<void> {
    try {
      const res = await api.analyze(id);
      if (seq !== this.analysisSeq || this.view?.id !== id || this.view?.state.ply !== ply) return;
      this.analysis = res;
    } catch (err) {
      if (seq !== this.analysisSeq) return;
      this.analysisError = err instanceof Error ? err.message : String(err);
    } finally {
      if (seq === this.analysisSeq) {
        this.analysisBusy = false;
        this.render();
      }
    }
  }

  // The 8 cast lines of the selected wizard, drawn as faint dots.
  private wizardRays(): string[] {
    const view = this.view;
    if (!view || this.selected == null) return [];
    const wiz = view.state.pieces.find((p) => p.id === this.selected);
    if (!wiz || wiz.type !== 'wizard') return [];
    const fx = wiz.square.charCodeAt(0) - 97;
    const fy = Number.parseInt(wiz.square.slice(1), 10) - 1;
    const dirs: Array<[number, number]> = [
      [0, 1],
      [1, 1],
      [1, 0],
      [1, -1],
      [0, -1],
      [-1, -1],
      [-1, 0],
      [-1, 1],
    ];
    const out: string[] = [];
    for (const [dx, dy] of dirs) {
      for (let n = 1; n < 8; n++) {
        const x = fx + dx * n;
        const y = fy + dy * n;
        if (x < 0 || x > 7 || y < 0 || y > 7) break;
        out.push(`${String.fromCharCode(97 + x)}${y + 1}`);
      }
    }
    return out;
  }

  // Every unit of the active phase that actually has a legal action.
  private selectableSquares(): string[] {
    const view = this.view;
    if (!view || view.state.over || view.pendingAI) return [];
    const st = view.state;
    if (st.pending >= 0) {
      const w = st.pieces.find((p) => p.id === st.pending);
      return w ? [w.square] : [];
    }
    const withActions = new Set(view.legal.map((a) => a.unit));
    return st.pieces
      .filter((p) => p.owner === st.turn && p.type === st.phase && withActions.has(p.id))
      .map((p) => p.square);
  }

  private selectUnit(id: number): void {
    if (this.busy || !this.view || this.view.state.over) return;
    this.selected = id;
    this.mode = 'idle';
    this.targetPick = null;
    this.render();
  }

  private highlights(): Highlights {
    const view = this.view;
    const h: Highlights = {
      move: [],
      target: [],
      ally: [],
      selectable: [],
      selected: '',
      last: view?.last ?? [],
      rays: [],
      preview: this.preview,
    };
    if (!view || view.state.over || view.pendingAI) return h;
    const st = view.state;
    h.selectable = this.selectableSquares();

    if (st.pending >= 0) {
      for (const a of view.legal) {
        if (a.kind === 'freemove' && a.to) h.move.push(a.to);
      }
      h.selected = st.pieces.find((p) => p.id === st.pending)?.square ?? '';
      return h;
    }

    if (this.mode === 'teleport') {
      for (const a of view.legal) {
        if (a.kind !== 'teleport' || !a.to) continue;
        if (this.selected != null && a.unit !== this.selected) continue;
        h.move.push(a.to);
      }
    }

    if (this.mode === 'swap') {
      for (const a of view.legal) {
        if (a.kind !== 'swap' || !a.targetSquare) continue;
        if (this.selected != null && a.unit !== this.selected) continue;
        h.ally.push(a.targetSquare);
      }
    }

    h.rays = this.wizardRays();

    if (this.selected != null) {
      h.selected = st.pieces.find((p) => p.id === this.selected)?.square ?? '';
      for (const a of view.legal) {
        if (a.unit !== this.selected) continue;
        if (a.kind === 'move' || a.kind === 'moveturn') h.move.push(a.to);
        else if (a.kind === 'capture' || a.kind === 'shoot') h.target.push(a.targetSquare);
        else if (a.kind === 'cast') {
          if (a.spell === 'buff') {
            h.ally.push(a.targetSquare);
          } else if (a.spell === 'nerf') {
            h.target.push(a.targetSquare);
          } else {
            // Cleanse is support on your own units and hostile on enemy units.
            const caster = st.pieces.find((p) => p.id === a.unit);
            const target = st.pieces.find((p) => p.id === a.target);
            if (caster && target && caster.owner === target.owner) h.ally.push(a.targetSquare);
            else h.target.push(a.targetSquare);
          }
        }
      }
    }
    return h;
  }

  onSquareClick(sq: string, x: number, y: number): void {
    const view = this.view;
    if (!view || this.busy || this.aiThinking || view.pendingAI || view.state.over) return;
    const st = view.state;

    if (st.pending >= 0) {
      const a = view.legal.find((x) => x.kind === 'freemove' && x.to === sq);
      if (a) {
        this.targetPick = null;
        void this.submit(a.index);
      }
      return;
    }

    if (this.mode === 'teleport') {
      // Every wizard may teleport to the same empty square, so the selected
      // unit must be part of the lookup; otherwise the first teleport action
      // for that square wins and a different wizard moves.
      const a = view.legal.find(
        (x) =>
          x.kind === 'teleport' &&
          x.to === sq &&
          (this.selected == null || x.unit === this.selected),
      );
      if (a) {
        this.mode = 'idle';
        void this.submit(a.index);
      }
      return;
    }

    if (this.mode === 'swap') {
      // A wizard may exchange places with any adjacent friendly piece.
      const a = view.legal.find(
        (x) =>
          x.kind === 'swap' &&
          x.targetSquare === sq &&
          (this.selected == null || x.unit === this.selected),
      );
      if (a) {
        this.mode = 'idle';
        void this.submit(a.index);
      }
      return;
    }

    if (this.targetPick) return;

    const piece = st.pieces.find((p) => p.square === sq);
    const activeUnit =
      piece && piece.owner === st.turn && piece.type === st.phase ? piece : null;

    // Nothing selected yet: the first click picks one of the active units.
    if (this.selected == null) {
      if (activeUnit) this.selectUnit(activeUnit.id);
      return;
    }

    // Casts may come from either of the player's wizards: on a wizard turn the
    // player may act with any one of them, so a cleanse that the selected
    // wizard cannot reach must still be offered. Clicking an active unit keeps
    // the selected-unit filter so wizard switching still works.
    const casts = view.legal.filter((a) => {
      if (a.kind !== 'cast' || a.targetSquare !== sq) return false;
      if (activeUnit) return a.unit === this.selected;
      return true;
    });
    if (casts.length > 1) {
      this.targetPick = { sq, actions: casts };
      this.render();
      return;
    }
    if (casts.length === 1) {
      void this.submit(casts[0].index);
      return;
    }

    if (activeUnit) {
      // Pressing the already selected cannon starts the facing wheel: drag the
      // mouse toward a direction and release.
      if (activeUnit.id === this.selected && activeUnit.type === 'cannon' && this.board) {
        const turns = view.legal.filter(
          (a) => a.unit === this.selected && a.kind === 'turn',
        );
        if (turns.length > 0) {
          this.board.beginDirectionPick({
            x,
            y,
            directions: turns.map((t) => t.facing),
            centerLabel: 'cancel',
            onResolve: (res) => {
              if (res.kind !== 'dir') return;
              const a = turns.find((t) => t.facing === res.dir);
              if (a) void this.submit(a.index);
            },
          });
          return;
        }
      }
      this.selectUnit(activeUnit.id);
      return;
    }

    const dests = view.legal.filter(
      (a) =>
        a.unit === this.selected &&
        a.to === sq &&
        (a.kind === 'move' || a.kind === 'moveturn' || a.kind === 'capture' || a.kind === 'shoot'),
    );
    if (dests.length === 0) return;

    const capture = dests.find((a) => a.kind === 'capture');
    if (capture) {
      void this.submit(capture.index);
      return;
    }
    const shot = dests.find((a) => a.kind === 'shoot');
    if (shot) {
      void this.submit(shot.index);
      return;
    }
    const turns = dests.filter((a) => a.kind === 'moveturn');
    const move = dests.find((a) => a.kind === 'move');
    if (turns.length > 0 && this.board) {
      // Click = move, drag toward a direction = move and face that way.
      this.board.beginDirectionPick({
        x,
        y,
        directions: turns.map((t) => t.facing),
        centerLabel: 'keep',
        onResolve: (res) => {
          if (res.kind === 'cancel') return;
          if (res.kind === 'center') {
            if (move) void this.submit(move.index);
            return;
          }
          const a = turns.find((t) => t.facing === res.dir);
          if (a) void this.submit(a.index);
        },
      });
      return;
    }
    if (move) void this.submit(move.index);
  }

  private onPanelClick(ev: MouseEvent): void {
    const target = ev.target as HTMLElement | null;
    const el = target?.closest<HTMLElement>('[data-cmd]');
    if (!el) return;
    const cmd = el.dataset.cmd;
    switch (cmd) {
      case 'act': {
        const idx = Number(el.dataset.idx);
        if (!Number.isFinite(idx) || idx < 0) return;
        this.board?.cancelDirectionPick();
        this.targetPick = null;
        this.mode = 'idle';
        void this.submit(idx);
        break;
      }
      case 'new':
        void this.newGame();
        break;
      case 'ai':
        void this.runAI(this.view?.id ?? '');
        break;
      case 'save':
        void this.saveState();
        break;
      case 'load':
        void this.loadState();
        break;
      case 'apply':
        void this.applyConfig();
        break;
      case 'assist': {
        this.assist = !this.assist;
        try {
          localStorage.setItem('wb-assist', this.assist ? '1' : '0');
        } catch {
          // localStorage can be unavailable; the toggle still works for this session.
        }
        this.analysisSeq++;
        this.analysis = null;
        this.analysisError = '';
        this.analysisBusy = false;
        this.preview = [];
        this.render();
        if (this.assist) this.maybeAnalyze();
        break;
      }
      case 'preview': {
        const from = el.dataset.from ?? '';
        const to = el.dataset.to ?? '';
        const target = el.dataset.target ?? '';
        this.preview = [from, to, target].filter(Boolean);
        this.render();
        break;
      }
      case 'undo':
        void this.undo();
        break;
      case 'sound':
        this.sound.toggle();
        this.render();
        break;
      case 'teleport':
        this.mode = 'teleport';
        this.targetPick = null;
        this.render();
        break;
      case 'swap':
        this.mode = 'swap';
        this.targetPick = null;
        this.render();
        break;
      case 'cancel':
        this.mode = 'idle';
        this.targetPick = null;
        this.render();
        break;
      default:
        break;
    }
  }

  private onPanelChange(ev: Event): void {
    const el = (ev.target as HTMLElement | null)?.closest<HTMLSelectElement>('select[data-field]');
    if (!el) return;
    switch (el.dataset.field) {
      case 'opponent':
        this.settings.opponent = el.value as typeof this.settings.opponent;
        if (this.settings.opponent === 'human') return;
        this.render();
        break;
      case 'engine':
        this.settings.engine = el.value;
        break;
      case 'strength':
        this.settings.strength = el.value as typeof this.settings.strength;
        break;
      default:
        break;
    }
  }

  private render(): void {
    if (this.board) this.board.setView(this.view?.state ?? null, this.highlights());
    this.panel.innerHTML = this.html();
  }

  private header(): string {
    // Undo must stay available while the AI is thinking so it can cancel it.
    const canUndo = !!this.view?.canUndo && (!this.busy || this.aiThinking);
    const canSave = !!this.view && !this.busy;
    const canLoad = !this.busy;
    return `
      <header class="hdr">
        <div class="brand"><span class="logo">✦</span> Wizard Boardgame</div>
        <div class="hdr-actions">
          <button class="icon-btn" data-cmd="sound" title="${this.sound.isEnabled() ? 'Mute sound' : 'Enable sound'}" aria-label="${this.sound.isEnabled() ? 'Mute sound' : 'Enable sound'}" aria-pressed="${this.sound.isEnabled()}">${this.sound.isEnabled() ? '🔊' : '🔇'}</button>
          <button class="icon-btn" data-cmd="save" title="Save state" aria-label="Save state" ${
            canSave ? '' : 'disabled'
          }>${SAVE_ICON}</button>
          <button class="icon-btn" data-cmd="load" title="Load state from clipboard" aria-label="Load state from clipboard" ${
            canLoad ? '' : 'disabled'
          }>${LOAD_ICON}</button>
          <button data-cmd="undo" ${canUndo ? '' : 'disabled'}>Undo</button>
        </div>
      </header>`;
  }

  private toastHtml(): string {
    if (!this.toast) return '';
    return `<div class="toast ${this.toastErr ? 'err' : ''}">${esc(this.toast)}</div>`;
  }

  private showToast(message: string, isError = false): void {
    this.toast = message;
    this.toastErr = isError;
    if (this.toastTimer) window.clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => {
      this.toast = '';
      this.toastTimer = 0;
      this.render();
    }, 4500);
    this.render();
  }

  private async saveState(): Promise<void> {
    const view = this.view;
    if (!view || this.busy) return;
    this.busy = true;
    this.render();
    try {
      const res = await api.save(view.id);
      let copied = false;
      try {
        await navigator.clipboard.writeText(res.json);
        copied = true;
      } catch {
        // Clipboard access can be denied; the file is still saved.
      }
      this.busy = false;
      this.showToast(`Saved ${res.path}${copied ? ' · copied to clipboard' : ''}`);
    } catch (err) {
      this.busy = false;
      this.showToast(err instanceof Error ? err.message : String(err), true);
    }
  }

  private async loadState(): Promise<void> {
    if (this.busy) return;
    let text = '';
    try {
      text = await navigator.clipboard.readText();
    } catch {
      this.showToast('Clipboard read failed. Use the save icon to copy a state first.', true);
      return;
    }
    if (!text.trim()) {
      this.showToast('Clipboard is empty.', true);
      return;
    }
    await this.loadSaveText(text);
  }

  private async loadSaveText(text: string): Promise<void> {
    const seq = ++this.opSeq;
    this.aiThinking = false;
    this.selected = null;
    this.mode = 'idle';
    this.targetPick = null;
    this.error = '';
    this.busy = true;
    this.render();
    try {
      const view = await api.load(text);
      if (seq !== this.opSeq) return;
      this.busy = false;
      this.silentNext = true;
      this.acceptView(view);
      // Reflect the loaded match in the game controls and say clearly which
      // mode was restored.
      this.settings.opponent = view.ai.A ? 'aiA' : view.ai.B ? 'aiB' : 'human';
      if (view.engine) this.settings.engine = view.engine;
      if (view.difficulty === 'fast' || view.difficulty === 'strong' || view.difficulty === 'max') {
        this.settings.strength = view.difficulty;
      }
      this.showToast(`State loaded · ${this.modeLabel(view)}`);
    } catch (err) {
      if (seq !== this.opSeq) return;
      this.busy = false;
      this.showToast(err instanceof Error ? err.message : String(err), true);
      this.sound.error();
    }
  }

  private modeLabel(view: MatchView): string {
    const side = view.ai.A ? 'A' : view.ai.B ? 'B' : '';
    if (!side) return 'human vs human';
    const name = this.ais.find((a) => a.id === view.engine)?.name ?? view.engine;
    return `AI ${side}: ${name}`;
  }

  // Applies the selected opponent/engine/strength to the running match,
  // keeping the position and history. Useful after loading a save in a
  // different mode.
  private async applyConfig(): Promise<void> {
    const view = this.view;
    if (!view || this.busy) return;
    const opponent = this.settings.opponent;
    const opts = {
      ai: { A: opponent === 'aiA', B: opponent === 'aiB' },
      engine: this.settings.engine,
      difficulty: this.settings.strength,
    };
    const seq = ++this.opSeq;
    this.aiThinking = false;
    this.error = '';
    this.busy = true;
    this.render();
    try {
      const next = await api.setConfig(view.id, opts);
      if (seq !== this.opSeq) return;
      this.busy = false;
      this.silentNext = true;
      this.acceptView(next);
      this.showToast(`Mode: ${this.modeLabel(next)}`);
    } catch (err) {
      if (seq !== this.opSeq) return;
      this.busy = false;
      this.showToast(err instanceof Error ? err.message : String(err), true);
      this.sound.error();
    }
  }

  private setupHtml(): string {
    const list = this.ais.length
      ? this.ais
      : [{ id: 'ml-r2-v2', name: 'ML r2 v2', notes: '', elo: 1200, games: 0 }];
    const engineOpts = list
      .map(
        (a) =>
          `<option value="${a.id}" ${this.settings.engine === a.id ? 'selected' : ''}>${esc(
            a.name,
          )} (${Math.round(a.elo)})</option>`,
      )
      .join('');
    const sel = (v: string, cur: string) => (v === cur ? 'selected' : '');
    const aiDisabled = this.settings.opponent === 'human' || this.busy;
    return `<section class="setup">
      <div class="section-title">New game</div>
      <label class="field"><span>Opponent</span>
        <select data-field="opponent" ${this.busy ? 'disabled' : ''}>
          <option value="human" ${sel('human', this.settings.opponent)}>Human vs Human</option>
          <option value="aiB" ${sel('aiB', this.settings.opponent)}>Human (A) vs AI (B)</option>
          <option value="aiA" ${sel('aiA', this.settings.opponent)}>AI (A) vs Human (B)</option>
        </select>
      </label>
      <label class="field"><span>Engine</span>
        <select data-field="engine" ${aiDisabled ? 'disabled' : ''}>${engineOpts}</select>
      </label>
      <label class="field"><span>Strength</span>
        <select data-field="strength" ${aiDisabled ? 'disabled' : ''}>
          <option value="fast" ${sel('fast', this.settings.strength)}>Fast</option>
          <option value="strong" ${sel('strong', this.settings.strength)}>Strong</option>
          <option value="max" ${sel('max', this.settings.strength)}>Max</option>
        </select>
      </label>
      <button class="big-btn" data-cmd="new" ${this.busy ? 'disabled' : ''}>Start game</button>
      ${
        this.view
          ? `<button class="big-btn ghost" data-cmd="apply" ${this.busy ? 'disabled' : ''} title="Change the mode of the running game without restarting it">Apply to current</button>`
          : ''
      }
    </section>`;
  }

  private assistHtml(): string {
    const view = this.view;
    if (!view) return '';
    const on = this.assist;
    const toggle = `<button class="assist-toggle ${on ? 'on' : ''}" data-cmd="assist"><span class="big-icon">✨</span> AI assist ${on ? 'on' : 'off'}</button>`;
    if (!on) return `<section class="assist">${toggle}</section>`;

    const humanTurn = !view.state.over && !view.pendingAI && !this.aiThinking;
    if (!humanTurn) {
      return `<section class="assist">${toggle}<div class="hint">Available on your turn.</div></section>`;
    }
    let body = '';
    if (this.analysisBusy) {
      body = '<div class="hint">Analyzing…</div>';
    } else if (this.analysisError) {
      body = `<div class="error">${esc(this.analysisError)}</div>`;
    } else if (this.analysis) {
      body = this.analysisList(this.analysis);
    } else {
      body = '<div class="hint">No analysis yet.</div>';
    }
    return `<section class="assist">${toggle}${body}</section>`;
  }

  private analysisList(a: AnalysisView): string {
    const scored = a.listDepth > 0 && a.moves.length > 0;
    const rows = a.moves.map((m, i) => this.analysisRow(m, i)).join('');
    const engine = a.engineMove?.label ? this.engineRow(a.engineMove) : '';
    const counts = scored
      ? `<div class="hint">${a.within5} within 5 · ${a.within15} within 15 of ${a.analysed} analysed</div>`
      : '';
    return `<div class="assist-head">depth ${a.depth}</div>
      ${engine}
      ${
        rows
          ? `<div class="assist-list">${rows}</div>`
          : '<div class="hint">No alternative move was scored within the budget.</div>'
      }
      ${counts}`;
  }

  private engineRow(m: AnalysisMove): string {
    return `<button class="assist-row engine" data-cmd="preview" data-from="${m.from}" data-to="${m.to}" data-target="${m.target}" title="${esc(m.label)}">
      <span class="rank">★</span>
      <span class="alabel">${esc(m.label)}</span>
      <span class="ascore">engine</span>
    </button>`;
  }

  private analysisRow(m: AnalysisMove, i: number): string {
    const scoreText = m.bound ? `≤${m.score}` : fmtScore(m.score);
    return `<button class="assist-row ${i === 0 ? 'best' : ''} ${m.bound ? 'bound' : ''}" data-cmd="preview" data-from="${m.from}" data-to="${m.to}" data-target="${m.target}" title="${esc(m.label)}">
      <span class="rank">${i + 2}</span>
      <span class="alabel">${esc(m.label)}</span>
      <span class="ascore">${scoreText}</span>
    </button>`;
  }

  private html(): string {
    const head = this.header();
    const err = this.error ? `<div class="error">${esc(this.error)}</div>` : '';
    const view = this.view;
    if (!view) {
      return `${head}${err}${this.toastHtml()}${this.setupHtml()}<div class="empty">${
        this.busy ? 'Starting…' : 'Choose an opponent and press Start game.'
      }</div>`;
    }

    const st = view.state;
    const over = st.over
      ? `<div class="banner over">${
          st.winner === 'draw' ? 'Draw — move limit reached' : `Player ${st.winner} wins!`
        }</div>`
      : '';
    const turnClass = st.turn === 'A' ? 'player-a' : 'player-b';

    const aiSide = view.ai.A ? 'A' : view.ai.B ? 'B' : '';
    const aiMeta = this.ais.find((a) => a.id === view.engine);
    const aiLine = aiSide
      ? `<div class="ai-line">AI (${aiSide}): <b>${esc(aiMeta?.name ?? view.engine)}</b>${
          aiMeta ? ` <span class="muted">Elo ${Math.round(aiMeta.elo)}</span>` : ''
        } <span class="muted">· ${view.difficulty}</span></div>`
      : '';

    const dots = (n: number) =>
      Array.from(
        { length: st.maxTokens },
        (_, i) => `<span class="tk ${i < n ? 'on' : ''}"></span>`,
      ).join('');

    const tokens = `
      <section class="tokens-card">
        <div class="section-title">Tokens</div>
        <div class="token-row ${st.turn === 'A' ? 'active' : ''}">
          <span class="token-player player-a">A</span>
          <span class="token-dots">${dots(st.tokens.A)}</span>
          <span class="token-count">${st.tokens.A}/${st.maxTokens}</span>
        </div>
        <div class="token-row ${st.turn === 'B' ? 'active' : ''}">
          <span class="token-player player-b">B</span>
          <span class="token-dots">${dots(st.tokens.B)}</span>
          <span class="token-count">${st.tokens.B}/${st.maxTokens}</span>
        </div>
      </section>`;

    const status = `
      <section class="status">
        <div class="turn ${turnClass}"><span class="dot"></span> Player ${st.turn} to move</div>
        <div class="phase">Phase: <b>${st.phase}</b> <span class="muted">(${st.phaseIndex + 1}/3)</span></div>
        <div class="ply">Turn ${st.ply + 1}</div>
        ${aiLine}
      </section>`;

    if (st.over) {
      return `${head}${err}${this.toastHtml()}${this.setupHtml()}${over}${status}${tokens}${this.logHtml(view)}`;
    }

    return `${head}${err}${this.toastHtml()}${this.setupHtml()}${over}${status}${tokens}${this.assistHtml()}${this.actionsHtml(view)}${this.logHtml(view)}`;
  }

  private effectsHtml(piece: PieceView): string {
    const st = this.view?.state;
    if (!st) return '';
    const rows: string[] = [];
    if (piece.buffed) {
      const suppressed = piece.type === 'pawn' && piece.nerfed;
      let icon = effectIcon(piece.type, 'buff', piece.owner);
      if (suppressed) icon = icon.replace('class="fx-icon ', 'class="fx-icon fx-suppressed ');
      const by = piece.buffBy
        ? `<span class="fx-by">by Wizard ${esc(piece.buffBy)}</span>`
        : '';
      const supp = suppressed
        ? '<span class="fx-supp">suppressed by nerf</span>'
        : '';
      rows.push(
        `<div class="fx-row buff">${icon}<span class="fx-label">Buffed</span><span class="fx-meaning">${esc(
          buffMeaning(piece.type, st),
        )}</span>${by}${supp}</div>`,
      );
    }
    if (piece.nerfed) {
      const by = piece.nerfBy
        ? `<span class="fx-by">by Wizard ${esc(piece.nerfBy)}</span>`
        : '';
      rows.push(
        `<div class="fx-row nerf">${effectIcon(piece.type, 'nerf', piece.owner)}<span class="fx-label">Nerfed</span><span class="fx-meaning">${esc(
          nerfMeaning(piece.type),
        )}</span>${by}</div>`,
      );
    }
    return rows.length ? `<div class="fx-list">${rows.join('')}</div>` : '';
  }

  private actionsHtml(view: MatchView): string {
    const st = view.state;

    if (view.pendingAI) {
      return `<section class="actions">
        <div class="notice thinking"><span class="spinner"></span> AI is thinking…</div>
        ${!this.busy ? '<button class="big-btn" data-cmd="ai">Retry AI move</button>' : ''}
      </section>`;
    }

    if (st.pending >= 0) {
      const stay = view.legal.find((a) => a.kind === 'freemove' && !a.to);
      return `<section class="actions">
        <div class="notice">${effectIcon('wizard', 'buff', st.turn)} Buffed wizard: teleport to any empty square, or stay.</div>
        ${stay ? `<button class="big-btn" data-cmd="act" data-idx="${stay.index}">Stay in place</button>` : ''}
      </section>`;
    }

    if (this.targetPick) {
      const tp = this.targetPick;
      const pieces = this.view?.state.pieces ?? [];
      return `<section class="actions">
        <div class="notice">Target ${tp.sq} — choose an action.</div>
        <div class="chips">
          ${tp.actions
            .map((a) => {
              const caster = pieces.find((p) => p.id === a.unit);
              const target = pieces.find((p) => p.id === a.target) ?? pieces.find((p) => p.square === tp.sq);
              const who = caster ? `Wizard ${caster.square} · ` : '';
              const suppressed = !!target && target.type === 'pawn' && target.nerfed && a.spell === 'buff';
              let icon = target ? spellIcon(a.spell, target) : '';
              if (suppressed) icon = icon.replace('class="fx-icon ', 'class="fx-icon fx-suppressed ');
              const note = target ? spellNote(a.spell, target, st) : '';
              return `<button class="chip" data-cmd="act" data-idx="${a.index}" title="${esc(
                a.label,
              )}">${icon}${who}${cap(a.spell)}${a.to ? ` → ${a.to}` : ''}${
                note ? ` <span class="fx-note">${esc(note)}</span>` : ''
              }${suppressed ? ' <span class="fx-supp-inline">suppressed by nerf</span>' : ''}</button>`;
            })
            .join('')}
          <button class="chip ghost" data-cmd="cancel">cancel</button>
        </div>
      </section>`;
    }

    if (this.mode === 'teleport') {
      return `<section class="actions">
        <div class="notice">Teleport: click any empty square.</div>
        <button class="chip ghost" data-cmd="cancel">cancel</button>
      </section>`;
    }

    if (this.mode === 'swap') {
      return `<section class="actions">
        <div class="notice">Swap: click an adjacent friendly unit.</div>
        <button class="chip ghost" data-cmd="cancel">cancel</button>
      </section>`;
    }

    const parts: string[] = [];

    const skip = view.legal.find((a) => a.kind === 'skip');
    if (skip) {
      const atMax = st.tokens[st.turn] >= st.maxTokens;
      parts.push(`<button class="big-btn skip" data-cmd="act" data-idx="${skip.index}">
        <span class="big-icon">⏭</span>
        <span>Skip turn</span>
        ${atMax ? '' : '<span class="gain">+1 token</span>'}
      </button>`);
    }

    const canSwap =
      this.selected != null &&
      view.legal.some((a) => a.unit === this.selected && a.kind === 'swap');
    if (canSwap) {
      parts.push(
        `<button class="big-btn" data-cmd="swap"><span class="big-icon">⇄</span> Swap <span class="muted">(adjacent ally)</span></button>`,
      );
    }

    const canTeleport =
      this.selected != null &&
      view.legal.some((a) => a.unit === this.selected && a.kind === 'teleport');
    if (canTeleport) {
      parts.push(
        `<button class="big-btn" data-cmd="teleport"><span class="big-icon">✦</span> Teleport <span class="muted">(${st.teleportCost} tokens)</span></button>`,
      );
    }

    const pass = view.legal.find((a) => a.kind === 'pass');
    if (pass) {
      parts.push(
        `<button class="big-btn" data-cmd="act" data-idx="${pass.index}">Pass</button>`,
      );
    }

    const selectedPiece =
      this.selected != null ? st.pieces.find((p) => p.id === this.selected) : null;
    if (
      selectedPiece?.type === 'cannon' &&
      view.legal.some((a) => a.unit === selectedPiece.id && a.kind === 'turn')
    ) {
      parts.push(
        '<div class="hint">Click the cannon again to turn it. Click a destination to move — the facing wheel opens.</div>',
      );
    }
    if (selectedPiece?.type === 'wizard') {
      parts.push(
        '<div class="hint">Casts only along the 8 lines; the first square toward the target must be empty. Swap exchanges places with an adjacent friendly unit.</div>',
      );
      parts.push(`<div class="fx-legend">
        <span>${effectIcon('pawn', 'buff', 'A')} Buff: side capture / range / free teleport</span>
        <span>${effectIcon('pawn', 'nerf', 'A')} Nerf: retreat / no turn</span>
        <span>${spellIcon('cleanse', { type: 'pawn', owner: 'A' })} Cleanse: removes an effect</span>
      </div>`);
    }

    if (parts.length === 0) {
      parts.push(
        selectedPiece
          ? '<div class="hint">Click a highlighted square to move or capture.</div>'
          : '<div class="empty">Select a unit on the board.</div>',
      );
    }
    const fx = selectedPiece ? this.effectsHtml(selectedPiece) : '';
    return `<section class="actions">${fx}${parts.join('')}</section>`;
  }

  private logHtml(view: MatchView): string {
    const items = view.history
      .map((label, i) => `<li><span class="num">${i + 1}.</span> ${esc(label)}</li>`)
      .reverse()
      .slice(0, 40)
      .join('');
    return `<section class="log-block">
      <div class="section-title">History</div>
      <ol class="log">${items || '<li class="muted">No moves yet.</li>'}</ol>
    </section>`;
  }
}
