export type PlayerId = 'A' | 'B';
export type UnitTypeName = 'pawn' | 'cannon' | 'wizard';
export type SpellName = 'none' | 'buff' | 'nerf' | 'cleanse';
export type KindName =
  | 'move'
  | 'capture'
  | 'shoot'
  | 'turn'
  | 'moveturn'
  | 'cast'
  | 'skip'
  | 'teleport'
  | 'freemove'
  | 'pass'
  | 'swap';

export interface PieceView {
  id: number;
  type: UnitTypeName;
  owner: PlayerId;
  square: string;
  facing: string;
  acts: number;
  buffed: boolean;
  nerfed: boolean;
  buffBy: string;
  nerfBy: string;
}

export interface TokensView {
  A: number;
  B: number;
}

export interface StateView {
  turn: PlayerId;
  phase: UnitTypeName;
  phaseIndex: number;
  pieces: PieceView[];
  tokens: TokensView;
  pending: number;
  ply: number;
  over: boolean;
  winner: '' | PlayerId | 'draw';
  teleportCost: number;
  maxTokens: number;
  cannonRange: number;
  cannonBuffRange: number;
  pawnBuffSideCapture: boolean;
}

export interface ActionView {
  index: number;
  kind: KindName;
  unit: number;
  from: string;
  to: string;
  target: number;
  targetSquare: string;
  spell: SpellName;
  facing: string;
  label: string;
}

export interface LastActionView {
  kind: KindName;
  unit: number;
  unitType: UnitTypeName | '';
  from: string;
  to: string;
  target: number;
  targetSquare: string;
  targetType: UnitTypeName | '';
  spell: SpellName;
  facing: string;
  label: string;
}

export interface MatchView {
  id: string;
  state: StateView;
  legal: ActionView[];
  history: string[];
  canUndo: boolean;
  last: string[];
  lastAction?: LastActionView | null;
  ai: AISeats;
  engine: string;
  difficulty: string;
  pendingAI: boolean;
}

export interface Highlights {
  move: string[];
  target: string[];
  ally: string[];
  selectable: string[];
  selected: string;
  last: string[];
  rays: string[];
  preview: string[];
}

export interface AnalysisMove {
  index: number;
  label: string;
  kind: string;
  from: string;
  to: string;
  target: string;
  score: number;
  bound: boolean;
}

export interface AnalysisView {
  engine: string;
  depth: number;
  listDepth: number;
  ply: number;
  best: number;
  candidates: number;
  analysed: number;
  within5: number;
  within15: number;
  engineMove: AnalysisMove;
  moves: AnalysisMove[];
}

export interface AISeats {
  A: boolean;
  B: boolean;
}

export interface AIInfo {
  id: string;
  name: string;
  notes: string;
  elo: number;
  games: number;
}

export interface CreateOptions {
  ai?: AISeats;
  engine?: string;
  difficulty?: string;
}
