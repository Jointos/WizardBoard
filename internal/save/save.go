// Package save implements the debug save format: a versioned JSON document
// holding the replayable action log, the match configuration and the rules.
package save

import (
	"encoding/json"
	"fmt"
	"time"

	"wizardboardgame/internal/game"
	"wizardboardgame/internal/match"
)

const (
	FormatName = "wizard-boardgame-save"
	Version    = 1
)

// File is one saved match.
type File struct {
	Format    string    `json:"format"`
	Version   int       `json:"version"`
	SavedAt   time.Time `json:"savedAt"`
	Match     MatchData `json:"match"`
	StateHash uint64    `json:"stateHash"`
}

// MatchData is the replayable part of a save.
type MatchData struct {
	MatchID    string       `json:"matchId"`
	Engine     string       `json:"engine"`
	Difficulty string       `json:"difficulty"`
	AI         [2]bool      `json:"ai"`
	Seed       int64        `json:"seed"`
	Rules      game.Rules   `json:"rules"`
	Actions    []ActionData `json:"actions"`
	History    []string     `json:"history"`
}

// ActionData is the JSON form of one action (readable, not the raw enum).
type ActionData struct {
	Kind   string `json:"kind"`
	Unit   int    `json:"unit"`
	From   string `json:"from"`
	To     string `json:"to"`
	Target int    `json:"target"`
	Spell  string `json:"spell"`
	Facing string `json:"facing"`
}

// Encode builds a save file from a match export.
func Encode(exp match.Export) File {
	actions := make([]ActionData, 0, len(exp.Log))
	history := make([]string, 0, len(exp.Log))
	for _, e := range exp.Log {
		actions = append(actions, actionToData(e.Action))
		history = append(history, e.Label)
	}
	rules := game.DefaultRules()
	if exp.Rules != nil {
		rules = exp.Rules.Clone()
	}
	return File{
		Format:  FormatName,
		Version: Version,
		SavedAt: time.Now(),
		Match: MatchData{
			MatchID:    exp.ID,
			Engine:     exp.Engine,
			Difficulty: exp.Difficulty,
			AI:         exp.AI,
			Seed:       exp.Seed,
			Rules:      *rules,
			Actions:    actions,
			History:    history,
		},
		StateHash: exp.StateHash,
	}
}

// Decode parses and validates a save document.
func Decode(data []byte) (File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("%w: %v", match.ErrBadSave, err)
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

// Validate checks the format marker and version.
func (f File) Validate() error {
	if f.Format != FormatName {
		return fmt.Errorf("%w: unknown format %q", match.ErrBadSave, f.Format)
	}
	if f.Version != Version {
		return fmt.Errorf("%w: unsupported version %d", match.ErrBadSave, f.Version)
	}
	return nil
}

// ToExport converts a save file back into a match export.
func (f File) ToExport() (match.Export, error) {
	if err := f.Validate(); err != nil {
		return match.Export{}, err
	}
	log := make([]match.Entry, 0, len(f.Match.Actions))
	for i, ad := range f.Match.Actions {
		a, err := actionFromData(ad)
		if err != nil {
			return match.Export{}, fmt.Errorf("%w: action %d: %v", match.ErrBadSave, i, err)
		}
		log = append(log, match.Entry{Action: a})
	}
	rules := f.Match.Rules
	if rules == (game.Rules{}) {
		rules = *game.DefaultRules()
	}
	return match.Export{
		ID:         f.Match.MatchID,
		Engine:     f.Match.Engine,
		Difficulty: f.Match.Difficulty,
		AI:         f.Match.AI,
		Seed:       f.Match.Seed,
		Rules:      &rules,
		Log:        log,
		StateHash:  f.StateHash,
	}, nil
}

func actionToData(a game.Action) ActionData {
	target := -1
	if a.Target != game.NoPiece {
		target = int(a.Target)
	}
	return ActionData{
		Kind:   a.Kind.String(),
		Unit:   int(a.Unit),
		From:   a.From.String(),
		To:     a.To.String(),
		Target: target,
		Spell:  a.Spell.String(),
		Facing: a.Facing.String(),
	}
}

func actionFromData(d ActionData) (game.Action, error) {
	kind, ok := kindByName[d.Kind]
	if !ok {
		return game.Action{}, fmt.Errorf("unknown action kind %q", d.Kind)
	}
	spell, ok := spellByName[d.Spell]
	if !ok {
		return game.Action{}, fmt.Errorf("unknown spell %q", d.Spell)
	}
	facing, ok := dirByName[d.Facing]
	if !ok {
		return game.Action{}, fmt.Errorf("unknown facing %q", d.Facing)
	}
	from, err := parseSquare(d.From)
	if err != nil {
		return game.Action{}, fmt.Errorf("from: %v", err)
	}
	to, err := parseSquare(d.To)
	if err != nil {
		return game.Action{}, fmt.Errorf("to: %v", err)
	}
	if d.Unit < 0 || (d.Unit >= game.MaxPieces && d.Unit != int(game.NoPiece)) {
		return game.Action{}, fmt.Errorf("unit %d out of range", d.Unit)
	}
	target := game.NoPiece
	if d.Target >= 0 {
		if d.Target >= game.MaxPieces {
			return game.Action{}, fmt.Errorf("target %d out of range", d.Target)
		}
		target = game.PieceID(d.Target)
	}
	return game.Action{
		Kind:   kind,
		Unit:   game.PieceID(d.Unit),
		From:   from,
		To:     to,
		Target: target,
		Spell:  spell,
		Facing: facing,
	}, nil
}

func parseSquare(s string) (game.Square, error) {
	if s == "" || s == "-" {
		return game.NoSquare, nil
	}
	sq, ok := game.ParseSquare(s)
	if !ok {
		return game.NoSquare, fmt.Errorf("bad square %q", s)
	}
	return sq, nil
}

var kindByName = map[string]game.ActionKind{
	"move":     game.ActMove,
	"capture":  game.ActCapture,
	"shoot":    game.ActShoot,
	"turn":     game.ActTurn,
	"moveturn": game.ActMoveTurn,
	"cast":     game.ActCast,
	"skip":     game.ActSkip,
	"teleport": game.ActTeleport,
	"freemove": game.ActFreeMove,
	"pass":     game.ActPass,
}

var spellByName = map[string]game.Spell{
	"none":    game.SpellNone,
	"buff":    game.SpellBuff,
	"nerf":    game.SpellNerf,
	"cleanse": game.SpellCleanse,
}

var dirByName = map[string]game.Dir{
	"N":  game.North,
	"NE": game.NorthEast,
	"E":  game.East,
	"SE": game.SouthEast,
	"S":  game.South,
	"SW": game.SouthWest,
	"W":  game.West,
	"NW": game.NorthWest,
}
