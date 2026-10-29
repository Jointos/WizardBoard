// Package catalog imports every engine so that a single blank import wires the
// whole registry (server, selfplay and arena all use it).
package catalog

import (
	_ "wizardboardgame/internal/ai/alphabeta_r2_v1"
	_ "wizardboardgame/internal/ai/ml_r2_v1"
	_ "wizardboardgame/internal/ai/ml_r2_v2"
	_ "wizardboardgame/internal/ai/ml_r2_v3"
	_ "wizardboardgame/internal/ai/ml_r2_v4"
)
