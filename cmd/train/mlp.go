package main

import (
	"fmt"
	"math"
	"math/rand"
)

type layerWeights struct {
	In  int       `json:"in"`
	Out int       `json:"out"`
	W   []float64 `json:"w"`
	B   []float64 `json:"b"`
	Act string    `json:"act"`
}

// trainMLP fits a d -> hidden -> 1 ReLU network with Adam, keeping the best
// validation epoch. The engine supports up to 128 units per layer.
func trainMLP(train, val, test []sample, mean, std []float64, d, hidden, epochs int, lr float64, batch int, l2 float64) ([]layerWeights, metrics) {
	if hidden > 128 {
		hidden = 128
	}
	xs := make([][]float64, len(train))
	ys := make([]float64, len(train))
	for i, s := range train {
		z := make([]float64, d)
		for j, v := range s.x {
			z[j] = (v - mean[j]) / std[j]
		}
		xs[i] = z
		ys[i] = s.y
	}

	rng := rand.New(rand.NewSource(1))
	w1 := make([]float64, hidden*d)
	b1 := make([]float64, hidden)
	w2 := make([]float64, hidden)
	var b2 float64
	scale1 := math.Sqrt(2.0 / float64(d))
	for i := range w1 {
		w1[i] = rng.NormFloat64() * scale1
	}
	scale2 := math.Sqrt(2.0 / float64(hidden))
	for i := range w2 {
		w2[i] = rng.NormFloat64() * scale2
	}

	mW1, vW1 := make([]float64, len(w1)), make([]float64, len(w1))
	mB1, vB1 := make([]float64, hidden), make([]float64, hidden)
	mW2, vW2 := make([]float64, hidden), make([]float64, hidden)
	var mB2, vB2 float64
	gW1, gB1 := make([]float64, len(w1)), make([]float64, hidden)
	gW2 := make([]float64, hidden)
	var gB2 float64
	h := make([]float64, hidden)
	dh := make([]float64, hidden)
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}

	bestVal := math.Inf(1)
	var bestW1, bestB1, bestW2 []float64
	var bestB2 float64
	patience := 0
	step := 0

	for ep := 1; ep <= epochs; ep++ {
		rng.Shuffle(len(idx), func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
		for start := 0; start < len(idx); start += batch {
			end := start + batch
			if end > len(idx) {
				end = len(idx)
			}
			n := float64(end - start)
			for i := range gW1 {
				gW1[i] = 0
			}
			for i := range gB1 {
				gB1[i] = 0
			}
			for i := range gW2 {
				gW2[i] = 0
			}
			gB2 = 0

			for _, id := range idx[start:end] {
				x := xs[id]
				y := ys[id]
				for o := 0; o < hidden; o++ {
					sum := b1[o]
					base := o * d
					for i := 0; i < d; i++ {
						sum += w1[base+i] * x[i]
					}
					if sum < 0 {
						sum = 0
					}
					h[o] = sum
				}
				yhat := b2
				for o := 0; o < hidden; o++ {
					yhat += w2[o] * h[o]
				}
				dy := 2 * (yhat - y) / n
				gB2 += dy
				for o := 0; o < hidden; o++ {
					gW2[o] += dy * h[o]
					dh[o] = dy * w2[o]
					if h[o] <= 0 {
						dh[o] = 0
					}
				}
				for o := 0; o < hidden; o++ {
					gB1[o] += dh[o]
					base := o * d
					for i := 0; i < d; i++ {
						gW1[base+i] += dh[o] * x[i]
					}
				}
			}

			step++
			bc1 := 1 - math.Pow(0.9, float64(step))
			bc2 := 1 - math.Pow(0.999, float64(step))
			upd := func(w, g, m, v []float64, weight bool) {
				for i := range w {
					gg := g[i]
					if weight {
						gg += l2 * w[i]
					}
					m[i] = 0.9*m[i] + 0.1*gg
					v[i] = 0.999*v[i] + 0.001*gg*gg
					w[i] -= lr * (m[i] / bc1) / (math.Sqrt(v[i]/bc2) + 1e-8)
				}
			}
			upd(w1, gW1, mW1, vW1, true)
			upd(b1, gB1, mB1, vB1, false)
			upd(w2, gW2, mW2, vW2, true)
			mB2 = 0.9*mB2 + 0.1*gB2
			vB2 = 0.999*vB2 + 0.001*gB2*gB2
			b2 -= lr * (mB2 / bc1) / (math.Sqrt(vB2/bc2) + 1e-8)
		}

		vm := mlpMSE(val, w1, b1, w2, b2, hidden, d, mean, std)
		if vm < bestVal {
			bestVal = vm
			bestW1 = append([]float64(nil), w1...)
			bestB1 = append([]float64(nil), b1...)
			bestW2 = append([]float64(nil), w2...)
			bestB2 = b2
			patience = 0
		} else {
			patience++
		}
		if ep == 1 || ep%5 == 0 {
			fmt.Printf("epoch %d: val MSE %.1f\n", ep, vm)
		}
		if patience >= 8 {
			fmt.Printf("early stop at epoch %d\n", ep)
			break
		}
	}

	layers := []layerWeights{
		{In: d, Out: hidden, W: bestW1, B: bestB1, Act: "relu"},
		{In: hidden, Out: 1, W: bestW2, B: []float64{bestB2}, Act: "linear"},
	}
	m := metrics{
		TrainMSE:   mlpMSE(train, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		ValMSE:     bestVal,
		TestMSE:    mlpMSE(test, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		ValMAE:     mlpMAE(val, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		TestMAE:    mlpMAE(test, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		ValSign:    mlpSign(val, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		TestSign:   mlpSign(test, bestW1, bestB1, bestW2, bestB2, hidden, d, mean, std),
		ZeroValMSE: zeroMSE(val),
	}
	return layers, m
}

func mlpForward(s sample, w1, b1, w2 []float64, b2 float64, hidden, d int, mean, std []float64) float64 {
	h := make([]float64, hidden)
	for o := 0; o < hidden; o++ {
		sum := b1[o]
		base := o * d
		for i := 0; i < d; i++ {
			sd := std[i]
			if sd == 0 {
				sd = 1
			}
			sum += w1[base+i] * (s.x[i] - mean[i]) / sd
		}
		if sum < 0 {
			sum = 0
		}
		h[o] = sum
	}
	y := b2
	for o := 0; o < hidden; o++ {
		y += w2[o] * h[o]
	}
	return y
}

func mlpMSE(samples []sample, w1, b1, w2 []float64, b2 float64, hidden, d int, mean, std []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		diff := mlpForward(s, w1, b1, w2, b2, hidden, d, mean, std) - s.y
		sum += diff * diff
	}
	return sum / float64(len(samples))
}

func mlpMAE(samples []sample, w1, b1, w2 []float64, b2 float64, hidden, d int, mean, std []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += math.Abs(mlpForward(s, w1, b1, w2, b2, hidden, d, mean, std) - s.y)
	}
	return sum / float64(len(samples))
}

func mlpSign(samples []sample, w1, b1, w2 []float64, b2 float64, hidden, d int, mean, std []float64) float64 {
	var ok, n int
	for _, s := range samples {
		if s.y == 0 {
			continue
		}
		n++
		if (mlpForward(s, w1, b1, w2, b2, hidden, d, mean, std) > 0) == (s.y > 0) {
			ok++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(ok) / float64(n)
}
