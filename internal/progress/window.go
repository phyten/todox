package progress

import (
	"math"
	"sort"
)

type window struct {
	size   int
	values []float64
}

func newWindow(size int) *window {
	if size <= 0 {
		size = 1
	}
	return &window{size: size}
}

func (w *window) Add(v float64) {
	if w.size <= 0 {
		return
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	if len(w.values) < w.size {
		w.values = append(w.values, v)
		return
	}
	copy(w.values, w.values[1:])
	w.values[len(w.values)-1] = v
}

func (w *window) Quantile(q float64) float64 {
	if len(w.values) == 0 {
		return 0
	}
	if q <= 0 {
		return w.min()
	}
	if q >= 1 {
		return w.max()
	}
	cp := append([]float64(nil), w.values...)
	sort.Float64s(cp)
	pos := q * float64(len(cp)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower < 0 {
		lower = 0
	}
	if upper >= len(cp) {
		upper = len(cp) - 1
	}
	if lower == upper {
		return cp[lower]
	}
	weight := pos - float64(lower)
	return cp[lower]*(1-weight) + cp[upper]*weight
}

func (w *window) min() float64 {
	if len(w.values) == 0 {
		return 0
	}
	min := w.values[0]
	for _, v := range w.values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func (w *window) max() float64 {
	if len(w.values) == 0 {
		return 0
	}
	max := w.values[0]
	for _, v := range w.values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
