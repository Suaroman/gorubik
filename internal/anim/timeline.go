package anim

import (
	"fmt"
	"math"

	"github.com/suaro/gorubik/internal/cube"
	"github.com/suaro/gorubik/internal/math3d"
)

// Phase is the stage of the presentation.
type Phase int

const (
	PhaseIntro Phase = iota // solved cube, slow rotation
	PhaseScramble
	PhasePaused // holding the scrambled state
	PhaseSolving
	PhaseOutro // solved again, rotating forever
)

func (p Phase) String() string {
	switch p {
	case PhaseIntro:
		return "solved"
	case PhaseScramble:
		return "scramble"
	case PhasePaused:
		return "scrambled pause"
	case PhaseSolving:
		return "solve"
	default:
		return "solved (endless)"
	}
}

// SpinConfig describes the whole-cube motion that runs through every phase. Three
// incommensurate rates keep it from reading like a turntable: a steady yaw, a slow
// pitch sway, and a faint roll.
type SpinConfig struct {
	Yaw0, YawRate     float32
	Pitch0, PitchAmp  float32
	PitchRate         float32
	RollAmp, RollRate float32
	RollPhase         float32
}

// DefaultSpin is roughly 6 degrees per second of yaw with a gentle sway.
func DefaultSpin() SpinConfig {
	return SpinConfig{
		Yaw0: -0.42, YawRate: 0.105,
		Pitch0: 0.27, PitchAmp: 0.085, PitchRate: 0.062,
		RollAmp: 0.030, RollRate: 0.047, RollPhase: 1.1,
	}
}

// Matrix returns the whole-cube orientation at time t. It depends only on t, so a
// captured frame is reproducible.
func (s SpinConfig) Matrix(t float32) math3d.Mat4 {
	yaw := s.Yaw0 + s.YawRate*t
	pitch := s.Pitch0 + s.PitchAmp*float32(math.Sin(float64(s.PitchRate)*float64(t)))
	roll := s.RollAmp * float32(math.Sin(float64(s.RollRate)*float64(t)+float64(s.RollPhase)))
	return math3d.RotY(yaw).Mul(math3d.RotX(pitch)).Mul(math3d.RotZ(roll))
}

// Config is the timeline's shape.
type Config struct {
	Intro  float64 // seconds of solved cube before the first turn
	Turn   float64 // seconds per face turn
	Gap    float64 // beat between consecutive turns
	Pause  float64 // seconds holding the scrambled cube
	Settle float64 // turn overshoot, as a fraction of the turn
	Spin   SpinConfig
}

// DefaultConfig matches the brief: 2 s intro, ~0.5 s per turn, 2 s pause.
func DefaultConfig() Config {
	return Config{
		Intro:  2.0,
		Turn:   0.50,
		Gap:    0.06,
		Pause:  2.0,
		Settle: 0.006, // 0.006 * 90 degrees = about half a degree
		Spin:   DefaultSpin(),
	}
}

// segment is one scheduled face turn.
type segment struct {
	start, end float64
	mv         cube.Move
	seq, idx   int // 0 = scramble, 1 = solve; idx within that sequence
}

// Player owns the cube and walks it through the timeline. Everything is derived
// from a single elapsed-time value, so Restart plus a fixed number of steps
// reproduces any moment exactly.
type Player struct {
	cfg  Config
	cube *cube.Cube
	seq  [2][]cube.Move

	segs      []segment
	t         float64
	committed int
	phase     Phase
	solveDone bool

	// Timeline boundaries, in seconds.
	scrambleEnd, pauseEnd, solveEnd float64
}

// New builds a player for the given sequences.
func New(scramble, solve []cube.Move, cfg Config) *Player {
	p := &Player{cfg: cfg, cube: cube.New(), seq: [2][]cube.Move{scramble, solve}}
	t := cfg.Intro
	for s := 0; s < 2; s++ {
		for i, mv := range p.seq[s] {
			seg := segment{start: t, end: t + cfg.Turn, mv: mv, seq: s, idx: i}
			p.segs = append(p.segs, seg)
			t = seg.end
			if i+1 < len(p.seq[s]) {
				t += cfg.Gap
			}
		}
		if s == 0 {
			p.scrambleEnd = t
			t += cfg.Pause
			p.pauseEnd = t
		}
	}
	p.solveEnd = t
	p.phase = PhaseIntro
	return p
}

// Restart returns to the solved cube at t=0.
func (p *Player) Restart() {
	p.cube = cube.New()
	p.t = 0
	p.committed = 0
	p.phase = PhaseIntro
	p.solveDone = false
}

// Update advances the timeline by dt seconds, committing every turn whose window
// has been passed. Committing from the schedule rather than from "the frame where
// progress reached 1" is what keeps the state correct when a frame is long.
func (p *Player) Update(dt float64) {
	p.t += dt
	for p.committed < len(p.segs) && p.segs[p.committed].end <= p.t {
		p.cube.Apply(p.segs[p.committed].mv)
		p.committed++
		if p.committed == len(p.segs) {
			p.solveDone = true
		}
	}
	p.phase = p.phaseAt(p.t)
}

func (p *Player) phaseAt(t float64) Phase {
	switch {
	case t < p.cfg.Intro:
		return PhaseIntro
	case t < p.scrambleEnd:
		return PhaseScramble
	case t < p.pauseEnd:
		return PhasePaused
	case t < p.solveEnd:
		return PhaseSolving
	default:
		return PhaseOutro
	}
}

// Turn is the in-flight face turn, or nil when nothing is rotating.
func (p *Player) Turn() *cube.LayerTurn {
	for i := range p.segs {
		s := &p.segs[i]
		if p.t < s.start || p.t >= s.end {
			continue
		}
		progress := (p.t - s.start) / (s.end - s.start)
		return &cube.LayerTurn{
			Move:  s.mv,
			Angle: float32(TurnAngle(progress, float64(s.mv.Angle()), p.cfg.Settle)),
		}
	}
	return nil
}

// Phase is the current stage.
func (p *Player) Phase() Phase { return p.phase }

// MoveName is the move turning right now, or "-" between turns.
func (p *Player) MoveName() string {
	for i := range p.segs {
		s := &p.segs[i]
		if p.t >= s.start && p.t < s.end {
			return fmt.Sprintf("%s (%s %d/%d)", s.mv, seqName(s.seq), s.idx+1, len(p.seq[s.seq]))
		}
	}
	return "-"
}

func seqName(s int) string {
	if s == 0 {
		return "scramble"
	}
	return "solve"
}

// Cube exposes the simulated state.
func (p *Player) Cube() *cube.Cube { return p.cube }

// Time is the elapsed timeline position in seconds.
func (p *Player) Time() float64 { return p.t }

// SolveCompleted reports whether the inverse sequence has finished.
func (p *Player) SolveCompleted() bool { return p.solveDone }

// Spin is the whole-cube orientation for the current time.
func (p *Player) Spin() math3d.Mat4 { return p.cfg.Spin.Matrix(float32(p.t)) }

// Timeline returns the phase boundaries, for the debug overlay and the README.
func (p *Player) Timeline() (intro, scrambleEnd, pauseEnd, solveEnd float64) {
	return p.cfg.Intro, p.scrambleEnd, p.pauseEnd, p.solveEnd
}
