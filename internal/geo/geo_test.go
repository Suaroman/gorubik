package geo

import (
	"math"
	"testing"

	"github.com/suaro/gorubik/internal/math3d"
)

// These tests are the reason the procedural meshes can be trusted before a single
// pixel is rendered: watertightness, winding, normal validity and bounding boxes
// are all asserted here.

func TestRoundedBoxIsWatertightAndCorrectlySized(t *testing.T) {
	const h = 0.485
	m := RoundedBox(h, 0.055, 3)
	lo := [3]float32{-h, -h, -h}
	hi := [3]float32{h, h, h}
	if err := m.Check(lo, hi); err != nil {
		t.Fatalf("rounded box failed self-check: %v", err)
	}
	if m.TriCount() < 200 {
		t.Errorf("rounded box has only %d triangles, bevels will look faceted", m.TriCount())
	}
	// The mesh is instanced 26 times, so it must stay small.
	if len(m.Verts) > 1200 {
		t.Errorf("rounded box has %d vertices, unexpectedly heavy", len(m.Verts))
	}
}

func TestRoundedBoxFacesStayFlat(t *testing.T) {
	// A cubie has to read as a cubie: everything that is not inside a bevel band
	// must lie exactly on a flat face with an axis-aligned normal.
	const h, r = 0.5, 0.06
	m := RoundedBox(h, r, 3)
	inner := h - r
	flat, bevel := 0, 0
	for _, v := range m.Verts {
		ax := math.Abs(float64(v.Pos[0]))
		ay := math.Abs(float64(v.Pos[1]))
		az := math.Abs(float64(v.Pos[2]))
		onFace := (within(ax, h) && ay <= inner+1e-9 && az <= inner+1e-9) ||
			(within(ay, h) && ax <= inner+1e-9 && az <= inner+1e-9) ||
			(within(az, h) && ax <= inner+1e-9 && ay <= inner+1e-9)
		if !onFace {
			bevel++
			continue
		}
		flat++
		nx, ny, nz := math.Abs(float64(v.Normal[0])), math.Abs(float64(v.Normal[1])), math.Abs(float64(v.Normal[2]))
		if !(within(nx, 1) || within(ny, 1) || within(nz, 1)) {
			t.Fatalf("flat vertex %v has a non-axis normal %v", v.Pos, v.Normal)
		}
	}
	if flat == 0 || bevel == 0 {
		t.Fatalf("expected both flat and beveled vertices, got %d flat / %d beveled", flat, bevel)
	}
	// The bevel must actually round the corner: the diagonal direction should be
	// pulled in relative to a sharp cube's corner distance.
	sharp := math.Sqrt(3) * h
	if m.Radius() >= float32(sharp)-1e-6 {
		t.Errorf("corner radius %v is not inside the sharp corner distance %v", m.Radius(), sharp)
	}
}

func TestStickerPlateIsWatertightAndCorrectlySized(t *testing.T) {
	const half, rad, thick, cham = 0.36, 0.1, 0.012, 0.006
	m := StickerPlate(half, rad, thick, cham, 6)
	lo := [3]float32{-half, -half, 0}
	hi := [3]float32{half, half, thick}
	if err := m.Check(lo, hi); err != nil {
		t.Fatalf("sticker plate failed self-check: %v", err)
	}
	// The chamfer must lean outward: some rim normal has to have both a positive
	// radial and a positive z component, otherwise the edge would be a hard step.
	lean := 0
	for _, v := range m.Verts {
		radial := math.Hypot(float64(v.Normal[0]), float64(v.Normal[1]))
		if radial > 0.2 && float64(v.Normal[2]) > 0.2 {
			lean++
		}
	}
	if lean == 0 {
		t.Errorf("no chamfer vertex leans outward; the sticker rim will not catch light")
	}
	// Rounded corners must be present: no vertex may sit exactly on a corner of the
	// bounding square.
	for _, v := range m.Verts {
		if within(math.Abs(float64(v.Pos[0])), half) && within(math.Abs(float64(v.Pos[1])), half) {
			t.Errorf("vertex %v sits on a sharp corner of the sticker square", v.Pos)
		}
	}
}

func TestFaceTransformsAreRotationsOntoTheRightFaces(t *testing.T) {
	const half = 0.485
	tf := FaceTransforms(half)
	wantNormals := [6]math3d.Vec3{
		{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1},
	}
	for i, m := range tf {
		if got := m.TransformDir(math3d.V(0, 0, 1)); got != wantNormals[i] {
			t.Errorf("face %d: local +Z maps to %v want %v", i, got, wantNormals[i])
		}
		if got := m.TransformDir(math3d.V(1, 0, 0)); got.Dot(wantNormals[i]) != 0 {
			t.Errorf("face %d: local basis is not perpendicular to the normal", i)
		}
		// det +1: a sticker must never be mirrored, or its winding would invert and
		// back-face culling would delete the visible side.
		basis := m.Upper3x3()
		det := float64(basis[0])*(float64(basis[4])*float64(basis[8])-float64(basis[5])*float64(basis[7])) -
			float64(basis[3])*(float64(basis[1])*float64(basis[8])-float64(basis[2])*float64(basis[7])) +
			float64(basis[6])*(float64(basis[1])*float64(basis[5])-float64(basis[2])*float64(basis[4]))
		if math.Abs(det-1) > 1e-5 {
			t.Errorf("face %d: basis determinant %v is not +1", i, det)
		}
		if got := m.TransformPoint(math3d.V(0, 0, 0)); got != wantNormals[i].Mul(half) {
			t.Errorf("face %d: origin lands at %v want %v", i, got, wantNormals[i].Mul(half))
		}
	}
}

func within(v, want float64) bool { return math.Abs(v-want) < 1e-9 }
