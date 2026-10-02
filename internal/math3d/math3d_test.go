package math3d

import "testing"

// TestQuarterTurnGroup enumerates every product of the three quarter turns and
// requires the result to be one of the 24 proper rotation matrices, which is what
// Mat3i.valid is supposed to guarantee.
func TestQuarterTurnGroup(t *testing.T) {
	seen := map[Mat3i]bool{}
	frontier := []Mat3i{Identity3}
	for len(frontier) > 0 {
		next := []Mat3i{}
		for _, m := range frontier {
			if seen[m] {
				continue
			}
			seen[m] = true
			if !m.valid() {
				t.Fatalf("generated an invalid rotation matrix: %v", m)
			}
			for axis := 0; axis < 3; axis++ {
				for _, q := range []int{1, 2, 3} {
					c := m.Mul(QuarterTurn(axis, q))
					if !seen[c] {
						next = append(next, c)
					}
				}
			}
		}
		frontier = next
	}
	if len(seen) != 24 {
		t.Fatalf("rotation group should close at 24 elements, got %d", len(seen))
	}
}

// TestMat4EmbeddingIsOrthonormal checks the row-major to column-major embedding,
// the one place an integer orientation becomes a float transform.
func TestMat4EmbeddingIsOrthonormal(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for q := 0; q < 4; q++ {
			m := QuarterTurn(axis, q).Mat4()
			for c := 0; c < 3; c++ {
				col := m.col(c)
				if l := col[0]*col[0] + col[1]*col[1] + col[2]*col[2]; l < 0.9999 || l > 1.0001 {
					t.Errorf("axis %d q %d col %d not unit (%v)", axis, q, c, l)
				}
			}
			// det of the 3x3 block must be +1 (a rotation, not a reflection).
			a, b, cc := m[0], m[4], m[8]
			d, e, f := m[1], m[5], m[9]
			g, h, i := m[2], m[6], m[10]
			det := a*(e*i-f*h) - b*(d*i-f*g) + cc*(d*h-e*g)
			if det < 0.999 || det > 1.001 {
				t.Errorf("axis %d q %d: det %v", axis, q, det)
			}
			if m[12] != 0 || m[13] != 0 || m[14] != 0 || m[15] != 1 {
				t.Errorf("axis %d q %d: translation row is not zero", axis, q)
			}
		}
	}
}

func TestApplyMatchesMat4TransformDir(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		for _, v := range []Vec3i{{1, 0, 0}, {-1, 1, 0}, {0, -1, -1}, {1, 1, 1}} {
			m := QuarterTurn(axis, 1)
			if got, want := m.Apply(v).ToVec3(), m.Mat4().TransformDir(v.ToVec3()); got != want {
				t.Errorf("axis %d v %v: integer %v != float %v", axis, v, got, want)
			}
		}
	}
}

func TestTransposeIsInverse(t *testing.T) {
	for axis := 0; axis < 3; axis++ {
		m := QuarterTurn(axis, 1)
		if got := m.Mul(m.Transpose()); !got.IsIdentity() {
			t.Errorf("axis %d: m*transpose = %v", axis, got)
		}
	}
}

func TestMat4Invert(t *testing.T) {
	m := Translation(V(1.5, -2, 0.25)).Mul(RotY(0.7)).Mul(RotX(-0.3))
	inv, ok := m.Invert()
	if !ok {
		t.Fatalf("matrix reported singular")
	}
	if got := m.Mul(inv); !got.ApproxEqual(Identity(), 1e-5) {
		t.Errorf("m * m^-1 != identity: %v", got)
	}
	if got := inv.Mul(m); !got.ApproxEqual(Identity(), 1e-5) {
		t.Errorf("m^-1 * m != identity: %v", got)
	}
	var zero Mat4
	if _, ok := zero.Invert(); ok {
		t.Errorf("singular matrix reported invertible")
	}
}

func TestRotationHandedness(t *testing.T) {
	// Right-hand rule about +Z by +90 must take +X to +Y.
	if got := RotY(0).Mul(RotZ(float32(PiHalf))).TransformPoint(V(1, 0, 0)); !near(got, V(0, 1, 0)) {
		t.Errorf("RotZ(+90) sent +X to %v", got)
	}
	// Right-hand rule about +X by +90 must take +Y to +Z.
	if got := RotX(float32(PiHalf)).TransformPoint(V(0, 1, 0)); !near(got, V(0, 0, 1)) {
		t.Errorf("RotX(+90) sent +Y to %v", got)
	}
	// Right-hand rule about +Y by +90 must take +Z to +X.
	if got := RotY(float32(PiHalf)).TransformPoint(V(0, 0, 1)); !near(got, V(1, 0, 0)) {
		t.Errorf("RotY(+90) sent +Z to %v", got)
	}
}

const PiHalf = 1.5707963267948966

func near(a, b Vec3) bool {
	for i := range a {
		d := a[i] - b[i]
		if d > 1e-5 || d < -1e-5 {
			return false
		}
	}
	return true
}

func TestSRGBRoundTrip(t *testing.T) {
	for _, v := range []float32{0, 0.001, 0.04045, 0.2, 0.5, 0.8, 1} {
		if got := LinearToSRGB(SRGBToLinear(v)); got-float32(0.0005) > v || got+float32(0.0005) < v {
			t.Errorf("sRGB round trip of %v gave %v", v, got)
		}
	}
	// The sRGB transfer function is not a plain power law near black: 0.5 encoded
	// must decode above 0.214, which a gamma-2.2 approximation would get wrong.
	if got := SRGBToLinear(0.5); got < 0.21 || got > 0.22 {
		t.Errorf("SRGBToLinear(0.5) = %v", got)
	}
}
