package ml_r2_v3

import (
	_ "embed"
	"encoding/json"
	"math"

	"wizardboardgame/internal/game"
)

//go:embed weights.json
var weightsJSON []byte

type layerWeights struct {
	In  int       `json:"in"`
	Out int       `json:"out"`
	W   []float64 `json:"w"`
	B   []float64 `json:"b"`
	Act string    `json:"act"`
}

type linearWeights struct {
	Version      int            `json:"version"`
	Type         string         `json:"type,omitempty"`
	FeatureCount int            `json:"featureCount"`
	Mean         []float64      `json:"mean"`
	Std          []float64      `json:"std"`
	Coef         []float64      `json:"coef"`
	Intercept    float64        `json:"intercept"`
	Scale        float64        `json:"scale,omitempty"`
	Layers       []layerWeights `json:"layers,omitempty"`
}

const maxLayer = 128

// fastLayer is the inference form of one layer: float32 weights with the
// feature standardization folded into the first layer, so the hot path does
// no divisions and no allocations.
type fastLayer struct {
	in, out int
	w, b    []float32
	relu    bool
}

var (
	fast   []fastLayer
	fastOK bool
	scale  float64
)

func init() {
	var m linearWeights
	if err := json.Unmarshal(weightsJSON, &m); err != nil {
		return
	}
	if m.Type != "mlp" || len(m.Layers) == 0 {
		return
	}
	if m.FeatureCount != len(m.Mean) || m.FeatureCount != len(m.Std) {
		return
	}
	prev := m.FeatureCount
	for _, l := range m.Layers {
		if l.In != prev || l.Out <= 0 || l.Out > maxLayer || len(l.W) != l.In*l.Out || len(l.B) != l.Out {
			return
		}
		prev = l.Out
	}

	for li, l := range m.Layers {
		fl := fastLayer{in: l.In, out: l.Out, relu: l.Act == "relu"}
		fl.w = make([]float32, len(l.W))
		fl.b = make([]float32, len(l.B))
		for o := 0; o < l.Out; o++ {
			bsum := l.B[o]
			for i := 0; i < l.In; i++ {
				v := l.W[o*l.In+i]
				if li == 0 {
					sd := m.Std[i]
					if sd == 0 {
						sd = 1
					}
					// Fold (x - mean) / std into the weights and bias.
					bsum -= v * m.Mean[i] / sd
					v /= sd
				}
				fl.w[o*l.In+i] = float32(v)
			}
			fl.b[o] = float32(bsum)
		}
		fast = append(fast, fl)
	}
	scale = m.Scale
	fastOK = true
}

func predict(x []float32) float64 {
	if !fastOK || len(x) != fast[0].in || fast[0].in > maxLayer {
		return 0
	}
	var cur, next [maxLayer]float32
	copy(cur[:], x)
	n := len(x)
	for _, l := range fast {
		if l.in != n || l.out <= 0 || l.out > maxLayer {
			return 0
		}
		for o := 0; o < l.out; o++ {
			sum := l.b[o]
			base := o * l.in
			for i := 0; i < l.in; i++ {
				sum += l.w[base+i] * cur[i]
			}
			if l.relu && sum < 0 {
				sum = 0
			}
			next[o] = sum
		}
		copy(cur[:l.out], next[:l.out])
		n = l.out
	}
	return float64(cur[0])
}

// evaluate is the search's leaf evaluation: the hand breakdown plus the learned
// correction, clamped so a bad prediction cannot dominate.
func evaluate(s *game.State) int {
	if s.Over {
		return HandEval(s)
	}
	p := HandPartsOf(s)
	x := FeaturesWithParts(s, p)
	c := predict(x[:])
	if scale != 0 {
		c *= scale
	}
	if c > 100 {
		c = 100
	} else if c < -100 {
		c = -100
	}
	return p.Total(s.Turn) + int(math.Round(c))
}
