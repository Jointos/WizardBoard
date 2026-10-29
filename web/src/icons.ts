import portalSvg from './assets/portal.svg?raw';
import type { PlayerId, StateView, UnitTypeName } from './types';

export type EffectKind = 'buff' | 'nerf';

// The teleport glyph is a simplified twin-portal drawing; currentColor keeps
// the panel icon green while the board uses the derived PNG with a green tint.
const PORTAL = portalSvg
  .replace(/fill="#fff"/g, 'fill="currentColor"')
  .replace(/<svg\b/, '<svg class="fx-svg" aria-hidden="true"');

const SIDE_ARROWS = `
  <path d="M7 8H2M2 8l2-2M2 8l2 2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M9 8h5M14 8l-2-2M14 8l-2 2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>`;

const RETREAT = `
  <path d="M8 2v11M8 13l-3-3M8 13l3-3" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>`;

const RETICLE = `
  <circle cx="5.5" cy="8" r="3" stroke="currentColor" stroke-width="1.3"/>
  <path d="M1.5 8h8M5.5 4v8" stroke="currentColor" stroke-width="1.2"/>
  <circle cx="10.7" cy="8" r="0.8" fill="currentColor"/>
  <circle cx="12.7" cy="8" r="0.8" fill="currentColor"/>
  <circle cx="14.7" cy="8" r="0.8" fill="currentColor"/>`;

const NO_TURN = `
  <path d="M12.2 3.4A5 5 0 1 0 13 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
  <path d="M10.2 1.2 14.6 3.6 10.2 6Z" fill="currentColor" stroke="none"/>
  <path d="M3 3 13 13" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>`;

const MINUS = `<path d="M4 8h8" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>`;

const CLEANSE = `
  <path d="M8 2.2l1.4 4.4L14 8l-4.6 1.4L8 13.8 6.6 9.4 2 8l4.6-1.4z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>`;

function glyph(inner: string, color: EffectKind, extra = ''): string {
  return `<span class="fx-icon fx-${color}${extra ? ` ${extra}` : ''}"><svg class="fx-svg" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">${inner}</svg></span>`;
}

/** The effect icon for a piece type and effect. */
export function effectIcon(
  type: UnitTypeName,
  effect: EffectKind,
  owner: PlayerId,
): string {
  if (effect === 'buff') {
    if (type === 'wizard') return `<span class="fx-icon fx-buff">${PORTAL}</span>`;
    return glyph(type === 'cannon' ? RETICLE : SIDE_ARROWS, 'buff');
  }
  if (type === 'cannon') return glyph(NO_TURN, 'nerf');
  if (type === 'pawn') {
    // The retreat direction is owner-relative (A retreats down, B up).
    return glyph(RETREAT, 'nerf', owner === 'B' ? 'flip-y' : '');
  }
  return glyph(MINUS, 'nerf');
}

/** What a buff means for this piece in the current ruleset. */
export function buffMeaning(type: UnitTypeName, state: StateView): string {
  switch (type) {
    case 'pawn':
      return state.pawnBuffSideCapture ? 'can capture sideways' : 'buffed';
    case 'cannon':
      return state.cannonBuffRange > 0 ? `shoots at range ${state.cannonBuffRange}` : 'extended range';
    case 'wizard':
      return 'free teleport after casting';
  }
}

/** What a nerf means for this piece. */
export function nerfMeaning(type: UnitTypeName): string {
  switch (type) {
    case 'pawn':
      return 'must retreat';
    case 'cannon':
      return 'cannot turn';
    default:
      return 'nerfed';
  }
}

/** Icon for a cast option on the target piece. */
export function spellIcon(
  spell: string,
  target: { type: UnitTypeName; owner: PlayerId },
): string {
  if (spell === 'cleanse') return glyph(CLEANSE, 'buff');
  return effectIcon(target.type, spell === 'nerf' ? 'nerf' : 'buff', target.owner);
}

/** Short explanation of what a cast option does to the target. */
export function spellNote(
  spell: string,
  target: { type: UnitTypeName; buffed: boolean; nerfed: boolean },
  state: StateView,
): string {
  if (spell === 'cleanse') {
    if (target.nerfed) return `removes nerf (${nerfMeaning(target.type)})`;
    if (target.buffed) return `removes buff (${buffMeaning(target.type, state)})`;
    return 'removes effects';
  }
  if (spell === 'nerf') return nerfMeaning(target.type);
  return buffMeaning(target.type, state);
}
