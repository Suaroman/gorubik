package cube

import (
	"fmt"
	"math"
	"strings"

	"github.com/Suaroman/gorubik/internal/math3d"
)

// ---------------------------------------------------------------------------
// The authoritative move table.
//
// Coordinate system (right-handed, the same one the renderer uses):
//
//	+X right, +Y up, +Z toward the viewer
//	Right face = +X (red), Left = -X (orange), Up = +Y (white),
//	Down = -Y (yellow), Front = +Z (green), Back = -Z (blue)
//
// Notation rule, stated once and used everywhere:
//
//	A letter is a clockwise quarter turn of the face it names, seen from outside
//	that face. A clockwise turn seen from outside the face whose outward normal is
//	n is, by the right-hand rule, a rotation about n by -90 degrees.
//
//	Therefore Quarter() = -Turns * sign(face), expressed about the *positive* end
//	of the face's axis so that the animated float rotation (RotAxis) and the
//	committed integer rotation (QuarterTurn) are the same rotation.
//
// Worked check for U: sign(+Y) = +1, Turns = +1, so Quarter = -1 about +Y, which
// maps +Z -> -X, i.e. the front-top row moves to the left face. That is the
// textbook behaviour of U, and TestNotationMatchesPhysicalCube pins it.
// ---------------------------------------------------------------------------

// Move is a face turn: a face plus a number of clockwise quarter turns of that
// face (negative for counterclockwise, 2 for a half turn).
type Move struct {
	Face  Dir
	Turns int
}

// Axis returns the rotation axis index (0=X, 1=Y, 2=Z).
func (m Move) Axis() int { return m.Face.Axis() }

// Layer returns the grid coordinate (-1 or +1) of the nine cubies the turn moves.
func (m Move) Layer() int8 { return m.Face.Vec()[m.Face.Axis()] }

// Quarter returns the signed number of +90 degree right-hand rotations about the
// positive end of the axis. This is the one place the sign convention lives.
func (m Move) Quarter() int { return -m.Turns * int(m.Face.Sign()) }

// Matrix is the exact integer rotation the turn commits.
func (m Move) Matrix() Mat3i { return math3d.QuarterTurn(m.Axis(), m.Quarter()) }

// Angle is the same rotation as a float angle in radians about the positive axis,
// used to drive the animation so the animated and committed turns agree exactly.
func (m Move) Angle() float32 {
	return float32(m.Quarter()) * float32(math.Pi/2)
}

// Inverse returns the move that undoes this one.
func (m Move) Inverse() Move { return Move{Face: m.Face, Turns: -m.Turns} }

func (m Move) String() string {
	suffix := ""
	switch m.Turns {
	case 1:
	case -1:
		suffix = "'"
	case 2:
		suffix = "2"
	default:
		suffix = fmt.Sprintf("%d", m.Turns)
	}
	return m.Face.String() + suffix
}

// ParseMove parses a single turn in standard notation: R, L, U, D, F, B with an
// optional ' (counterclockwise) or 2 (half turn) suffix.
func ParseMove(s string) (Move, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Move{}, fmt.Errorf("empty move")
	}
	face, err := parseFace(t[0])
	if err != nil {
		return Move{}, err
	}
	turns := 1
	switch t[1:] {
	case "":
	case "'":
		turns = -1
	case "2":
		turns = 2
	default:
		return Move{}, fmt.Errorf("move %q: suffix %q must be ', 2 or nothing", s, t[1:])
	}
	return Move{Face: face, Turns: turns}, nil
}

// ParseSequence parses whitespace or comma separated moves.
func ParseSequence(s string) ([]Move, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == ';'
	})
	if len(fields) == 0 {
		return nil, fmt.Errorf("no moves in %q", s)
	}
	out := make([]Move, 0, len(fields))
	for _, f := range fields {
		m, err := ParseMove(f)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// InverseSequence returns the moves that undo seq, in reverse order.
func InverseSequence(seq []Move) []Move {
	out := make([]Move, 0, len(seq))
	for i := len(seq) - 1; i >= 0; i-- {
		out = append(out, seq[i].Inverse())
	}
	return out
}

// FormatSequence renders a move list back into notation.
func FormatSequence(seq []Move) string {
	parts := make([]string, len(seq))
	for i, m := range seq {
		parts[i] = m.String()
	}
	return strings.Join(parts, " ")
}

func parseFace(b byte) (Dir, error) {
	switch b {
	case 'R', 'r':
		return DirPX, nil
	case 'L', 'l':
		return DirNX, nil
	case 'U', 'u':
		return DirPY, nil
	case 'D', 'd':
		return DirNY, nil
	case 'F', 'f':
		return DirPZ, nil
	case 'B', 'b':
		return DirNZ, nil
	}
	return 0, fmt.Errorf("move letter %q must be one of R L U D F B", string(b))
}

func (d Dir) String() string {
	switch d {
	case DirPX:
		return "R"
	case DirNX:
		return "L"
	case DirPY:
		return "U"
	case DirNY:
		return "D"
	case DirPZ:
		return "F"
	default:
		return "B"
	}
}
