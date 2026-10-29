package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/arena"
	"wizardboardgame/internal/game"
	"wizardboardgame/internal/match"
	"wizardboardgame/internal/save"
	"wizardboardgame/internal/webui"
)

// Server exposes the match service over JSON/HTTP and serves the web client.
type Server struct {
	svc      *match.Service
	rankPath string
	saves    *save.Store
}

// New builds the complete HTTP handler. rankPath points at rankings.json and
// savesDir at the debug save folder; empty strings use the defaults.
func New(svc *match.Service, rankPath, savesDir string) http.Handler {
	s := &Server{svc: svc, rankPath: rankPath, saves: save.NewStore(savesDir)}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/ai", s.listAI)
	mux.HandleFunc("POST /api/matches", s.create)
	mux.HandleFunc("GET /api/matches/{id}", s.get)
	mux.HandleFunc("POST /api/matches/{id}/actions", s.play)
	mux.HandleFunc("POST /api/matches/{id}/ai", s.aiMove)
	mux.HandleFunc("POST /api/matches/{id}/undo", s.undo)
	mux.HandleFunc("POST /api/matches/{id}/random", s.random)
	mux.HandleFunc("POST /api/matches/{id}/config", s.updateConfig)
	mux.HandleFunc("POST /api/matches/{id}/analyze", s.analyze)
	mux.HandleFunc("POST /api/matches/{id}/save", s.saveMatch)
	mux.HandleFunc("POST /api/saves/load", s.loadSave)
	mux.Handle("/", webui.Handler())

	return mux
}

type createRequest struct {
	AI struct {
		A bool `json:"A"`
		B bool `json:"B"`
	} `json:"ai"`
	Engine     string `json:"engine"`
	Difficulty string `json:"difficulty"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if r.Body != nil {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	cfg := match.Config{
		AI:         [2]bool{req.AI.A, req.AI.B},
		EngineID:   req.Engine,
		Difficulty: req.Difficulty,
	}
	writeJSON(w, http.StatusOK, matchDTO(s.svc.Create(cfg)))
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.Get(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

func (s *Server) listAI(w http.ResponseWriter, _ *http.Request) {
	ranks, err := arena.Load(s.rankPath)
	if err != nil {
		log.Printf("load rankings: %v", err)
		ranks = arena.NewRankings()
	}
	out := make([]AIInfoDTO, 0, len(ranks.Engines))
	for _, info := range ai.List() {
		elo := float64(arena.StartElo)
		games := 0
		if er := ranks.Engines[info.ID]; er != nil {
			elo = er.Elo
			games = er.Games
		}
		out = append(out, AIInfoDTO{
			ID:    info.ID,
			Name:  info.Name,
			Notes: info.Notes,
			Elo:   elo,
			Games: games,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type playRequest struct {
	Index int `json:"index"`
}

func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	var req playRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	snap, err := s.svc.Play(r.PathValue("id"), req.Index)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

func (s *Server) aiMove(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.PlayAI(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

func (s *Server) undo(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.Undo(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

func (s *Server) random(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.PlayRandom(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

// analysisNodes is the deterministic budget for one AI-assist analysis. It is a
// node budget (not a time limit) so repeated analyses of the same position
// return the same ranking.
const (
	analysisNodes = 5_000_000
	analysisTime  = 12 * time.Second
	analysisDepth = 8
	analysisTop   = 10
)

type analysisMoveDTO struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	From   string `json:"from"`
	To     string `json:"to"`
	Target string `json:"target"`
	Score  int    `json:"score"`
	Bound  bool   `json:"bound"`
}

// analyze runs the engine on the current position without touching the match.
// It feeds the balance-analysis panel in the client.
func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	snap, err := s.svc.Get(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if snap.State.Over {
		writeError(w, http.StatusBadRequest, "game is over")
		return
	}
	eng, ok := ai.New(snap.EngineID, ai.Config{MaxNodes: analysisNodes, MaxTime: analysisTime, MaxDepth: analysisDepth, DisableFilter: true})
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown engine")
		return
	}
	an, ok := eng.(ai.Analyzer)
	if !ok {
		writeError(w, http.StatusBadRequest, "analysis is not supported by this engine")
		return
	}
	res := an.Analyze(snap.State, snap.Legal)

	toDTO := func(sa ai.ScoredAction) (analysisMoveDTO, bool) {
		idx := -1
		for j, a := range snap.Legal {
			if a == sa.Action {
				idx = j
				break
			}
		}
		if idx < 0 {
			return analysisMoveDTO{}, false
		}
		target := ""
		if sa.Action.Target != game.NoPiece && sa.Action.Target < game.MaxPieces {
			if p := snap.State.Pieces[sa.Action.Target]; p.Alive {
				target = p.Sq.String()
			}
		}
		to := ""
		if sa.Action.To != game.NoSquare {
			to = sa.Action.To.String()
		}
		return analysisMoveDTO{
			Index:  idx,
			Label:  snap.State.Describe(sa.Action),
			Kind:   sa.Action.Kind.String(),
			From:   sa.Action.From.String(),
			To:     to,
			Target: target,
			Score:  sa.Score,
			Bound:  sa.Bound,
		}, true
	}

	engineMove, _ := toDTO(res.Engine)

	moves := make([]analysisMoveDTO, 0, analysisTop)
	for _, sa := range res.Moves {
		if len(moves) >= analysisTop {
			break
		}
		// The engine move is shown separately, at the top of the panel.
		if sa.Action == res.Engine.Action {
			continue
		}
		if m, ok := toDTO(sa); ok {
			moves = append(moves, m)
		}
	}
	best := 0
	if res.ListDepth > 0 {
		best = res.Engine.Score
	}
	// Count the good moves over the analysed set (the engine move plus the
	// scored alternatives). Bound moves are outside the analysed band.
	within5, within15 := 0, 0
	if res.ListDepth > 0 {
		within5, within15 = 1, 1 // the engine move is the best
		for _, sa := range res.Moves {
			if sa.Bound {
				continue
			}
			d := best - sa.Score
			if d <= 5 {
				within5++
			}
			if d <= 15 {
				within15++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"engine":     snap.EngineID,
		"depth":      res.Depth,
		"listDepth":  res.ListDepth,
		"ply":        int(snap.State.Ply),
		"best":       best,
		"candidates": len(snap.Legal),
		"analysed":   len(res.Moves) + 1,
		"within5":    within5,
		"within15":   within15,
		"engineMove": engineMove,
		"moves":      moves,
	})
}

// updateConfig changes the mode of a running match (AI seats, engine,
// difficulty) without restarting it.
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cfg := match.Config{
		AI:         [2]bool{req.AI.A, req.AI.B},
		EngineID:   req.Engine,
		Difficulty: req.Difficulty,
	}
	snap, err := s.svc.UpdateConfig(r.PathValue("id"), cfg)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

// saveMatch writes a debug save into the save folder and returns its name plus
// the JSON (the client copies it to the clipboard).
func (s *Server) saveMatch(w http.ResponseWriter, r *http.Request) {
	exp, err := s.svc.Export(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	f := save.Encode(exp)
	name, err := s.saves.Save(f)
	if err != nil {
		log.Printf("save match: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save state")
		return
	}
	path := filepath.ToSlash(filepath.Join(s.saves.Dir, name))
	log.Printf("saved match %s -> %s", exp.ID, path)
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "path": path, "json": string(data)})
}

// loadSave restores a match from a save document (usually pasted from the
// clipboard) and returns the new match.
func (s *Server) loadSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "save is too large")
		return
	}
	f, err := save.Decode(body)
	if err != nil {
		s.fail(w, err)
		return
	}
	exp, err := f.ToExport()
	if err != nil {
		s.fail(w, err)
		return
	}
	snap, err := s.svc.Import(exp)
	if err != nil {
		s.fail(w, err)
		return
	}
	log.Printf("loaded save -> match %s (ai A=%v B=%v, engine=%s, difficulty=%s)",
		snap.ID, snap.AI[0], snap.AI[1], snap.EngineID, snap.Difficulty)
	writeJSON(w, http.StatusOK, matchDTO(snap))
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, match.ErrNotFound):
		writeError(w, http.StatusNotFound, "match not found")
	case errors.Is(err, match.ErrBadAction):
		writeError(w, http.StatusBadRequest, "action index out of range")
	case errors.Is(err, match.ErrNothingToUndo):
		writeError(w, http.StatusBadRequest, "nothing to undo")
	case errors.Is(err, match.ErrAITurn):
		writeError(w, http.StatusConflict, "it is the AI's turn")
	case errors.Is(err, match.ErrStale):
		writeError(w, http.StatusConflict, "the match changed while the AI was thinking")
	case errors.Is(err, match.ErrBadSave):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, match.ErrBadConfig):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, game.ErrIllegal), errors.Is(err, game.ErrGameOver):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
