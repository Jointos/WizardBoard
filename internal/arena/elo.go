package arena

import "math"

// KFactor controls how much a single batch of games moves a rating. It is
// deliberately smaller than the classic per-game K because the arena plays
// 100-game batches (K=8 per game keeps the scale useful for a small pool).
const KFactor = 8.0

// Expected returns the expected score of ra against rb.
func Expected(ra, rb float64) float64 {
	return 1 / (1 + math.Pow(10, (rb-ra)/400))
}

// Update applies a batch result (scoreA in [0,1] over `games` games) and returns
// the new ratings. The pool average is preserved.
func Update(ra, rb, scoreA float64, games int, k float64) (float64, float64) {
	if games <= 0 {
		return ra, rb
	}
	ea := Expected(ra, rb)
	ra2 := ra + k*float64(games)*(scoreA-ea)
	rb2 := ra + rb - ra2
	return ra2, rb2
}
