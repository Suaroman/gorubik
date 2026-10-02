// Package anim drives the timeline: which phase we are in, which move is turning,
// how far through the turn it is, and how the whole cube is oriented in space.
package anim

import "math"

// Ease maps normalised turn progress [0,1] to turn completion [0,1].
type Ease func(t float64) float64

// Smootherstep is Ken Perlin's improved smoothstep: the first and second
// derivatives vanish at both ends, so a turn starts and settles without a jolt.
func Smootherstep(t float64) float64 {
	t = clamp01(t)
	return t * t * t * (t*(t*6-15) + 10)
}

// EaseInOutQuint is sharper in the middle than Smootherstep, which reads as a more
// decisive, mechanical turn.
func EaseInOutQuint(t float64) float64 {
	t = clamp01(t)
	if t < 0.5 {
		return 16 * t * t * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u*u*u/2
}

// EaseInOutCubic is the mildest of the three: a quick, light flick.
func EaseInOutCubic(t float64) float64 {
	t = clamp01(t)
	if t < 0.5 {
		return 4 * t * t * t
	}
	u := -2*t + 2
	return 1 - u*u*u/2
}

// DefaultEase is the curve the timeline uses: quintic in-out, which accelerates
// naturally and settles smoothly, with no bounce.
func DefaultEase(t float64) float64 { return EaseInOutQuint(t) }

// settleShape is a normalised bump that peaks at about 70% of the turn and returns
// to exactly zero, with zero slope, at both ends. Because it is zero at t=1, the
// animated angle at the end of a turn is exactly the committed quarter turn, so
// committing cannot produce a visual pop.
func settleShape(t float64) float64 {
	t = clamp01(t)
	s := math.Sin(math.Pi * t)
	// 6.03 normalises the peak of t^3 sin^2(pi t) to 1.0.
	return 6.03 * t * t * t * s * s
}

// TurnAngle returns the animated angle for a turn, in the same units as total
// (radians in the renderer). settle is the peak overshoot expressed as a fraction
// of total and must be small; 0 disables it entirely.
func TurnAngle(progress, total, settle float64) float64 {
	progress = clamp01(progress)
	a := DefaultEase(progress)
	if settle != 0 {
		a += settle * settleShape(progress)
	}
	return a * total
}

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
