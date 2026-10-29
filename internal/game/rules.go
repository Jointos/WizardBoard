package game

// Rules holds every tunable number of the game. It is immutable while a match
// is running: all States share the same *Rules.
type Rules struct {
	// PawnForwardCaptureEvery means a pawn may capture forward on every Nth
	// own action (0 disables forward captures entirely).
	PawnForwardCaptureEvery uint8

	// CannonRange is the base shooting range in squares.
	CannonRange int
	// CannonBuffRange is the shooting range while the cannon is buffed.
	CannonBuffRange int

	// MaxTokens is the per-player token cap.
	MaxTokens uint8
	// TeleportCost is how many tokens a wizard teleport costs.
	TeleportCost uint8

	// MaxPly ends the game in a draw after this many turns (0 = unlimited).
	MaxPly uint16

	// PawnBuffSideCapture lets a buffed pawn capture sideways.
	PawnBuffSideCapture bool

	// WizardNeedsLOS requires line of sight for cleanse. Buff and nerf ignore
	// pieces in between; their only spatial constraint is the mandatory step.
	WizardNeedsLOS bool

	// WizardStepTowardTarget forces the wizard to step one square closer to the
	// target before casting (when not already adjacent).
	WizardStepTowardTarget bool

	// PawnBuffAllowsSideMove is reserved for variants where a buffed pawn may
	// also step sideways onto an enemy square. Sideways movement onto empty
	// squares is always allowed (the pawn moves like a king).
	PawnBuffAllowsSideMove bool

	// WizardNerf allows nerfing enemy wizards. It is off by default because the
	// current ruleset deliberately has no wizard nerf.
	WizardNerf bool

	// WizardSwap lets a wizard use its action to exchange places with a friendly
	// unit standing on one of the 8 adjacent squares (wizards included).
	WizardSwap bool
}

// DefaultRules returns the rules described in docs/rules.hu.md.
func DefaultRules() *Rules {
	return &Rules{
		PawnForwardCaptureEvery: 3,
		CannonRange:             2,
		CannonBuffRange:         3,
		MaxTokens:               4,
		TeleportCost:            4,
		MaxPly:                  300,
		PawnBuffSideCapture:     true,
		WizardNeedsLOS:          true,
		WizardStepTowardTarget:  true,
		PawnBuffAllowsSideMove:  false,
		WizardNerf:              false,
		WizardSwap:              true,
	}
}

// Clone returns a deep copy (Rules contains only scalars).
func (r *Rules) Clone() *Rules {
	if r == nil {
		return DefaultRules()
	}
	c := *r
	return &c
}
