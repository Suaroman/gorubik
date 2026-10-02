package math3d

import "fmt"

// Mat3i is an exact 3x3 integer rotation matrix. It is the storage type for a
// cubie's orientation: every committed quarter turn is a multiplication by
// another Mat3i, so orientations stay exactly orthonormal forever and a solved
// cube compares bit-exactly against the identity.
//
// Entries are stored row-major in m[row*3+col].
type Mat3i struct {
	m [9]int8
}

// Identity3 is the identity orientation.
var Identity3 = Mat3i{m: [9]int8{1, 0, 0, 0, 1, 0, 0, 0, 1}}

// NewMat3 builds a Mat3i from row-major entries. It is used by tests and by the
// move table; invalid input panics because it can only come from a coding error.
func NewMat3(r0, r1, r2 int8, r3, r4, r5 int8, r6, r7, r8 int8) Mat3i {
	m := Mat3i{m: [9]int8{r0, r1, r2, r3, r4, r5, r6, r7, r8}}
	if !m.valid() {
		panic("math3d: NewMat3 called with a non-rotation matrix")
	}
	return m
}

// valid reports whether the matrix is one of the 24 proper rotation matrices
// (orthonormal rows, determinant +1).
func (a Mat3i) valid() bool {
	for r := 0; r < 3; r++ {
		n := int16(0)
		for c := 0; c < 3; c++ {
			n += int16(a.m[r*3+c]) * int16(a.m[r*3+c])
		}
		if n != 1 {
			return false
		}
	}
	// Rows must be mutually orthogonal with cross(row0, row1) == row2, which also
	// pins the determinant to +1.
	cx := int16(a.m[1])*int16(a.m[5]) - int16(a.m[2])*int16(a.m[4])
	cy := int16(a.m[2])*int16(a.m[3]) - int16(a.m[0])*int16(a.m[5])
	cz := int16(a.m[0])*int16(a.m[4]) - int16(a.m[1])*int16(a.m[3])
	return cx == int16(a.m[6]) && cy == int16(a.m[7]) && cz == int16(a.m[8])
}

// Mul returns a*b.
func (a Mat3i) Mul(b Mat3i) Mat3i {
	var out Mat3i
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			var s int16
			for k := 0; k < 3; k++ {
				s += int16(a.m[r*3+k]) * int16(b.m[k*3+c])
			}
			out.m[r*3+c] = int8(s)
		}
	}
	return out
}

// Apply rotates an integer vector.
func (a Mat3i) Apply(v Vec3i) Vec3i {
	var out Vec3i
	for r := 0; r < 3; r++ {
		var s int16
		for c := 0; c < 3; c++ {
			s += int16(a.m[r*3+c]) * int16(v[c])
		}
		out[r] = int8(s)
	}
	return out
}

// Transpose returns the inverse of a rotation matrix.
func (a Mat3i) Transpose() Mat3i {
	return Mat3i{m: [9]int8{a.m[0], a.m[3], a.m[6], a.m[1], a.m[4], a.m[7], a.m[2], a.m[5], a.m[8]}}
}

// IsIdentity reports whether the orientation is exactly unrotated.
func (a Mat3i) IsIdentity() bool { return a == Identity3 }

// QuarterTurn builds the exact rotation of quarter*90 degrees about the positive
// end of the given axis (0=X, 1=Y, 2=Z), using the right-hand rule. quarter may
// be negative and is taken modulo 4.
//
// This is the single source of truth for what "a quarter turn" means; the
// animated float rotation uses RotAxis with the same axis and angle sign.
func QuarterTurn(axis, quarter int) Mat3i {
	q := ((quarter % 4) + 4) % 4
	var base Mat3i
	switch axis {
	case 0: // +90 about +X: y -> z, z -> -y
		base = NewMat3(1, 0, 0, 0, 0, -1, 0, 1, 0)
	case 1: // +90 about +Y: z -> x, x -> -z
		base = NewMat3(0, 0, 1, 0, 1, 0, -1, 0, 0)
	default: // +90 about +Z: x -> y, y -> -x
		base = NewMat3(0, -1, 0, 1, 0, 0, 0, 0, 1)
	}
	out := Identity3
	for i := 0; i < q; i++ {
		out = out.Mul(base)
	}
	return out
}

// Mat4 embeds the integer rotation in a float 4x4 rotation matrix.
func (a Mat3i) Mat4() Mat4 {
	m := Identity()
	m[0], m[1], m[2] = float32(a.m[0]), float32(a.m[3]), float32(a.m[6])
	m[4], m[5], m[6] = float32(a.m[1]), float32(a.m[4]), float32(a.m[7])
	m[8], m[9], m[10] = float32(a.m[2]), float32(a.m[5]), float32(a.m[8])
	return m
}

func (a Mat3i) String() string {
	return fmt.Sprintf("[%d %d %d; %d %d %d; %d %d %d]",
		a.m[0], a.m[1], a.m[2], a.m[3], a.m[4], a.m[5], a.m[6], a.m[7], a.m[8])
}
