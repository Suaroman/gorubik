package geo

import "math"

// StickerPlate builds one sticker as a thin, rounded-corner plate standing proud
// of the plastic: a flat top face at z=thickness, a chamfered rim, and a bottom
// face at z=0 that ends up buried in the cubie face (back-face culled, so it costs
// nothing but keeps the mesh closed).
//
// Building it as geometry instead of as a coloured quad buys three things the flat
// version cannot have: rounded corners, a black border that is real exposed plastic
// rather than a texture, and a chamfer that catches a fine bright line where the
// sticker meets the body.
//
//	halfSize      half width of the sticker
//	cornerRadius  radius of the rounded corners
//	thickness     height above the cubie face
//	chamfer       horizontal run of the chamfered rim
//	segs          segments per rounded corner
func StickerPlate(halfSize, cornerRadius, thickness, chamfer float32, segs int) *Mesh {
	m := NewMesh("sticker")
	b := newBuilder(m)
	t := float64(thickness)

	outline, _ := roundedRectOutline(float64(halfSize), float64(cornerRadius), segs)
	top, _ := roundedRectOutline(float64(halfSize-chamfer), float64(cornerRadius-chamfer), segs)
	n := len(outline)

	// Top face: a fan over the convex rounded rectangle, wound counter-clockwise
	// as seen from +Z so its normal points away from the cubie.
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		c := b.vert([3]float64{0, 0, t}, [3]float64{0, 0, 1})
		a := b.vert([3]float64{top[i][0], top[i][1], t}, [3]float64{0, 0, 1})
		d := b.vert([3]float64{top[j][0], top[j][1], t}, [3]float64{0, 0, 1})
		m.Idx = append(m.Idx, c, a, d)
	}
	// Chamfered rim. Its normal blends the outward direction with the upward slope
	// of the chamfer, which is what draws the thin highlight around a sticker.
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		o0, o1 := outline[i], outline[j]
		t0, t1 := top[i], top[j]
		edge := [3]float64{o1[0] - o0[0], o1[1] - o0[1], 0}
		profile := [3]float64{o0[0] - t0[0], o0[1] - t0[1], -t}
		rim := normalizeF(cross3(profile, edge))
		v0 := b.vert([3]float64{o0[0], o0[1], 0}, rim)
		v1 := b.vert([3]float64{o1[0], o1[1], 0}, rim)
		v2 := b.vert([3]float64{t0[0], t0[1], t}, rim)
		v3 := b.vert([3]float64{t1[0], t1[1], t}, rim)
		m.Idx = append(m.Idx, v0, v1, v2, v1, v3, v2)
	}
	// Bottom face, wound the other way so it points down into the cubie.
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		c := b.vert([3]float64{0, 0, 0}, [3]float64{0, 0, -1})
		a := b.vert([3]float64{outline[i][0], outline[i][1], 0}, [3]float64{0, 0, -1})
		d := b.vert([3]float64{outline[j][0], outline[j][1], 0}, [3]float64{0, 0, -1})
		m.Idx = append(m.Idx, c, d, a)
	}
	return m
}

// roundedRectOutline returns a counter-clockwise outline of a rounded rectangle
// centred on the origin, together with the matching outward 2D normals. Offsetting
// the rectangle inward by a constant keeps the normals identical, which is what
// lets the chamfer pair each outline point with its inset partner.
func roundedRectOutline(half, radius float64, segs int) ([][2]float64, [][2]float64) {
	c := half - radius
	corners := [4][2]float64{{c, -c}, {c, c}, {-c, c}, {-c, -c}}
	start := [4]float64{-math.Pi / 2, 0, math.Pi / 2, math.Pi}
	pts := make([][2]float64, 0, 4*segs)
	nrm := make([][2]float64, 0, 4*segs)
	for k := 0; k < 4; k++ {
		for s := 0; s < segs; s++ {
			a := start[k] + float64(s)/float64(segs)*(math.Pi/2)
			ca, sa := math.Cos(a), math.Sin(a)
			pts = append(pts, [2]float64{corners[k][0] + radius*ca, corners[k][1] + radius*sa})
			nrm = append(nrm, [2]float64{ca, sa})
		}
	}
	return pts, nrm
}

func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func normalizeF(n [3]float64) [3]float64 {
	l := math.Sqrt(dot(n, n))
	if l == 0 {
		return [3]float64{0, 0, 1}
	}
	return [3]float64{n[0] / l, n[1] / l, n[2] / l}
}
