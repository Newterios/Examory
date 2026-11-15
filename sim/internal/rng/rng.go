// Package rng derives independent, reproducible random streams.
//
// Every stream is keyed by (seed, purpose, ids...). The key is hashed with
// SHA-256 and the first 16 bytes seed a PCG generator from math/rand/v2. Only
// the raw 64-bit PCG output is consumed; all continuous variates are built
// here from that output so that the sequence does not depend on library
// sampling algorithms that could change between Go releases.
package rng

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Domain separates simulator streams from any other use of the same seed.
const Domain = "examory-sim/v1"

// Stream is a deterministic source of variates. It is not safe for
// concurrent use; create one stream per purpose.
type Stream struct {
	src *rand.PCG
}

// Key returns the canonical byte string hashed for a stream.
func Key(seed int64, purpose string, ids ...int) []byte {
	if strings.ContainsAny(purpose, ":\x00") {
		panic("rng: purpose must not contain ':' or NUL") // programming error only
	}
	b := make([]byte, 0, 64)
	b = append(b, Domain...)
	b = append(b, ':')
	b = strconv.AppendInt(b, seed, 10)
	b = append(b, ':')
	b = append(b, purpose...)
	for _, id := range ids {
		b = append(b, ':')
		b = strconv.AppendInt(b, int64(id), 10)
	}
	return b
}

// New returns the stream for (seed, purpose, ids...).
func New(seed int64, purpose string, ids ...int) *Stream {
	h := sha256.Sum256(Key(seed, purpose, ids...))
	return &Stream{src: rand.NewPCG(binary.BigEndian.Uint64(h[0:8]), binary.BigEndian.Uint64(h[8:16]))}
}

// NewString keys a stream additionally by a string (e.g. a pool item id).
func NewString(seed int64, purpose, s string) *Stream {
	b := Key(seed, purpose)
	b = append(b, 0)
	b = append(b, s...)
	h := sha256.Sum256(b)
	return &Stream{src: rand.NewPCG(binary.BigEndian.Uint64(h[0:8]), binary.BigEndian.Uint64(h[8:16]))}
}

// Uint64 returns the next raw 64-bit value.
func (s *Stream) Uint64() uint64 { return s.src.Uint64() }

// Float64 returns a uniform variate in [0, 1) with 53 bits of precision.
func (s *Stream) Float64() float64 { return float64(s.src.Uint64()>>11) * 0x1p-53 }

// Open01 returns a uniform variate in (0, 1).
func (s *Stream) Open01() float64 { return (float64(s.src.Uint64()>>11) + 0.5) * 0x1p-53 }

// Uniform returns a uniform variate in [lo, hi).
func (s *Stream) Uniform(lo, hi float64) float64 { return lo + (hi-lo)*s.Float64() }

// IntN returns a uniform integer in [0, n) without modulo bias. n must be > 0.
func (s *Stream) IntN(n int) int {
	if n <= 0 {
		panic("rng: IntN with n <= 0")
	}
	// Lemire's nearly-divisionless method on 64 bits.
	un := uint64(n)
	hi, lo := mul64(s.src.Uint64(), un)
	if lo < un {
		thresh := -un % un
		for lo < thresh {
			hi, lo = mul64(s.src.Uint64(), un)
		}
	}
	return int(hi)
}

func mul64(a, b uint64) (hi, lo uint64) {
	const mask32 = 1<<32 - 1
	a0, a1 := a&mask32, a>>32
	b0, b1 := b&mask32, b>>32
	w0 := a0 * b0
	t := a1*b0 + w0>>32
	w1 := t&mask32 + a0*b1
	hi = a1*b1 + t>>32 + w1>>32
	lo = a * b
	return
}

// IntRange returns a uniform integer in [lo, hi] inclusive.
func (s *Stream) IntRange(lo, hi int64) int64 {
	if hi < lo {
		panic("rng: IntRange with hi < lo")
	}
	return lo + int64(s.IntN(int(hi-lo+1)))
}

// Norm returns a standard normal variate (Box-Muller, exactly two uniforms).
func (s *Stream) Norm() float64 {
	u1 := s.Open01()
	u2 := s.Float64()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

// Gamma returns a Gamma(shape, 1) variate (Marsaglia-Tsang). shape > 0.
func (s *Stream) Gamma(shape float64) float64 {
	if shape < 1 {
		u := s.Open01()
		return s.Gamma(shape+1) * math.Pow(u, 1/shape)
	}
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := s.Norm()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := s.Open01()
		if math.Log(u) < 0.5*x*x+d-d*v+d*math.Log(v) {
			return d * v
		}
	}
}

// Beta returns a Beta(a, b) variate. a, b > 0.
func (s *Stream) Beta(a, b float64) float64 {
	x := s.Gamma(a)
	y := s.Gamma(b)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// Dirichlet returns a Dirichlet(alpha) variate.
func (s *Stream) Dirichlet(alpha []float64) []float64 {
	out := make([]float64, len(alpha))
	var sum float64
	for i, a := range alpha {
		out[i] = s.Gamma(a)
		sum += out[i]
	}
	for i := range out {
		if sum == 0 {
			out[i] = 1 / float64(len(out))
		} else {
			out[i] /= sum
		}
	}
	return out
}

// Shuffle permutes the first n positions in place (Fisher-Yates).
func (s *Stream) Shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		swap(i, s.IntN(i+1))
	}
}

// Perm returns a uniformly random permutation of [0, n).
func (s *Stream) Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	s.Shuffle(n, func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}

// LogNormalMs returns round(exp(mu + sd*z)) as integer milliseconds, floored
// at floor, for a supplied standard normal z (common random numbers).
func LogNormalMs(mu, sd, z float64, floor int64) int64 {
	v := math.Exp(mu + sd*z)
	if math.IsInf(v, 1) || v > 1e15 {
		v = 1e15
	}
	ms := int64(math.Round(v))
	if ms < floor {
		ms = floor
	}
	return ms
}

// TruncNormalLower draws N(mu, sd^2) truncated to [lower, inf) by inversion
// of the upper tail with one uniform u in (0, 1]. It is numerically stable
// far into the tail.
func TruncNormalLower(mu, sd, lower, u float64) float64 {
	if sd <= 0 {
		return math.Max(mu, lower)
	}
	alpha := (lower - mu) / sd
	tail := 0.5 * math.Erfc(alpha/math.Sqrt2) // P(Z >= alpha)
	if tail <= 0 {
		return lower
	}
	x := upperQuantile(math.Log(u)+math.Log(tail), alpha)
	v := mu + sd*x
	if math.IsNaN(v) || v < lower {
		return lower
	}
	return v
}

// upperQuantile returns x with log P(Z >= x) = logq for a standard normal Z,
// accurate far into the tail: an Erfcinv start refined by Newton steps on
// log P(Z >= x). x is kept >= lower.
func upperQuantile(logq, lower float64) float64 {
	x := math.Sqrt2 * math.Erfcinv(2*math.Exp(logq))
	if math.IsInf(x, 0) || math.IsNaN(x) {
		x = math.Sqrt(-2 * logq)
	}
	if x < lower {
		x = lower
	}
	for i := 0; i < 100; i++ {
		s := 0.5 * math.Erfc(x/math.Sqrt2)
		if s <= 0 {
			break
		}
		phi := math.Exp(-x*x/2) / math.Sqrt(2*math.Pi)
		step := (math.Log(s) - logq) / (-phi / s)
		x -= step
		if math.Abs(step) < 1e-12*math.Max(1, math.Abs(x)) {
			break
		}
	}
	return x
}
