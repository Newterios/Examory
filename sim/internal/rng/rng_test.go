package rng

import (
	"math"
	"testing"
)

func TestStreamsDeterministicAndIndependent(t *testing.T) {
	a, b := New(42, "x", 1, 2), New(42, "x", 1, 2)
	c, d := New(42, "x", 12), New(43, "x", 1, 2)
	for i := 0; i < 100; i++ {
		va := a.Uint64()
		if va != b.Uint64() {
			t.Fatal("same key, different output")
		}
		if va == c.Uint64() || va == d.Uint64() {
			t.Fatal("distinct keys collide")
		}
	}
	if string(Key(1, "p", 1, 23)) == string(Key(1, "p", 12, 3)) {
		t.Fatal("ambiguous key encoding")
	}
	if s1, s2 := NewString(1, "p", "ab"), NewString(1, "p", "ac"); s1.Uint64() == s2.Uint64() {
		t.Fatal("string keys collide")
	}
}

func moments(n int, f func() float64) (mean, variance float64) {
	var s, s2 float64
	for i := 0; i < n; i++ {
		x := f()
		s += x
		s2 += x * x
	}
	mean = s / float64(n)
	return mean, s2/float64(n) - mean*mean
}

func TestDistributions(t *testing.T) {
	s := New(7, "dist")
	const n = 200000
	if m, v := moments(n, s.Norm); math.Abs(m) > 0.01 || math.Abs(v-1) > 0.02 {
		t.Fatalf("normal moments %v %v", m, v)
	}
	if m, v := moments(n, s.Float64); math.Abs(m-0.5) > 0.005 || math.Abs(v-1.0/12) > 0.005 {
		t.Fatalf("uniform moments %v %v", m, v)
	}
	if m, _ := moments(n, func() float64 { return s.Beta(1.2, 12) }); math.Abs(m-1.2/13.2) > 0.003 {
		t.Fatalf("beta mean %v", m)
	}
	if m, _ := moments(n, func() float64 { return s.Gamma(0.5) }); math.Abs(m-0.5) > 0.01 {
		t.Fatalf("gamma(0.5) mean %v", m)
	}
	counts := make([]int, 7)
	for i := 0; i < n; i++ {
		v := s.IntN(7)
		if v < 0 || v >= 7 {
			t.Fatal("IntN out of range")
		}
		counts[v]++
	}
	for _, c := range counts {
		if math.Abs(float64(c)/n-1.0/7) > 0.005 {
			t.Fatalf("IntN not uniform: %v", counts)
		}
	}
	for i := 0; i < 1000; i++ {
		if v := s.IntRange(3, 5); v < 3 || v > 5 {
			t.Fatal("IntRange out of range")
		}
		if v := s.Open01(); v <= 0 || v >= 1 {
			t.Fatal("Open01 out of range")
		}
	}
	d := s.Dirichlet([]float64{2, 1, 0.5})
	if math.Abs(d[0]+d[1]+d[2]-1) > 1e-12 {
		t.Fatal("dirichlet does not sum to 1")
	}
	p := s.Perm(10)
	seen := map[int]bool{}
	for _, v := range p {
		seen[v] = true
	}
	if len(seen) != 10 {
		t.Fatal("perm is not a permutation")
	}
}

func TestTruncNormal(t *testing.T) {
	s := New(1, "tn")
	for _, lower := range []float64{-3, 0, 1.7, 5, 12, 40} {
		var sum float64
		for i := 0; i < 20000; i++ {
			v := TruncNormalLower(1.2, 0.5, lower, 1-s.Float64())
			if v < lower || math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("lower %v: got %v", lower, v)
			}
			sum += v
		}
		mean := sum / 20000
		// Mean of the truncated normal: mu + sd*phi(a)/(1-Phi(a)).
		a := (lower - 1.2) / 0.5
		tail := 0.5 * math.Erfc(a/math.Sqrt2)
		if tail > 1e-12 {
			want := 1.2 + 0.5*math.Exp(-a*a/2)/math.Sqrt(2*math.Pi)/tail
			if math.Abs(mean-want) > 0.02 {
				t.Fatalf("lower %v: mean %v want %v", lower, mean, want)
			}
		}
	}
}

func TestLogNormalMs(t *testing.T) {
	if LogNormalMs(math.Log(1000), 0, 0, 1) != 1000 {
		t.Fatal("median")
	}
	if LogNormalMs(0, 1, -10, 500) != 500 {
		t.Fatal("floor")
	}
	if LogNormalMs(100, 1, 0, 1) != 1e15 {
		t.Fatal("overflow cap")
	}
}
