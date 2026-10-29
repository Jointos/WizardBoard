// Command train fits the linear residual / MLP model used by the ML engines.
//
// It reads the JSONL produced by cmd/dataset, splits by game, standardizes the
// features, solves ridge regression, and writes internal/ai/ml_r2_v1/weights.json.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type row struct {
	Game     int       `json:"game"`
	Ply      int       `json:"ply"`
	X        []float32 `json:"x"`
	Hand     int       `json:"hand"`
	Score    int       `json:"score,omitempty"`
	HasScore bool      `json:"hasScore,omitempty"`
	Outcome  int       `json:"outcome"`
}

type sample struct {
	x []float64
	y float64
}

type weightsFile struct {
	Version      int            `json:"version"`
	Type         string         `json:"type,omitempty"`
	FeatureCount int            `json:"featureCount"`
	Mean         []float64      `json:"mean"`
	Std          []float64      `json:"std"`
	Coef         []float64      `json:"coef"`
	Intercept    float64        `json:"intercept"`
	Layers       []layerWeights `json:"layers,omitempty"`
	Metrics      metrics        `json:"metrics"`
}

type metrics struct {
	TrainMSE   float64 `json:"trainMse"`
	ValMSE     float64 `json:"valMse"`
	TestMSE    float64 `json:"testMse"`
	ValMAE     float64 `json:"valMae"`
	TestMAE    float64 `json:"testMae"`
	ValSign    float64 `json:"valSign"`
	TestSign   float64 `json:"testSign"`
	ZeroValMSE float64 `json:"zeroValMse"`
}

func main() {
	in := flag.String("in", "data/datasets/ml_v1.jsonl", "input JSONL")
	out := flag.String("out", "internal/ai/ml_r2_v1/weights.json", "weights output")
	lambda := flag.Float64("lambda", 5.0, "ridge regularization (linear)")
	modelName := flag.String("model", "mlp", "linear or mlp")
	hidden := flag.Int("hidden", 32, "MLP hidden units")
	epochs := flag.Int("epochs", 40, "MLP epochs")
	lr := flag.Float64("lr", 0.001, "MLP learning rate")
	batch := flag.Int("batch", 256, "MLP batch size")
	l2 := flag.Float64("l2", 1e-4, "MLP L2 regularization")
	target := flag.String("target", "outcome", "outcome or score")
	flag.Parse()

	rows, err := load(*in)
	if err != nil {
		fatal(err)
	}
	if len(rows) == 0 {
		fatal(fmt.Errorf("empty dataset"))
	}
	d := len(rows[0].X)

	byGame := make(map[int][]row)
	var games []int
	for _, r := range rows {
		if _, ok := byGame[r.Game]; !ok {
			games = append(games, r.Game)
		}
		byGame[r.Game] = append(byGame[r.Game], r)
	}
	sort.Ints(games)

	var train, val, test []sample
	for i, g := range games {
		bucket := i * 10 / len(games)
		for _, r := range byGame[g] {
			y := float64(r.Outcome)*100 - float64(r.Hand)
			if *target == "score" && r.HasScore {
				y = clamp(float64(r.Score-r.Hand), -200, 200)
			}
			s := sample{x: toFloat64(r.X), y: y}
			switch {
			case bucket < 8:
				train = append(train, s)
			case bucket == 8:
				val = append(val, s)
			default:
				test = append(test, s)
			}
		}
	}
	if len(train) == 0 {
		fatal(fmt.Errorf("no training samples"))
	}

	mean, std := standardize(train, d)

	var w weightsFile
	if *modelName == "mlp" {
		layers, m := trainMLP(train, val, test, mean, std, d, *hidden, *epochs, *lr, *batch, *l2)
		w = weightsFile{
			Version:      2,
			Type:         "mlp",
			FeatureCount: d,
			Mean:         mean,
			Std:          std,
			Layers:       layers,
			Metrics:      m,
		}
	} else {
		coef, intercept := ridge(train, mean, std, *lambda)
		m := metrics{
			TrainMSE:   mse(train, coef, intercept, mean, std),
			ValMSE:     mse(val, coef, intercept, mean, std),
			TestMSE:    mse(test, coef, intercept, mean, std),
			ValMAE:     mae(val, coef, intercept, mean, std),
			TestMAE:    mae(test, coef, intercept, mean, std),
			ValSign:    signAcc(val, coef, intercept, mean, std),
			TestSign:   signAcc(test, coef, intercept, mean, std),
			ZeroValMSE: zeroMSE(val),
		}
		w = weightsFile{
			Version:      1,
			Type:         "linear",
			FeatureCount: d,
			Mean:         mean,
			Std:          std,
			Coef:         coef,
			Intercept:    intercept,
			Metrics:      m,
		}
	}
	m := w.Metrics
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		fatal(err)
	}
	data = append(data, '\n')
	tmp := *out + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		fatal(err)
	}
	if err := os.Rename(tmp, *out); err != nil {
		fatal(err)
	}

	fmt.Printf("rows: %d (train %d, val %d, test %d), games: %d, features: %d\n",
		len(rows), len(train), len(val), len(test), len(games), d)
	fmt.Printf("train MSE %.1f | val MSE %.1f (zero %.1f) | test MSE %.1f\n", m.TrainMSE, m.ValMSE, m.ZeroValMSE, m.TestMSE)
	fmt.Printf("val MAE %.1f, sign %.1f%% | test MAE %.1f, sign %.1f%%\n", m.ValMAE, m.ValSign*100, m.TestMAE, m.TestSign*100)
	fmt.Printf("wrote %s\n", *out)
}

func load(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	var rows []row
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r row
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		rows = append(rows, r)
	}
	return rows, sc.Err()
}

func toFloat64(x []float32) []float64 {
	out := make([]float64, len(x))
	for i, v := range x {
		out[i] = float64(v)
	}
	return out
}

func standardize(samples []sample, d int) ([]float64, []float64) {
	mean := make([]float64, d)
	for _, s := range samples {
		for i, v := range s.x {
			mean[i] += v
		}
	}
	for i := range mean {
		mean[i] /= float64(len(samples))
	}
	std := make([]float64, d)
	for _, s := range samples {
		for i, v := range s.x {
			diff := v - mean[i]
			std[i] += diff * diff
		}
	}
	for i := range std {
		std[i] = math.Sqrt(std[i] / float64(len(samples)))
		if std[i] == 0 {
			std[i] = 1
		}
	}
	return mean, std
}

func ridge(samples []sample, mean, std []float64, lambda float64) ([]float64, float64) {
	d := len(mean)
	xtx := make([][]float64, d)
	for i := range xtx {
		xtx[i] = make([]float64, d)
	}
	xty := make([]float64, d)
	var meanY float64
	for _, s := range samples {
		meanY += s.y
	}
	meanY /= float64(len(samples))
	for _, s := range samples {
		z := make([]float64, d)
		for i, v := range s.x {
			z[i] = (v - mean[i]) / std[i]
		}
		for i := 0; i < d; i++ {
			xty[i] += z[i] * (s.y - meanY)
			for j := 0; j < d; j++ {
				xtx[i][j] += z[i] * z[j]
			}
		}
	}
	for i := 0; i < d; i++ {
		xtx[i][i] += lambda
	}
	coef := solve(xtx, xty)
	return coef, meanY
}

func solve(a [][]float64, b []float64) []float64 {
	n := len(b)
	m := make([][]float64, n)
	for i := 0; i < n; i++ {
		m[i] = make([]float64, n+1)
		copy(m[i], a[i])
		m[i][n] = b[i]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(m[r][col]) > math.Abs(m[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(m[pivot][col]) < 1e-12 {
			continue
		}
		m[col], m[pivot] = m[pivot], m[col]
		pv := m[col][col]
		for j := col; j <= n; j++ {
			m[col][j] /= pv
		}
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			f := m[r][col]
			if f == 0 {
				continue
			}
			for j := col; j <= n; j++ {
				m[r][j] -= f * m[col][j]
			}
		}
	}
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = m[i][n]
	}
	return x
}

func predict(s sample, coef []float64, intercept float64, mean, std []float64) float64 {
	z := intercept
	for i, v := range s.x {
		z += coef[i] * (v - mean[i]) / std[i]
	}
	return z
}

func mse(samples []sample, coef []float64, intercept float64, mean, std []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		d := predict(s, coef, intercept, mean, std) - s.y
		sum += d * d
	}
	return sum / float64(len(samples))
}

func mae(samples []sample, coef []float64, intercept float64, mean, std []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += math.Abs(predict(s, coef, intercept, mean, std) - s.y)
	}
	return sum / float64(len(samples))
}

func signAcc(samples []sample, coef []float64, intercept float64, mean, std []float64) float64 {
	var ok, n int
	for _, s := range samples {
		if s.y == 0 {
			continue
		}
		n++
		if (predict(s, coef, intercept, mean, std) > 0) == (s.y > 0) {
			ok++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(ok) / float64(n)
}

func zeroMSE(samples []sample) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += s.y * s.y
	}
	return sum / float64(len(samples))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "train:", err)
	os.Exit(1)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
