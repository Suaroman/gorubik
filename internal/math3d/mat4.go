package math3d

import "math"

// Mat4 is a 4x4 matrix stored column-major, matching GLSL's layout so it can be
// uploaded with a single uniformMatrix4fv call.
type Mat4 [16]float32

// col returns the slice for column c.
func (m *Mat4) col(c int) []float32 { return m[c*4 : c*4+4] }

func Identity() Mat4 {
	var m Mat4
	m[0], m[5], m[10], m[15] = 1, 1, 1, 1
	return m
}

// Mul returns a*b, i.e. the transform that applies b first, then a.
func (a Mat4) Mul(b Mat4) Mat4 {
	var out Mat4
	for c := 0; c < 4; c++ {
		bc := b.col(c)
		oc := out.col(c)
		for r := 0; r < 4; r++ {
			oc[r] = a.col(0)[r]*bc[0] + a.col(1)[r]*bc[1] + a.col(2)[r]*bc[2] + a.col(3)[r]*bc[3]
		}
	}
	return out
}

// Translation builds a translation matrix.
func Translation(v Vec3) Mat4 {
	m := Identity()
	m[12], m[13], m[14] = v[0], v[1], v[2]
	return m
}

// Scale builds a (possibly non-uniform) scale matrix.
func Scale(v Vec3) Mat4 {
	m := Identity()
	m[0], m[5], m[10] = v[0], v[1], v[2]
	return m
}

// RotX rotates by ang radians about +X (right-hand rule).
func RotX(ang float32) Mat4 {
	s, c := float32(math.Sin(float64(ang))), float32(math.Cos(float64(ang)))
	m := Identity()
	m[5], m[6] = c, s
	m[9], m[10] = -s, c
	return m
}

// RotY rotates by ang radians about +Y (right-hand rule).
func RotY(ang float32) Mat4 {
	s, c := float32(math.Sin(float64(ang))), float32(math.Cos(float64(ang)))
	m := Identity()
	m[0], m[2] = c, -s
	m[8], m[10] = s, c
	return m
}

// RotZ rotates by ang radians about +Z (right-hand rule).
func RotZ(ang float32) Mat4 {
	s, c := float32(math.Sin(float64(ang))), float32(math.Cos(float64(ang)))
	m := Identity()
	m[0], m[1] = c, s
	m[4], m[5] = -s, c
	return m
}

// RotAxis rotates by ang radians about a world axis index (0=X, 1=Y, 2=Z). It
// is the primitive used for the animated layer turn, so that the animated angle
// and the committed integer quarter turn describe exactly the same rotation.
func RotAxis(axis int, ang float32) Mat4 {
	switch axis {
	case 0:
		return RotX(ang)
	case 1:
		return RotY(ang)
	default:
		return RotZ(ang)
	}
}

// Perspective builds a right-handed OpenGL perspective matrix.
func Perspective(fovyRad, aspect, near, far float32) Mat4 {
	f := float32(1.0 / math.Tan(float64(fovyRad)/2))
	var m Mat4
	m[0] = f / aspect
	m[5] = f
	m[10] = (far + near) / (near - far)
	m[11] = -1
	m[14] = 2 * far * near / (near - far)
	return m
}

// Ortho builds a right-handed OpenGL orthographic matrix.
func Ortho(left, right, bottom, top, near, far float32) Mat4 {
	var m Mat4
	m[0] = 2 / (right - left)
	m[5] = 2 / (top - bottom)
	m[10] = -2 / (far - near)
	m[12] = -(right + left) / (right - left)
	m[13] = -(top + bottom) / (top - bottom)
	m[14] = -(far + near) / (far - near)
	m[15] = 1
	return m
}

// LookAt builds a view matrix for a right-handed camera looking from eye toward
// center.
func LookAt(eye, center, up Vec3) Mat4 {
	f := center.Sub(eye).Norm()
	s := f.Cross(up).Norm()
	u := s.Cross(f)
	var m Mat4
	m[0], m[4], m[8] = s[0], s[1], s[2]
	m[1], m[5], m[9] = u[0], u[1], u[2]
	m[2], m[6], m[10] = -f[0], -f[1], -f[2]
	m[12] = -s.Dot(eye)
	m[13] = -u.Dot(eye)
	m[14] = f.Dot(eye)
	m[15] = 1
	return m
}

// TransformPoint applies the matrix to a point (w = 1, perspective divided out).
func (m Mat4) TransformPoint(v Vec3) Vec3 {
	x, y, z := v[0], v[1], v[2]
	w := m[3]*x + m[7]*y + m[11]*z + m[15]
	return Vec3{
		(m[0]*x + m[4]*y + m[8]*z + m[12]) / w,
		(m[1]*x + m[5]*y + m[9]*z + m[13]) / w,
		(m[2]*x + m[6]*y + m[10]*z + m[14]) / w,
	}
}

// TransformDir applies only the upper-left 3x3 (no translation, no divide).
func (m Mat4) TransformDir(v Vec3) Vec3 {
	x, y, z := v[0], v[1], v[2]
	return Vec3{
		m[0]*x + m[4]*y + m[8]*z,
		m[1]*x + m[5]*y + m[9]*z,
		m[2]*x + m[6]*y + m[10]*z,
	}
}

// TransformVec4 applies the matrix to a homogeneous 4-vector.
func (m Mat4) TransformVec4(v [4]float32) [4]float32 {
	return [4]float32{
		m[0]*v[0] + m[4]*v[1] + m[8]*v[2] + m[12]*v[3],
		m[1]*v[0] + m[5]*v[1] + m[9]*v[2] + m[13]*v[3],
		m[2]*v[0] + m[6]*v[1] + m[10]*v[2] + m[14]*v[3],
		m[3]*v[0] + m[7]*v[1] + m[11]*v[2] + m[15]*v[3],
	}
}

// Upper3x3 returns the rotation/scale block as a 9-element column-major array,
// which is what the shaders use to transform normals.
func (m Mat4) Upper3x3() [9]float32 {
	return [9]float32{m[0], m[1], m[2], m[4], m[5], m[6], m[8], m[9], m[10]}
}

// Invert returns the inverse of a general 4x4 matrix, or false when singular.
func (m Mat4) Invert() (Mat4, bool) {
	var inv [16]float32
	a := m
	// Cofactor expansion, the standard gl-matrix formulation.
	a00, a01, a02, a03 := a[0], a[1], a[2], a[3]
	a10, a11, a12, a13 := a[4], a[5], a[6], a[7]
	a20, a21, a22, a23 := a[8], a[9], a[10], a[11]
	a30, a31, a32, a33 := a[12], a[13], a[14], a[15]

	b00 := a00*a11 - a01*a10
	b01 := a00*a12 - a02*a10
	b02 := a00*a13 - a03*a10
	b03 := a01*a12 - a02*a11
	b04 := a01*a13 - a03*a11
	b05 := a02*a13 - a03*a12
	b06 := a20*a31 - a21*a30
	b07 := a20*a32 - a22*a30
	b08 := a20*a33 - a23*a30
	b09 := a21*a32 - a22*a31
	b10 := a21*a33 - a23*a31
	b11 := a22*a33 - a23*a32

	det := b00*b11 - b01*b10 + b02*b09 + b03*b08 - b04*b07 + b05*b06
	if det == 0 {
		return Mat4{}, false
	}
	det = 1 / det
	inv[0] = (a11*b11 - a12*b10 + a13*b09) * det
	inv[1] = (a02*b10 - a01*b11 - a03*b09) * det
	inv[2] = (a31*b05 - a32*b04 + a33*b03) * det
	inv[3] = (a22*b04 - a21*b05 - a23*b03) * det
	inv[4] = (a12*b08 - a10*b11 - a13*b07) * det
	inv[5] = (a00*b11 - a02*b08 + a03*b07) * det
	inv[6] = (a32*b02 - a30*b05 - a33*b01) * det
	inv[7] = (a20*b05 - a22*b02 + a23*b01) * det
	inv[8] = (a10*b10 - a11*b08 + a13*b06) * det
	inv[9] = (a01*b08 - a00*b10 - a03*b06) * det
	inv[10] = (a30*b04 - a31*b02 + a33*b00) * det
	inv[11] = (a21*b02 - a20*b04 - a23*b00) * det
	inv[12] = (a11*b07 - a10*b09 - a12*b06) * det
	inv[13] = (a00*b09 - a01*b07 + a02*b06) * det
	inv[14] = (a31*b01 - a30*b03 - a32*b00) * det
	inv[15] = (a20*b03 - a21*b01 + a22*b00) * det
	return inv, true
}

// Transpose returns the matrix transpose.
func (m Mat4) Transpose() Mat4 {
	var out Mat4
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			out.col(c)[r] = m.col(r)[c]
		}
	}
	return out
}

// ApproxEqual reports whether every element is within eps of other. Used by
// tests that compare a float transform against an exact integer rotation.
func (m Mat4) ApproxEqual(o Mat4, eps float32) bool {
	for i := range m {
		if d := m[i] - o[i]; d > eps || d < -eps {
			return false
		}
	}
	return true
}
