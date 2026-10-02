// Package math3d holds the small linear-algebra types shared by the cube
// simulator, the geometry builders and the renderer.
//
// Two families of types live here on purpose:
//
//   - floating point vectors and 4x4 matrices used for transforms and shading
//   - exact integer types (Vec3i, Mat3i) used for logical cube state, where a
//     float must never be allowed to drift
package math3d

import "math"

// Vec3 is a 3-component float32 vector.
type Vec3 [3]float32

// Vec3d is a double precision vector, used where accumulation error matters
// (camera fitting, geometry generation).
type Vec3d [3]float64

func V(x, y, z float32) Vec3 { return Vec3{x, y, z} }

func (a Vec3) Add(b Vec3) Vec3 { return Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a Vec3) Mul(s float32) Vec3 {
	return Vec3{a[0] * s, a[1] * s, a[2] * s}
}
func (a Vec3) MulEl(b Vec3) Vec3 {
	return Vec3{a[0] * b[0], a[1] * b[1], a[2] * b[2]}
}
func (a Vec3) Neg() Vec3 { return Vec3{-a[0], -a[1], -a[2]} }
func (a Vec3) Dot(b Vec3) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
func (a Vec3) Len() float32 { return float32(math.Sqrt(float64(a.Dot(a)))) }

func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return Vec3{}
	}
	return a.Mul(1 / l)
}

// Lerp linearly interpolates between a and b.
func (a Vec3) Lerp(b Vec3, t float32) Vec3 {
	return Vec3{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
	}
}

// SRGBToLinear converts a gamma-encoded sRGB channel value to linear light.
// Sticker colours in the specification are given in sRGB, but shading must run
// in linear space for the tone mapper and the environment map to behave.
func SRGBToLinear(c float32) float32 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return float32(math.Pow(float64((c+0.055)/1.055), 2.4))
}

// SRGBToLinear3 converts an sRGB triple to linear light.
func SRGBToLinear3(c Vec3) Vec3 {
	return Vec3{SRGBToLinear(c[0]), SRGBToLinear(c[1]), SRGBToLinear(c[2])}
}

// LinearToSRGB is the inverse of SRGBToLinear. The final blit uses it, although
// the shader itself encodes sRGB in GLSL for precision.
func LinearToSRGB(c float32) float32 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return float32(1.055*math.Pow(float64(c), 1.0/2.4) - 0.055)
}

// Vec3i is an exact integer vector. Logical cubie coordinates live in it, so a
// committed quarter turn can never accumulate error.
type Vec3i [3]int8

func VI(x, y, z int8) Vec3i { return Vec3i{x, y, z} }

func (a Vec3i) Add(b Vec3i) Vec3i { return Vec3i{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a Vec3i) Dot(b Vec3i) int16 {
	return int16(a[0])*int16(b[0]) + int16(a[1])*int16(b[1]) + int16(a[2])*int16(b[2])
}

// MulS scales every component by an exact integer factor.
func (a Vec3i) MulS(s int8) Vec3i { return Vec3i{a[0] * s, a[1] * s, a[2] * s} }

func (a Vec3i) Abs() Vec3i {
	return Vec3i{iabs(a[0]), iabs(a[1]), iabs(a[2])}
}
func (a Vec3i) ToVec3() Vec3 {
	return Vec3{float32(a[0]), float32(a[1]), float32(a[2])}
}

// StickerCount is the number of outward faces of the cubie, i.e. the number of
// non-zero coordinates: 3 for a corner, 2 for an edge, 1 for a center.
func (a Vec3i) StickerCount() int {
	n := 0
	for _, c := range a {
		if c != 0 {
			n++
		}
	}
	return n
}

// Axis returns the index of the non-zero axis of an axis-aligned vector
// (0=X, 1=Y, 2=Z). For a zero vector it returns 2.
func (a Vec3i) Axis() int {
	if a[0] != 0 {
		return 0
	}
	if a[1] != 0 {
		return 1
	}
	return 2
}

// Sign returns +1 or -1 for an axis-aligned unit vector, 0 for the zero vector.
func (a Vec3i) Sign() int8 {
	for _, c := range a {
		if c > 0 {
			return 1
		}
		if c < 0 {
			return -1
		}
	}
	return 0
}

func iabs(v int8) int8 {
	if v < 0 {
		return -v
	}
	return v
}
