package cube

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/suaro/gorubik/internal/math3d"
)

func TestNewCubeIsSolved(t *testing.T) {
	c := New()
	if len(c.Cubies) != 26 {
		t.Fatalf("expected 26 visible cubies, got %d", len(c.Cubies))
	}
	if !c.IsSolved() {
		_, why := c.SolvedReport()
		t.Fatalf("a fresh cube must be solved:\n%s", why)
	}
	var corners, edges, centers int
	for _, cb := range c.Cubies {
		switch cb.Kind {
		case KindCorner:
			corners++
		case KindEdge:
			edges++
		case KindCenter:
			centers++
		}
	}
	if corners != 8 || edges != 12 || centers != 6 {
		t.Fatalf("expected 8/12/6 corner/edge/center, got %d/%d/%d", corners, edges, centers)
	}
	// Every cubie must carry exactly as many stickers as its kind implies, which
	// is 8*3 + 12*2 + 6*1 = 54 in total.
	total := 0
	for _, cb := range c.Cubies {
		n := len(cb.LocalStickerDirs())
		if n != int(cb.Kind)+1 {
			t.Errorf("cubie %d (%s) has %d stickers", cb.ID, cb.Kind, n)
		}
		total += n
	}
	if total != 54 {
		t.Fatalf("expected 54 stickers, got %d", total)
	}
}

func TestSolvedFaceDiagram(t *testing.T) {
	c := New()
	got := c.FaceString()
	want := "U WWW|WWW|WWW\nD YYY|YYY|YYY\nF GGG|GGG|GGG\nB BBB|BBB|BBB\nR RRR|RRR|RRR\nL OOO|OOO|OOO\n"
	if got != want {
		t.Fatalf("solved face diagram wrong\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStandardColorOrientation(t *testing.T) {
	want := map[Dir]Color{
		DirPY: ColorWhite, DirNY: ColorYellow, DirPZ: ColorGreen,
		DirNZ: ColorBlue, DirPX: ColorRed, DirNX: ColorOrange,
	}
	for d, c := range want {
		if got := SolvedFaceColor(d); got != c {
			t.Errorf("face %v: got %s want %s", d, got.Name(), c.Name())
		}
	}
}

// TestQuarterTurnConvention pins the exact integer matrices that the sign
// convention in moves.go produces.
func TestQuarterTurnConvention(t *testing.T) {
	// -90 about +X: y' = z, z' = -y
	q := math3d.QuarterTurn(0, -1)
	if got := q.Apply(VI(0, 1, 0)); got != VI(0, 0, -1) {
		t.Errorf("+Y under R: got %v want (0,0,-1)", got)
	}
	if got := q.Apply(VI(0, 0, 1)); got != VI(0, 1, 0) {
		t.Errorf("+Z under R: got %v want (0,1,0)", got)
	}
	// +90 about +Y: x' = z, z' = -x
	q = math3d.QuarterTurn(1, 1)
	if got := q.Apply(VI(0, 0, 1)); got != VI(1, 0, 0) {
		t.Errorf("+Z under +90Y: got %v want (1,0,0)", got)
	}
	// Four quarter turns about any axis are the identity.
	for axis := 0; axis < 3; axis++ {
		for _, quarter := range []int{-1, 1, 2} {
			m := math3d.QuarterTurn(axis, quarter)
			id := math3d.Identity3
			for i := 0; i < 4; i++ {
				id = id.Mul(m)
			}
			if !id.IsIdentity() {
				t.Errorf("axis %d quarter %d: four turns are not identity (%v)", axis, quarter, id)
			}
		}
	}
}

// TestNotationMatchesPhysicalCube checks the six letters against the behaviour a
// person holding a real cube would produce.
func TestNotationMatchesPhysicalCube(t *testing.T) {
	cases := []struct {
		move string
		// a cubie position and where that same cubie must end up
		from, to Vec3i
	}{
		// U clockwise carries the front-top row onto the left face.
		{"U", VI(0, 1, 1), VI(-1, 1, 0)},
		// D clockwise carries the front-bottom row onto the right face.
		{"D", VI(0, -1, 1), VI(1, -1, 0)},
		// R clockwise carries the front-right column up.
		{"R", VI(1, 0, 1), VI(1, 1, 0)},
		// L clockwise carries the up-left column to the front.
		{"L", VI(-1, 1, 0), VI(-1, 0, 1)},
		// F clockwise carries the top row to the right.
		{"F", VI(0, 1, 1), VI(1, 0, 1)},
		// B clockwise carries the top row of the back layer to the left face.
		{"B", VI(0, 1, -1), VI(-1, 0, -1)},
		// Primes run the other way.
		{"U'", VI(-1, 1, 0), VI(0, 1, 1)},
		{"R'", VI(1, 1, 0), VI(1, 0, 1)},
	}
	for _, tc := range cases {
		c := New()
		m, err := ParseMove(tc.move)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.move, err)
		}
		id := c.At(tc.from).ID
		c.Apply(m)
		cb := c.At(tc.to)
		if cb == nil || cb.ID != id {
			got := "nil"
			if cb != nil {
				got = fmtID(cb)
			}
			t.Errorf("%s: cubie from %v should land at %v, found %s", tc.move, tc.from, tc.to, got)
		}
	}
}

func fmtID(cb *Cubie) string { return strconv.Itoa(cb.ID) }

// TestStickersTravelWithTheirCubie is the anti-"repaint by position" test: after
// R, the corner that started at Down-Right-Front must present its own red, yellow
// and green stickers on Right, Front and Up.
func TestStickersTravelWithTheirCubie(t *testing.T) {
	c := New()
	drf := c.At(VI(1, -1, 1))
	if drf.Stickers[DirPX] != ColorRed || drf.Stickers[DirNY] != ColorYellow || drf.Stickers[DirPZ] != ColorGreen {
		t.Fatalf("precondition: DRF corner should be red/yellow/green, got %s/%s/%s",
			drf.Stickers[DirPX].Name(), drf.Stickers[DirNY].Name(), drf.Stickers[DirPZ].Name())
	}
	c.Apply(mustMove(t, "R"))

	moved := c.At(VI(1, 1, 1))
	if moved.ID != drf.ID {
		t.Fatalf("R should carry the DRF corner to the URF slot, found cubie %d", moved.ID)
	}
	// The sticker colours themselves must be untouched.
	if moved.Stickers[DirPX] != ColorRed || moved.Stickers[DirNY] != ColorYellow || moved.Stickers[DirPZ] != ColorGreen {
		t.Fatalf("sticker colours were rewritten during the turn")
	}
	// And they must now be presented on the faces a real cube would show.
	want := map[Dir]Color{DirPX: ColorRed, DirPZ: ColorYellow, DirPY: ColorGreen}
	for d, w := range want {
		if got := c.StickerOnGridFace(VI(1, 1, 1), d); got != w {
			t.Errorf("after R, URF slot face %v shows %s want %s", d, got.Name(), w.Name())
		}
	}
	// A sticker must never appear on a face the cubie has no sticker for.
	if got := c.StickerOnGridFace(VI(1, 1, 1), DirNX); got != ColorNone {
		t.Errorf("interior face reported a sticker: %s", got.Name())
	}
}

func TestEveryMoveMovesExactlyNineCubies(t *testing.T) {
	moves := []string{"R", "L", "U", "D", "F", "B", "R'", "L'", "U'", "D'", "F'", "B'", "R2", "U2", "F2"}
	for _, name := range moves {
		c := New()
		m := mustMove(t, name)
		beforePos := make(map[int]Vec3i, len(c.Cubies))
		beforeRot := make(map[int]Mat3i, len(c.Cubies))
		for _, cb := range c.Cubies {
			beforePos[cb.ID] = cb.Pos
			beforeRot[cb.ID] = cb.Rot
		}
		c.Apply(m)

		// Exactly nine cubies belong to the layer, and all nine are transformed.
		// Eight change grid position; the ninth is the layer center, which spins in
		// place. Cubies outside the layer must be untouched.
		movedPos, movedRot, outside := 0, 0, 0
		for _, cb := range c.Cubies {
			inLayer := beforePos[cb.ID][m.Axis()] == m.Layer()
			if inLayer {
				movedRot++
				if beforePos[cb.ID] != cb.Pos {
					movedPos++
				}
				if beforeRot[cb.ID] == cb.Rot {
					t.Errorf("%s: layer cubie %d kept its orientation", name, cb.ID)
				}
			} else {
				outside++
				if beforePos[cb.ID] != cb.Pos || beforeRot[cb.ID] != cb.Rot {
					t.Errorf("%s: cubie %d outside the layer moved", name, cb.ID)
				}
			}
		}
		if movedRot != 9 || movedPos != 8 {
			t.Errorf("%s: %d cubies turned / %d changed cell, want 9 / 8", name, movedRot, movedPos)
		}
		if outside != 17 {
			t.Errorf("%s: %d cubies outside the layer, want 17", name, outside)
		}
		if got := len(c.Layer(m.Axis(), m.Layer())); got != 9 {
			t.Errorf("%s layer has %d cubies, want 9", name, got)
		}
	}
}

func TestCentersNeverMove(t *testing.T) {
	c := New()
	for _, m := range mustSeq(t, scramble) {
		c.Apply(m)
	}
	for _, cb := range c.Cubies {
		if cb.Kind != KindCenter {
			continue
		}
		if cb.Pos != cb.Home {
			t.Errorf("center cubie %d drifted to %v", cb.ID, cb.Pos)
		}
		// A center may spin in place, but its sticker must still cover its own face.
		face, ok := DirOf(cb.Home)
		if !ok {
			t.Fatalf("center home %v is not a face direction", cb.Home)
		}
		if got := c.StickerOnGridFace(cb.Pos, face); got != SolvedFaceColor(face) {
			t.Errorf("center %d: face %v shows %s want %s", cb.ID, face,
				got.Name(), SolvedFaceColor(face).Name())
		}
	}
}

func TestFourRepeatOfAnyMoveRestoresSolved(t *testing.T) {
	for _, name := range []string{"R", "L", "U", "D", "F", "B", "R'", "U'"} {
		c := New()
		m := mustMove(t, name)
		for i := 0; i < 4; i++ {
			c.Apply(m)
			if i < 3 && c.IsSolved() {
				t.Errorf("%s returned to solved after %d turns", name, i+1)
			}
		}
		if !c.IsSolved() {
			_, why := c.SolvedReport()
			t.Errorf("four %s did not restore solved:\n%s", name, why)
		}
	}
}

// scramble and solve are the exact sequences from the project specification.
var scramble = "R U F' D L' B R' U' F D'"
var solve = "D F' U R B' L D' F U' R'"

func TestSpecifiedSolveInvertsSpecifiedScramble(t *testing.T) {
	sc := mustSeq(t, scramble)
	sv := mustSeq(t, solve)
	if len(sc) != 10 || len(sv) != 10 {
		t.Fatalf("expected 10 moves each, got %d and %d", len(sc), len(sv))
	}
	if got := InverseSequence(sc); FormatSequence(got) != FormatSequence(sv) {
		t.Fatalf("solve sequence is not the exact inverse\n got: %s\nwant: %s", FormatSequence(got), FormatSequence(sv))
	}
	c := New()
	for _, m := range sc {
		c.Apply(m)
	}
	if c.IsSolved() {
		t.Fatalf("the scramble must actually scramble the cube")
	}
	for _, m := range sv {
		c.Apply(m)
	}
	ok, why := c.SolvedReport()
	if !ok {
		t.Fatalf("cube not solved after the specified sequence:\n%s\nfaces:\n%s", why, c.FaceString())
	}
	if got := strings.Count(c.FaceString(), "\n"); got != 6 {
		t.Fatalf("face dump should have 6 lines, got %d", got)
	}
	if !strings.Contains(c.FaceString(), "U WWW|WWW|WWW") {
		t.Fatalf("final face dump is not the solved diagram:\n%s", c.FaceString())
	}
}

// TestRandomSequenceThenInverseIsExact is the drift test: a few thousand random
// moves followed by their inverse must land bit-exactly on the identity, because
// state is integer-only.
func TestRandomSequenceThenInverseIsExact(t *testing.T) {
	rng := rand.New(rand.NewSource(20261001))
	all := mustSeq(t, "R L U D F B R' L' U' D' F' B' R2 U2 F2")
	for trial := 0; trial < 40; trial++ {
		n := 1 + rng.Intn(120)
		seq := make([]Move, n)
		c := New()
		for i := range seq {
			seq[i] = all[rng.Intn(len(all))]
			c.Apply(seq[i])
		}
		for _, m := range InverseSequence(seq) {
			c.Apply(m)
		}
		if !c.IsSolved() {
			_, why := c.SolvedReport()
			t.Fatalf("trial %d (%d moves) did not return exactly solved:\n%s", trial, n, why)
		}
	}
}

// TestAnimatedAngleAgreesWithCommittedMatrix guarantees that when the animation
// reaches the end of a turn the float transform equals the integer transform, so
// committing the move cannot produce a visual pop.
func TestAnimatedAngleAgreesWithCommittedMatrix(t *testing.T) {
	for _, name := range []string{"R", "L", "U", "D", "F", "B", "R'", "U'", "F'", "R2"} {
		m := mustMove(t, name)
		want := m.Matrix().Mat4()
		got := math3d.RotAxis(m.Axis(), m.Angle())
		if !got.ApproxEqual(want, 1e-6) {
			t.Errorf("%s: animated end transform != committed transform\n got %v\nwant %v", name, got, want)
		}
		if math.Abs(float64(m.Angle())) > math.Pi+1e-6 {
			t.Errorf("%s: turn angle %v rad is larger than a half turn", name, m.Angle())
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{"", " ", "X", "RR", "R3", "r x", "M", "R'2"} {
		if _, err := ParseMove(bad); err == nil {
			t.Errorf("ParseMove(%q) should fail", bad)
		}
	}
	if _, err := ParseSequence("  "); err == nil {
		t.Errorf("ParseSequence of blank should fail")
	}
	got, err := ParseSequence("R,U F'  D'")
	if err != nil || FormatSequence(got) != "R U F' D'" {
		t.Errorf("ParseSequence: %v %v", FormatSequence(got), err)
	}
}

func mustMove(t *testing.T, s string) Move {
	t.Helper()
	m, err := ParseMove(s)
	if err != nil {
		t.Fatalf("ParseMove(%q): %v", s, err)
	}
	return m
}

func mustSeq(t *testing.T, s string) []Move {
	t.Helper()
	seq, err := ParseSequence(s)
	if err != nil {
		t.Fatalf("ParseSequence(%q): %v", s, err)
	}
	return seq
}
