# Wizard Boardgame - developer shortcuts.
# Run `make` or `make help` to list the targets.
#
# Ports are fixed: backend http://localhost:8080, frontend http://localhost:5173
# (the Vite dev server proxies /api to :8080).

PORT ?= 8080

ifeq ($(OS),Windows_NT)
BIN   := bin/wizard-server.exe
KILL  := powershell -NoProfile -ExecutionPolicy Bypass -File scripts/kill-port.ps1 -Port $(PORT)
CLEAN := powershell -NoProfile -Command "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue 'bin'"
else
BIN   := bin/wizard-server
KILL  := lsof -ti:$(PORT) | xargs -r kill -9
CLEAN := rm -rf bin
endif

.DEFAULT_GOAL := help
.PHONY: help backend frontend kill build test selfplay arena arena-eval arena-eval-all dataset-r2 train-r2 ml-r2-eval dataset-r2-v2 train-r2-v2 ml-r2-v2-selfplay dataset-r2-v3 train-r2-v3 ml-r2-arena ranks balance balance-selfplay arena-report clean

help:
	@echo "Usage: make <target>"
	@echo "  backend    build and run the Go server on http://localhost:8080"
	@echo "  frontend   install deps and run the Vite dev server on http://localhost:5173"
	@echo "  kill       kill whatever listens on port $(PORT)  (e.g. make kill PORT=8099)"
	@echo "  build      production build: web bundle + single binary (UI embedded)"
	@echo "  test       go vet + go test ./..."
	@echo "  selfplay   alphabeta-r2-v1 vs ml-r2-v2 self-play"
	@echo "  arena      run a full engine tournament and update the Elo table"
	@echo "  arena-eval evaluate new engines (depth 6) + update ELO and stats JSON"
	@echo "  arena-eval-all  full round-robin (depth 6) + update ELO and stats JSON"
	@echo "  dataset-r2 generate r2 alphabeta self-play data for ml-r2-v1"
	@echo "  train-r2   fit the ml-r2-v1 model (24 features, 64 hidden)"
	@echo "  ml-r2-eval arena ml-r2-v1 vs alphabeta-r2-v1 (30 games)"
	@echo "  dataset-r2-v2 generate score-labelled data for ml-r2-v2"
	@echo "  train-r2-v2   fit the ml-r2-v2 model (43 features, 64 hidden)"
	@echo "  ml-r2-v2-selfplay  100 self-play games + stats JSON"
	@echo "  dataset-r2-v3 generate richer-feature data for ml-r2-v3"
	@echo "  train-r2-v3   fit the ml-r2-v3 model (50 features, 96 hidden)"
	@echo "  ml-r2-arena   v2/v3/v4, 50 paired games per pairing"
	@echo "  balance    ml-r2-v2 vs alphabeta-r2-v1, stats only (no Elo)"
	@echo "  balance-selfplay  ml-r2-v2 vs itself (pure first-move / side balance)"
	@echo "  arena-report  render the HTML report from a stats JSON (IN=, OUT=)"
	@echo "  ranks      print the engine ranking table"
	@echo "  clean      remove the compiled binary"

# Terminal 1
backend:
	go build -o $(BIN) ./cmd/server
	$(BIN) -addr :8080

# Terminal 2
frontend:
	cd web && npm install --no-audit --no-fund
	cd web && npm run dev

kill:
	@$(KILL)

build:
	cd web && npm install --no-audit --no-fund
	cd web && npm run build
	go build -o $(BIN) ./cmd/server
	@echo "Built $(BIN) with the embedded web client."

test:
	go vet ./...
	go test ./...

selfplay:
	go run ./cmd/selfplay -games 100 -a alphabeta-r2-v1 -b ml-r2-v2 -nodes 30000

arena:
	go run ./cmd/arena -tournament -games 10 -depth 6 -time 1500ms -nodes 0

arena-eval:
	go run ./cmd/arena-eval -games 10 -depth 6 -time 1500ms -nodes 0

arena-eval-all:
	go run ./cmd/arena-eval -all -games 10 -depth 6 -time 1500ms -nodes 0

# r2 ruleset (wizard swap): one alphabeta baseline + one ML engine.
dataset-r2:
	go run ./cmd/dataset_r2 -games 500 -time 500ms -depth 6 -workers 8 -seed 1 -out data/datasets/r2_ml_v1.jsonl

train-r2:
	go run ./cmd/train -in data/datasets/r2_ml_v1.jsonl -out internal/ai/ml_r2_v1/weights.json -model mlp -target outcome -hidden 64 -epochs 40

ml-r2-eval:
	go run ./cmd/arena-eval -pairs "ml-r2-v1|alphabeta-r2-v1" -games 30 -depth 6 -time 1500ms -nodes 0 -no-elo -out data/balance/r2_ml_v1_vs_alpha.json

dataset-r2-v2:
	go run ./cmd/dataset_r2_v2 -games 300 -time 300ms -depth 6 -workers 8 -seed 11 -out data/datasets/r2_ml_v2.jsonl

train-r2-v2:
	go run ./cmd/train -in data/datasets/r2_ml_v2.jsonl -out internal/ai/ml_r2_v2/weights.json -model mlp -target score -hidden 64 -epochs 80 -lr 0.001

ml-r2-v2-selfplay:
	go run ./cmd/arena-eval -pairs "ml-r2-v2|ml-r2-v2" -games 100 -time 1500ms -depth 6 -nodes 0 -no-elo -paired -stamp -out data/balance/balance_mlr2v2_selfplay.json

dataset-r2-v3:
	go run ./cmd/dataset_r2_v3 -games 500 -time 500ms -depth 6 -workers 8 -seed 21 -out data/datasets/r2_ml_v3.jsonl

train-r2-v3:
	go run ./cmd/train -in data/datasets/r2_ml_v3.jsonl -out internal/ai/ml_r2_v3/weights.json -model mlp -target score -hidden 96 -epochs 80 -lr 0.001

ml-r2-arena:
	go run ./cmd/arena-eval -pairs "ml-r2-v2|ml-r2-v3,ml-r2-v2|ml-r2-v4,ml-r2-v3|ml-r2-v4" -games 50 -time 1500ms -depth 6 -nodes 0 -paired -best-move=false

# Statistics-only balance runs. They never touch data/rankings.json.
# The -stamp flag appends the local date/time to the output name.
balance:
	go run ./cmd/arena-eval -pairs "ml-r2-v2|alphabeta-r2-v1" -games 20 -depth 6 -time 1500ms -nodes 0 -no-elo -paired -stamp -out data/balance/balance_mlr2v2.json

balance-selfplay:
	go run ./cmd/arena-eval -pairs "ml-r2-v2|ml-r2-v2" -games 100 -depth 6 -time 1500ms -nodes 0 -no-elo -paired -stamp -out data/balance/balance_mlr2v2_selfplay.json

# Render the HTML report. IN and OUT are paths; the report is always a
# separate call, never generated during the run.
IN  ?= data/arena_stats.json
OUT ?= data/reports/arena_report.html
arena-report:
	go run ./cmd/arena-report -in $(IN) -out $(OUT) -stamp

ranks:
	go run ./cmd/arena -rankings

clean:
	@$(CLEAN)
	@echo "Removed bin/."
