// Package cube is the Rubik's Cube simulator. It owns the logical state of the
// puzzle: 26 cubies, each with a persistent identity, an integer grid position,
// an exact integer orientation and a fixed set of stickers.
//
// Two invariants drive the whole design:
//
//   - Sticker colours are attached to a cubie's *local* faces and are never
//     recomputed from the cubie's current position. A sticker can therefore only
//     move by moving its cubie.
//   - Every committed turn is a multiplication by an exact integer rotation
//     matrix, so grid coordinates stay in {-1,0,+1} and orientations stay exactly
//     one of the 24 proper rotations, with no floating point anywhere.
package cube

import (
	"fmt"
	"strings"

	"github.com/suaro/gorubik/internal/math3d"
)

// Dir identifies one of the six axis directions. It doubles as the identity of
// a cube face (a face is its outward normal) and as the index of a cubie's local
// face, which is why a sticker can be stored in a fixed 6-element array.
type Dir int8

const (
	DirPX Dir = iota // +X, the Right face
	DirNX            // -X, the Left face
	DirPY            // +Y, the Up face
	DirNY            // -Y, the Down face
	DirPZ            // +Z, the Front face
	DirNZ            // -Z, the Back face
)

// AllDirs lists the six directions in a stable order.
var AllDirs = [6]Dir{DirPX, DirNX, DirPY, DirNY, DirPZ, DirNZ}

// Vec returns the direction as an integer unit vector.
func (d Dir) Vec() Vec3i {
	switch d {
	case DirPX:
		return Vec3i{1, 0, 0}
	case DirNX:
		return Vec3i{-1, 0, 0}
	case DirPY:
		return Vec3i{0, 1, 0}
	case DirNY:
		return Vec3i{0, -1, 0}
	case DirPZ:
		return Vec3i{0, 0, 1}
	default:
		return Vec3i{0, 0, -1}
	}
}

// VI builds an integer vector, re-exported for callers of this package.
func VI(x, y, z int8) Vec3i { return math3d.VI(x, y, z) }

// DirOf maps an axis-aligned unit vector back to its Dir.
func DirOf(v Vec3i) (Dir, bool) {
	for _, d := range AllDirs {
		if d.Vec() == v {
			return d, true
		}
	}
	return 0, false
}

// AxisOf returns the axis index of a direction.
func (d Dir) Axis() int { return d.Vec().Axis() }

// Sign returns +1 for the positive faces (R, U, F) and -1 for (L, D, B).
func (d Dir) Sign() int8 { return d.Vec().Sign() }

// Color names the six sticker colours plus "no sticker".
type Color int8

const (
	ColorNone Color = iota
	ColorWhite
	ColorYellow
	ColorRed
	ColorOrange
	ColorGreen
	ColorBlue
)

// Name returns the colour's English name, used by the debug overlay and tests.
func (c Color) Name() string {
	switch c {
	case ColorWhite:
		return "white"
	case ColorYellow:
		return "yellow"
	case ColorRed:
		return "red"
	case ColorOrange:
		return "orange"
	case ColorGreen:
		return "green"
	case ColorBlue:
		return "blue"
	default:
		return "none"
	}
}

// SolvedFaceColor is the single authoritative mapping from a face direction to
// its sticker colour in the standard orientation:
//
//	Up = white, Down = yellow, Front = green, Back = blue, Right = red, Left = orange
func SolvedFaceColor(d Dir) Color {
	switch d {
	case DirPY:
		return ColorWhite
	case DirNY:
		return ColorYellow
	case DirPZ:
		return ColorGreen
	case DirNZ:
		return ColorBlue
	case DirPX:
		return ColorRed
	default:
		return ColorOrange
	}
}

// Kind classifies a cubie by how many stickers it carries.
type Kind int8

const (
	KindCenter Kind = iota
	KindEdge
	KindCorner
)

func (k Kind) String() string {
	switch k {
	case KindCenter:
		return "center"
	case KindEdge:
		return "edge"
	default:
		return "corner"
	}
}

// Cubie is one of the 26 visible plastic blocks.
type Cubie struct {
	ID   int
	Kind Kind

	// Home is the grid position the cubie occupies on a solved cube. It never
	// changes and is what the solved-state check compares against.
	Home Vec3i
	// Pos is the current integer grid position.
	Pos Vec3i
	// Rot maps the cubie's local axes onto the grid axes. Identity means the
	// cubie is unrotated, so local face d currently points along grid face d.
	Rot Mat3i

	// Stickers is indexed by *local* face direction. It is assigned once at
	// construction and never modified afterwards.
	Stickers [6]Color
}

// LocalStickerDirs lists the local faces that carry a sticker.
func (c *Cubie) LocalStickerDirs() []Dir {
	var out []Dir
	for _, d := range AllDirs {
		if c.Stickers[d] != ColorNone {
			out = append(out, d)
		}
	}
	return out
}

// GridDirOfSticker reports which grid direction the sticker sitting on local
// face d currently faces. This is only ever used for verification and debug
// output; rendering transforms the sticker with the cubie's matrix instead.
func (c *Cubie) GridDirOfSticker(d Dir) Dir {
	g, _ := DirOf(c.Rot.Apply(d.Vec()))
	return g
}

// Cube is the whole puzzle.
type Cube struct {
	Cubies    []*Cubie
	byPos     map[Vec3i]*Cubie
	moveCount int
}

// Vec3i and Mat3i are re-exported so callers of this package do not need to
// reach into the math package for the exact integer types.
type (
	Vec3i = math3d.Vec3i
	Mat3i = math3d.Mat3i
)

// New builds a solved cube: 26 cubies on the {-1,0,+1}^3 grid minus the hidden
// core, each carrying the stickers its home position implies.
func New() *Cube {
	c := &Cube{byPos: make(map[Vec3i]*Cubie, 26)}
	id := 0
	for x := int8(-1); x <= 1; x++ {
		for y := int8(-1); y <= 1; y++ {
			for z := int8(-1); z <= 1; z++ {
				p := Vec3i{x, y, z}
				if p == (Vec3i{0, 0, 0}) {
					continue // hidden core
				}
				cb := &Cubie{ID: id, Home: p, Pos: p, Rot: math3d.Identity3}
				cb.Kind = Kind(cb.Home.StickerCount() - 1) // 1 sticker = center ... 3 = corner
				for _, d := range AllDirs {
					// On a solved cube local faces align with grid faces, so the
					// sticker on local face d is simply the colour of that face.
					if cb.Home[d.Axis()] == d.Vec()[d.Axis()] {
						cb.Stickers[d] = SolvedFaceColor(d)
					}
				}
				c.Cubies = append(c.Cubies, cb)
				c.byPos[p] = cb
				id++
			}
		}
	}
	return c
}

// At returns the cubie currently occupying a grid position.
func (c *Cube) At(p Vec3i) *Cubie { return c.byPos[p] }

// Layer returns the nine cubies whose coordinate on the given axis equals coord.
func (c *Cube) Layer(axis int, coord int8) []*Cubie {
	var out []*Cubie
	for _, cb := range c.Cubies {
		if cb.Pos[axis] == coord {
			out = append(out, cb)
		}
	}
	return out
}

// Apply commits a move to the logical state. Positions and orientations are
// rotated by the exact integer quarter-turn matrix of the move, so the state
// stays exact no matter how many moves are played.
func (c *Cube) Apply(m Move) {
	q := m.Matrix()
	axis, coord := m.Axis(), m.Layer()
	for _, cb := range c.Cubies {
		if cb.Pos[axis] != coord {
			continue
		}
		cb.Pos = q.Apply(cb.Pos)
		cb.Rot = q.Mul(cb.Rot)
	}
	c.reindex()
	c.moveCount++
}

// reindex rebuilds the position lookup. It runs after every committed turn so
// that At and StickerOnGridFace can never read a stale cell, and it doubles as an
// invariant check: 26 cubies must occupy 26 distinct cells.
func (c *Cube) reindex() {
	c.byPos = make(map[Vec3i]*Cubie, len(c.Cubies))
	for _, cb := range c.Cubies {
		c.byPos[cb.Pos] = cb
	}
	if len(c.byPos) != len(c.Cubies) {
		panic("cube invariant violated: two cubies occupy the same grid cell")
	}
}

// MoveCount is how many moves have been committed.
func (c *Cube) MoveCount() int { return c.moveCount }

// IsSolved reports whether every cubie is home and unrotated. Because
// orientations are exact integer matrices this is a bit-exact comparison, not a
// tolerance test.
func (c *Cube) IsSolved() bool {
	for _, cb := range c.Cubies {
		if cb.Pos != cb.Home || !cb.Rot.IsIdentity() {
			return false
		}
	}
	return true
}

// SolvedReport returns a human readable diagnosis for the verification step,
// naming the first few cubies that are wrong and how they are wrong.
func (c *Cube) SolvedReport() (bool, string) {
	var bad []string
	for _, cb := range c.Cubies {
		if cb.Pos == cb.Home && cb.Rot.IsIdentity() {
			continue
		}
		bad = append(bad, fmt.Sprintf(
			"cubie %d (%s home %v): pos %v rot %v", cb.ID, cb.Kind, cb.Home, cb.Pos, cb.Rot))
	}
	if len(bad) == 0 {
		return true, fmt.Sprintf("all %d cubies home and unrotated after %d moves", len(c.Cubies), c.moveCount)
	}
	if len(bad) > 6 {
		bad = append(bad[:6], fmt.Sprintf("... and %d more", len(bad)-6))
	}
	return false, strings.Join(bad, "\n  ")
}

// StickerOnGridFace returns the colour currently presented on grid direction d by
// the cubie at grid position p, or ColorNone. Derived purely from identity,
// position and orientation, so it is a real check on the simulator rather than a
// re-derivation of the colour.
func (c *Cube) StickerOnGridFace(p Vec3i, d Dir) Color {
	cb := c.byPos[p]
	if cb == nil {
		return ColorNone
	}
	// Find the local face whose sticker currently points along d.
	for _, local := range cb.LocalStickerDirs() {
		if cb.GridDirOfSticker(local) == d {
			return cb.Stickers[local]
		}
	}
	return ColorNone
}

// faceView describes how one face is read when looked at from outside the cube:
// its outward normal plus the grid directions that appear to the right and above
// on the page. This keeps the printed diagram consistent with the notation table
// in moves.go instead of hard-coding 54 coordinates.
type faceView struct {
	name  Dir
	right Vec3i
	up    Vec3i
}

var faceViews = []faceView{
	{DirPY, VI(1, 0, 0), VI(0, 0, -1)}, // U: back edge at the top
	{DirNY, VI(1, 0, 0), VI(0, 0, 1)},  // D: front edge at the top
	{DirPZ, VI(1, 0, 0), VI(0, 1, 0)},  // F
	{DirNZ, VI(-1, 0, 0), VI(0, 1, 0)}, // B
	{DirPX, VI(0, 0, -1), VI(0, 1, 0)}, // R
	{DirNX, VI(0, 0, 1), VI(0, 1, 0)},  // L
}

// FaceString renders the six faces as colour initials in standard reading order
// (top row first, left to right), for the debug overlay and the verification log.
func (c *Cube) FaceString() string {
	initial := func(col Color) byte {
		switch col {
		case ColorWhite:
			return 'W'
		case ColorYellow:
			return 'Y'
		case ColorRed:
			return 'R'
		case ColorOrange:
			return 'O'
		case ColorGreen:
			return 'G'
		case ColorBlue:
			return 'B'
		default:
			return '.'
		}
	}
	var b strings.Builder
	for _, fv := range faceViews {
		n := fv.name.Vec()
		fmt.Fprintf(&b, "%s ", fv.name.String())
		for row := int8(1); row >= -1; row-- {
			for col := int8(-1); col <= 1; col++ {
				p := n.Add(fv.right.MulS(col)).Add(fv.up.MulS(row))
				b.WriteByte(initial(c.StickerOnGridFace(p, fv.name)))
			}
			if row > -1 {
				b.WriteByte('|')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
