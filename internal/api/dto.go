package api

import (
	"wizardboardgame/internal/game"
	"wizardboardgame/internal/match"
)

// TokensDTO is the per-player token counter.
type TokensDTO struct {
	A int `json:"A"`
	B int `json:"B"`
}

// PieceDTO is the wire representation of one living piece.
type PieceDTO struct {
	ID     int    `json:"id"`
	Type   string `json:"type"`
	Owner  string `json:"owner"`
	Square string `json:"square"`
	Facing string `json:"facing"`
	Acts   int    `json:"acts"`
	Buffed bool   `json:"buffed"`
	Nerfed bool   `json:"nerfed"`
	// BuffBy / NerfBy are the squares of the wizards that applied the effect
	// ("" when absent). This lets the client show who cast what.
	BuffBy string `json:"buffBy"`
	NerfBy string `json:"nerfBy"`
}

// StateDTO is the wire representation of the game state.
type StateDTO struct {
	Turn                string     `json:"turn"`
	Phase               string     `json:"phase"`
	PhaseIndex          int        `json:"phaseIndex"`
	Pieces              []PieceDTO `json:"pieces"`
	Tokens              TokensDTO  `json:"tokens"`
	Pending             int        `json:"pending"`
	Ply                 int        `json:"ply"`
	Over                bool       `json:"over"`
	Winner              string     `json:"winner"`
	TeleportCost        int        `json:"teleportCost"`
	MaxTokens           int        `json:"maxTokens"`
	CannonRange         int        `json:"cannonRange"`
	CannonBuffRange     int        `json:"cannonBuffRange"`
	PawnBuffSideCapture bool       `json:"pawnBuffSideCapture"`
}

// ActionDTO is a legal action, addressed by Index when played.
type ActionDTO struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	Unit         int    `json:"unit"`
	From         string `json:"from"`
	To           string `json:"to"`
	Target       int    `json:"target"`
	TargetSquare string `json:"targetSquare"`
	Spell        string `json:"spell"`
	Facing       string `json:"facing"`
	Label        string `json:"label"`
}

// LastActionDTO is the action that produced the current position. The client
// uses it to play move/cast/takedown sounds for both human and AI moves.
type LastActionDTO struct {
	Kind         string `json:"kind"`
	Unit         int    `json:"unit"`
	UnitType     string `json:"unitType"`
	From         string `json:"from"`
	To           string `json:"to"`
	Target       int    `json:"target"`
	TargetSquare string `json:"targetSquare"`
	TargetType   string `json:"targetType"`
	Spell        string `json:"spell"`
	Facing       string `json:"facing"`
	Label        string `json:"label"`
}

// AISeatsDTO says which sides are controlled by the engine.
type AISeatsDTO struct {
	A bool `json:"A"`
	B bool `json:"B"`
}

// AIInfoDTO is one entry of GET /api/ai.
type AIInfoDTO struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Notes string  `json:"notes"`
	Elo   float64 `json:"elo"`
	Games int     `json:"games"`
}

// MatchDTO is everything the client needs to render a position.
type MatchDTO struct {
	ID         string         `json:"id"`
	State      StateDTO       `json:"state"`
	Legal      []ActionDTO    `json:"legal"`
	History    []string       `json:"history"`
	CanUndo    bool           `json:"canUndo"`
	Last       []string       `json:"last"`
	LastAction *LastActionDTO `json:"lastAction,omitempty"`
	AI         AISeatsDTO     `json:"ai"`
	Engine     string         `json:"engine"`
	Difficulty string         `json:"difficulty"`
	PendingAI  bool           `json:"pendingAI"`
}

func stateDTO(s *game.State) StateDTO {
	pieces := make([]PieceDTO, 0, game.MaxPieces)
	for id := 0; id < game.MaxPieces; id++ {
		p := s.Pieces[id]
		if !p.Alive {
			continue
		}
		pid := game.PieceID(id)
		buffBy, nerfBy := "", ""
		if c := s.BuffCaster(pid); c != game.NoPiece {
			buffBy = s.Pieces[c].Sq.String()
		}
		// Wizards cannot be nerfed in the current ruleset; never display it.
		nerfed := s.IsNerfed(pid)
		if p.Type == game.UnitWizard && !s.Rules.WizardNerf {
			nerfed = false
		} else if c := s.NerfCaster(pid); c != game.NoPiece {
			nerfBy = s.Pieces[c].Sq.String()
		}
		pieces = append(pieces, PieceDTO{
			ID:     id,
			Type:   p.Type.String(),
			Owner:  p.Owner.String(),
			Square: p.Sq.String(),
			Facing: p.Facing.String(),
			Acts:   int(p.Acts % 3),
			Buffed: s.IsBuffed(pid),
			Nerfed: nerfed,
			BuffBy: buffBy,
			NerfBy: nerfBy,
		})
	}
	winner := ""
	switch {
	case !s.Over:
		winner = ""
	case s.Winner == game.NoPlayer:
		winner = "draw"
	default:
		winner = s.Winner.String()
	}
	pending := -1
	if s.Pending != game.NoPiece {
		pending = int(s.Pending)
	}
	return StateDTO{
		Turn:                s.Turn.String(),
		Phase:               s.Phase(s.Turn).String(),
		PhaseIndex:          int(s.PhaseIndex(s.Turn)),
		Pieces:              pieces,
		Tokens:              TokensDTO{A: int(s.Token[game.PlayerA]), B: int(s.Token[game.PlayerB])},
		Pending:             pending,
		Ply:                 int(s.Ply),
		Over:                s.Over,
		Winner:              winner,
		TeleportCost:        int(s.Rules.TeleportCost),
		MaxTokens:           int(s.Rules.MaxTokens),
		CannonRange:         s.Rules.CannonRange,
		CannonBuffRange:     s.Rules.CannonBuffRange,
		PawnBuffSideCapture: s.Rules.PawnBuffSideCapture,
	}
}

func actionDTOs(s *game.State, legal []game.Action) []ActionDTO {
	out := make([]ActionDTO, 0, len(legal))
	for i, a := range legal {
		target := -1
		targetSquare := ""
		if a.Target != game.NoPiece && a.Target < game.MaxPieces {
			target = int(a.Target)
			if s.Pieces[a.Target].Alive {
				targetSquare = s.Pieces[a.Target].Sq.String()
			}
		}
		to := ""
		if a.To != game.NoSquare {
			to = a.To.String()
		}
		facing := ""
		if a.Kind == game.ActTurn || a.Kind == game.ActMoveTurn {
			facing = a.Facing.String()
		}
		out = append(out, ActionDTO{
			Index:        i,
			Kind:         a.Kind.String(),
			Unit:         int(a.Unit),
			From:         a.From.String(),
			To:           to,
			Target:       target,
			TargetSquare: targetSquare,
			Spell:        a.Spell.String(),
			Facing:       facing,
			Label:        s.Describe(a),
		})
	}
	return out
}

func matchDTO(snap match.Snapshot) MatchDTO {
	history := make([]string, len(snap.Log))
	for i, e := range snap.Log {
		history[i] = e.Label
	}
	last := make([]string, 0, 2)
	if n := len(snap.Log); n > 0 {
		a := snap.Log[n-1].Action
		if a.From != game.NoSquare {
			last = append(last, a.From.String())
		}
		if a.To != game.NoSquare && a.To != a.From {
			last = append(last, a.To.String())
		}
	}
	var lastAction *LastActionDTO
	if n := len(snap.Log); n > 0 {
		e := snap.Log[n-1]
		a := e.Action
		la := &LastActionDTO{
			Kind:   a.Kind.String(),
			Unit:   int(a.Unit),
			From:   squareName(a.From),
			To:     squareName(a.To),
			Target: int(a.Target),
			Spell:  a.Spell.String(),
			Facing: a.Facing.String(),
			Label:  e.Label,
		}
		if a.Unit < game.MaxPieces {
			la.UnitType = snap.State.Pieces[a.Unit].Type.String()
		}
		if a.Target != game.NoPiece && a.Target < game.MaxPieces {
			target := snap.State.Pieces[a.Target]
			la.TargetType = target.Type.String()
			la.TargetSquare = squareName(target.Sq)
		}
		lastAction = la
	}
	return MatchDTO{
		ID:         snap.ID,
		State:      stateDTO(&snap.State),
		Legal:      actionDTOs(&snap.State, snap.Legal),
		History:    history,
		CanUndo:    snap.CanUndo,
		Last:       last,
		LastAction: lastAction,
		AI:         AISeatsDTO{A: snap.AI[game.PlayerA], B: snap.AI[game.PlayerB]},
		Engine:     snap.EngineID,
		Difficulty: snap.Difficulty,
		PendingAI:  snap.PendingAI,
	}
}

func squareName(sq game.Square) string {
	if sq == game.NoSquare {
		return ""
	}
	return sq.String()
}
