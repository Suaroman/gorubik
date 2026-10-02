// Package geo builds the procedural meshes: a beveled cubie body and a rounded,
// slightly raised sticker plate. Both are generated analytically so their normals
// are exact rather than approximated by a smoothing pass, which is what makes the
// edges catch a narrow highlight as the cube turns.
package geo

import (
	"fmt"
	"math"

	"github.com/suaro/gorubik/internal/math3d"
)

// Vertex is one mesh vertex: a position and its analytic unit normal.
type Vertex struct {
	Pos    [3]float32
	Normal [3]float32
}

// Mesh is a triangle list with 32-bit indices.
type Mesh struct {
	Verts []Vertex
	Idx   []uint32

	name string
}

// NewMesh returns an empty mesh. name only appears in error messages.
func NewMesh(name string) *Mesh { return &Mesh{name: name} }

// Interleaved returns the vertex buffer as [px py pz nx ny nz] triples, the exact
// layout the vertex shader expects.
func (m *Mesh) Interleaved() []float32 {
	out := make([]float32, 0, len(m.Verts)*6)
	for _, v := range m.Verts {
		out = append(out, v.Pos[0], v.Pos[1], v.Pos[2], v.Normal[0], v.Normal[1], v.Normal[2])
	}
	return out
}

// VertexStride is the byte stride of Interleaved.
const VertexStride = 6 * 4

// TriCount is the number of triangles.
func (m *Mesh) TriCount() int { return len(m.Idx) / 3 }

// IdxCount is the number of indices, which is what glDrawElements* consumes. It is
// deliberately a separate call from TriCount: passing the triangle count draws the
// first third of the mesh and looks, at a glance, like a lighting bug.
func (m *Mesh) IdxCount() int { return len(m.Idx) }

// vkey identifies a vertex by position *and* normal. Welding on both keeps a
// smooth seam shared (the bevel meets a flat face with a matching normal, so those
// vertices merge and shade continuously) while a genuine crease, such as a sticker
// top meeting its chamfer, stays split.
type vkey struct {
	p [3]int32
	n [3]int32
}

// builder accumulates parametric patches and welds coincident same-normal
// vertices.
type builder struct {
	m    *Mesh
	keys map[vkey]uint32
}

func newBuilder(m *Mesh) *builder { return &builder{m: m, keys: map[vkey]uint32{}} }

// quantize welds values that agree to well under a micron of the model unit.
func quantize3(p [3]float64) [3]int32 {
	const s = 1e6
	return [3]int32{int32(math.Round(p[0] * s)), int32(math.Round(p[1] * s)), int32(math.Round(p[2] * s))}
}

func (b *builder) vert(p, n [3]float64) uint32 {
	nn := normalize(n)
	k := vkey{p: quantize3(p), n: [3]int32{quantize3(n)[0], quantize3(n)[1], quantize3(n)[2]}}
	if id, ok := b.keys[k]; ok {
		return id
	}
	id := uint32(len(b.m.Verts))
	b.m.Verts = append(b.m.Verts, Vertex{
		Pos:    [3]float32{float32(p[0]), float32(p[1]), float32(p[2])},
		Normal: nn,
	})
	b.keys[k] = id
	return id
}

// patch emits a (nu x nv) grid of quads over f. Winding is (i,j), (i+1,j),
// (i+1,j+1), (i,j+1), which faces outward when cross(df/du, df/dv) does.
func (b *builder) patch(nu, nv int, f func(u, v float64) ([3]float64, [3]float64)) {
	prev := make([]uint32, nu+1)
	row := make([]uint32, nu+1)
	for j := 0; j <= nv; j++ {
		v := float64(j) / float64(nv)
		for i := 0; i <= nu; i++ {
			u := float64(i) / float64(nu)
			p, n := f(u, v)
			row[i] = b.vert(p, n)
		}
		if j > 0 {
			for i := 0; i < nu; i++ {
				a, c := row[i], row[i+1]   // current row: left, right
				d, e := prev[i], prev[i+1] // previous row: left, right
				b.m.Idx = append(b.m.Idx, a, d, e, a, e, c)
			}
		}
		prev, row = row, prev
	}
}

// Bounds returns the axis-aligned bounding box of the mesh.
func (m *Mesh) Bounds() (lo, hi [3]float32) {
	lo = [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	hi = [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
	for _, v := range m.Verts {
		for i := 0; i < 3; i++ {
			if v.Pos[i] < lo[i] {
				lo[i] = v.Pos[i]
			}
			if v.Pos[i] > hi[i] {
				hi[i] = v.Pos[i]
			}
		}
	}
	return lo, hi
}

// Radius returns the largest distance from the origin to any vertex, used to size
// the shadow frustum.
func (m *Mesh) Radius() float32 {
	r := float32(0)
	for _, v := range m.Verts {
		d := float32(v.Pos[0])*v.Pos[0] + float32(v.Pos[1])*v.Pos[1] + float32(v.Pos[2])*v.Pos[2]
		if d > r {
			r = d
		}
	}
	return float32(math.Sqrt(float64(r)))
}

// errTol is the tolerance shared by the mesh self-checks.
const errTol = 1e-4

// Check validates the mesh the way a mesh should be validated before it is ever
// rendered: unit normals, no degenerate triangles, winding that agrees with the
// normals, a watertight surface, positive enclosed volume, and a bounding box that
// matches what the generator was asked for. Running this in a test is what lets me
// trust the procedural geometry without staring at a GPU first.
func (m *Mesh) Check(wantLo, wantHi [3]float32) error {
	if len(m.Verts) == 0 || len(m.Idx) == 0 || len(m.Idx)%3 != 0 {
		return fmt.Errorf("%s: empty or ragged mesh (%d verts, %d indices)", m.name, len(m.Verts), len(m.Idx))
	}
	for i, v := range m.Verts {
		l := math.Sqrt(dot([3]float64{float64(v.Normal[0]), float64(v.Normal[1]), float64(v.Normal[2])},
			[3]float64{float64(v.Normal[0]), float64(v.Normal[1]), float64(v.Normal[2])}))
		if math.Abs(l-1) > errTol {
			return fmt.Errorf("%s: vertex %d normal length %v", m.name, i, l)
		}
	}
	used := map[[2][3]int32]int{}
	var vol float64
	for t := 0; t < len(m.Idx); t += 3 {
		a, b, c := m.Verts[m.Idx[t]], m.Verts[m.Idx[t+1]], m.Verts[m.Idx[t+2]]
		face := triNormal(a.Pos, b.Pos, c.Pos)
		if len2(face) < 1e-10 {
			return fmt.Errorf("%s: degenerate triangle at index %d", m.name, t)
		}
		avg := [3]float64{
			float64(a.Normal[0] + b.Normal[0] + c.Normal[0]),
			float64(a.Normal[1] + b.Normal[1] + c.Normal[1]),
			float64(a.Normal[2] + b.Normal[2] + c.Normal[2]),
		}
		if face[0]*avg[0]+face[1]*avg[1]+face[2]*avg[2] <= 0 {
			return fmt.Errorf("%s: triangle at index %d winds opposite to its normals", m.name, t)
		}
		vol += det3(a.Pos, b.Pos, c.Pos)
		// Watertightness is measured on positions, not indices, so a legitimately
		// split hard edge still counts as closed.
		pos := [3][3]int32{quantize3(toF(a.Pos)), quantize3(toF(b.Pos)), quantize3(toF(c.Pos))}
		for _, e := range [3][2][3]int32{{pos[0], pos[1]}, {pos[1], pos[2]}, {pos[2], pos[0]}} {
			if e[0] == e[1] {
				return fmt.Errorf("%s: triangle at index %d has a zero-length edge", m.name, t)
			}
			used[e]++
		}
	}
	for e, n := range used {
		if n != 1 {
			return fmt.Errorf("%s: directed edge %v used %d times", m.name, e, n)
		}
		if used[[2][3]int32{e[1], e[0]}] != 1 {
			return fmt.Errorf("%s: edge %v has no opposite twin (surface not watertight)", m.name, e)
		}
	}
	if vol <= 0 {
		return fmt.Errorf("%s: enclosed volume %v is not positive (inverted mesh?)", m.name, vol)
	}
	lo, hi := m.Bounds()
	for i := 0; i < 3; i++ {
		if math.Abs(float64(lo[i]-wantLo[i])) > 1e-4 || math.Abs(float64(hi[i]-wantHi[i])) > 1e-4 {
			return fmt.Errorf("%s: bounds lo %v hi %v, want lo %v hi %v", m.name, lo, hi, wantLo, wantHi)
		}
	}
	return nil
}

func toF(p [3]float32) [3]float64 {
	return [3]float64{float64(p[0]), float64(p[1]), float64(p[2])}
}

func triNormal(a, b, c [3]float32) [3]float64 {
	ab := [3]float64{float64(b[0] - a[0]), float64(b[1] - a[1]), float64(b[2] - a[2])}
	ac := [3]float64{float64(c[0] - a[0]), float64(c[1] - a[1]), float64(c[2] - a[2])}
	return [3]float64{
		ab[1]*ac[2] - ab[2]*ac[1],
		ab[2]*ac[0] - ab[0]*ac[2],
		ab[0]*ac[1] - ab[1]*ac[0],
	}
}

func det3(a, b, c [3]float32) float64 {
	return float64(a[0])*(float64(b[1])*float64(c[2])-float64(b[2])*float64(c[1])) -
		float64(a[1])*(float64(b[0])*float64(c[2])-float64(b[2])*float64(c[0])) +
		float64(a[2])*(float64(b[0])*float64(c[1])-float64(b[1])*float64(c[0]))
}

func len2(v [3]float64) float64 { return v[0]*v[0] + v[1]*v[1] + v[2]*v[2] }

func normalize(n [3]float64) [3]float32 {
	l := math.Sqrt(len2(n))
	if l == 0 {
		return [3]float32{0, 0, 1}
	}
	return [3]float32{float32(n[0] / l), float32(n[1] / l), float32(n[2] / l)}
}

// FaceTransforms returns, per local cube face, the model transform that places a
// sticker built on the +Z plane onto that face. The sticker's own thickness lifts
// it clear of the plastic, so no coplanar surface is left to z-fight.
//
// The face order matches geo.faceBases and cube.AllDirs: +X -X +Y -Y +Z -Z.
func FaceTransforms(half float32) [6]math3d.Mat4 {
	xs, ys, zs := faceBases()
	var out [6]math3d.Mat4
	for i := 0; i < 6; i++ {
		var m math3d.Mat4
		m[0], m[1], m[2] = float32(xs[i][0]), float32(xs[i][1]), float32(xs[i][2])
		m[4], m[5], m[6] = float32(ys[i][0]), float32(ys[i][1]), float32(ys[i][2])
		m[8], m[9], m[10] = float32(zs[i][0]), float32(zs[i][1]), float32(zs[i][2])
		m[12] = float32(zs[i][0]) * half
		m[13] = float32(zs[i][1]) * half
		m[14] = float32(zs[i][2]) * half
		m[15] = 1
		out[i] = m
	}
	return out
}
