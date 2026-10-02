package geo

import "math"

// RoundedBox builds a cubie: a cube whose edges are rounded cylinders and whose
// corners are spherical octants, with analytic normals throughout.
//
// The flat faces stay exactly flat (so the cube reads as a cube and the sticker
// plane is true), while the bevel band carries a smooth normal ramp. That
// combination is what produces the thin travelling highlight along an edge as the
// cube rotates, which is the single biggest visual-quality lever in the whole
// scene.
//
//	h           half extent of the cubie
//	r           bevel radius, must be < h
//	cornerDepth midpoint-subdivision levels for each spherical corner. It also sets
//	            the edge resolution to 2^cornerDepth: midpoint subdivision places
//	            points along a corner's arc at even angle steps, so matching the two
//	            counts is what lets bevels and corners weld into one watertight
//	            surface instead of meeting at T-junctions.
func RoundedBox(h, r float32, cornerDepth int) *Mesh {
	edgeSegs := 1 << cornerDepth
	m := NewMesh("roundedbox")
	b := newBuilder(m)
	inner := float64(h - r)
	hh, rr := float64(h), float64(r)

	// Six flat faces, each with the same tangent basis FaceTransforms uses so the
	// sticker's local frame lines up with the plastic underneath it.
	xs, ys, zs := faceBases()
	for i := 0; i < 6; i++ {
		x, y, z := xs[i], ys[i], zs[i]
		b.patch(1, 1, func(u, v float64) ([3]float64, [3]float64) {
			a, c := (u*2-1)*inner, (v*2-1)*inner
			p := add(scale(x, a), add(scale(y, c), scale(z, hh)))
			return p, z
		})
	}

	// Twelve quarter-cylinder edges, one per perpendicular pair of faces.
	for i := 0; i < 6; i++ {
		for j := i + 1; j < 6; j++ {
			a, bb := zs[i], zs[j]
			if dot(a, bb) != 0 {
				continue
			}
			c := cross(a, bb) // the free axis of this edge
			b.patch(edgeSegs, 1, func(u, v float64) ([3]float64, [3]float64) {
				theta := u * math.Pi / 2
				n := add(scale(a, math.Cos(theta)), scale(bb, math.Sin(theta)))
				p := add(scale(a, inner+rr*math.Cos(theta)),
					add(scale(bb, inner+rr*math.Sin(theta)), scale(c, (v*2-1)*inner)))
				return p, n
			})
		}
	}

	// Eight corners, each a spherical triangle between the three face normals of
	// that octant, refined by midpoint subdivision. A latitude/longitude octant
	// would collapse every row to a single point at the pole and produce
	// degenerate triangles; a subdivided spherical triangle has no such singularity.
	for sx := -1.0; sx <= 1; sx += 2 {
		for sy := -1.0; sy <= 1; sy += 2 {
			for sz := -1.0; sz <= 1; sz += 2 {
				a := [3]float64{sx, 0, 0}
				bb := [3]float64{0, sy, 0}
				c := [3]float64{0, 0, sz}
				if sx*sy*sz < 0 {
					bb, c = c, bb // keep outward winding in the mirrored octants
				}
				cx, cy, cz := sx*inner, sy*inner, sz*inner
				subdivideSphereTri(b, a, bb, c, cornerDepth, func(n [3]float64) [3]float64 {
					return [3]float64{cx + rr*n[0], cy + rr*n[1], cz + rr*n[2]}
				})
			}
		}
	}
	return m
}

// subdivideSphereTri emits a spherical triangle and its refinements. Normals are
// the unit direction itself, so the corner is exactly spherical.
func subdivideSphereTri(b *builder, a, bb, c [3]float64, depth int, place func([3]float64) [3]float64) {
	if depth == 0 {
		b.tri(place(a), a, place(bb), bb, place(c), c)
		return
	}
	ab, bc, ca := nmid(a, bb), nmid(bb, c), nmid(c, a)
	subdivideSphereTri(b, a, ab, ca, depth-1, place)
	subdivideSphereTri(b, ab, bb, bc, depth-1, place)
	subdivideSphereTri(b, ca, bc, c, depth-1, place)
	subdivideSphereTri(b, ab, bc, ca, depth-1, place)
}

// tri appends one triangle, reusing builder's welding.
func (b *builder) tri(p0, n0, p1, n1, p2, n2 [3]float64) {
	i0 := b.vert(p0, n0)
	i1 := b.vert(p1, n1)
	i2 := b.vert(p2, n2)
	b.m.Idx = append(b.m.Idx, i0, i1, i2)
}

func nmid(a, b [3]float64) [3]float64 {
	m := add(a, b)
	l := math.Sqrt(dot(m, m))
	return [3]float64{m[0] / l, m[1] / l, m[2] / l}
}

// faceBases returns, per cube face, a right-handed tangent frame whose third
// vector is the face normal.
func faceBases() (xs, ys, zs [6][3]float64) {
	xs = [6][3]float64{{0, 0, -1}, {0, 0, 1}, {1, 0, 0}, {-1, 0, 0}, {1, 0, 0}, {-1, 0, 0}}
	zs = [6][3]float64{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	for i := range xs {
		ys[i] = cross(zs[i], xs[i])
	}
	return xs, ys, zs
}

func add(a, b [3]float64) [3]float64 { return [3]float64{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func scale(a [3]float64, s float64) [3]float64 {
	return [3]float64{a[0] * s, a[1] * s, a[2] * s}
}
func cross(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
