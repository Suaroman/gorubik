package anim

import (
	"fmt"
	"math"

	"github.com/Suaroman/gorubik/internal/cube"
	"github.com/Suaroman/gorubik/internal/math3d"
)

// Phase is the stage of the presentation.
type Phase int

const (
	PhaseIntro Phase = iota // solved cube, slow rotation
	PhaseScramble
	PhasePaused // holding the scrambled state
	PhaseSolving
	PhaseOutro // solved again, rotating, until the next cycle begins
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

// DefaultSpin is roughly 12 degrees per second of yaw with a gentle sway.
func DefaultSpin() SpinConfig {
	return SpinConfig{
		Yaw0: -0.42, YawRate: 0.21,
		Pitch0: 0.27, PitchAmp: 0.085, PitchRate: 0.124,
		RollAmp: 0.030, RollRate: 0.094, RollPhase: 1.1,
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
	Hold   float64 // seconds holding the solved cube before the next cycle
	Settle float64 // turn overshoot, as a fraction of the turn
	Spin   SpinConfig
}

// DefaultConfig matches the brief: 2 s intro, ~0.5 s per turn, 2 s pause, and a
// 10 s solved hold before each repeat of the scramble/solve cycle.
func DefaultConfig() Config {
	return Config{
		Intro:  2.0,
		Turn:   0.50,
		Gap:    0.06,
		Pause:  2.0,
		Hold:   10.0,
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
//
// The presentation loops: one cycle is scramble, hold-the-scramble, solve, then a
// solved pause of cfg.Hold seconds, after which the same cycle starts again on a
// freshly solved cube and continues forever. The segment schedule is stored once,
// relative to unitOrigin (the moment the current cycle's scramble begins), and the
// whole spin depends on spinT, which never wraps — so the cube's global rotation
// carries across cycle boundaries without a snap.
type Player struct {
	cfg  Config
	cube *cube.Cube
	seq  [2][]cube.Move

	segs       []segment // times relative to unitOrigin
	t          float64   // absolute elapsed time
	unitOrigin float64   // t at which the current cycle's scramble starts
	spinT      float64   // monotonic clock for the whole-cube spin; never wraps
	committed  int
	cycles     int // completed cycles
	phase      Phase
	solveDone  bool

	// Cycle boundaries, in seconds, relative to unitOrigin.
	scrambleEnd, pauseEnd, solveEnd, cycleEnd float64
}

// New builds a player for the given sequences.
func New(scramble, solve []cube.Move, cfg Config) *Player {
	p := &Player{cfg: cfg, cube: cube.New(), seq: [2][]cube.Move{scramble, solve}}
	t := 0.0
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
	p.cycleEnd = t + cfg.Hold
	p.unitOrigin = cfg.Intro // the first cycle keeps the solved-cube intro
	p.phase = PhaseIntro
	return p
}

// Restart returns to the solved cube at t=0.
func (p *Player) Restart() {
	p.cube = cube.New()
	p.t = 0
	p.spinT = 0
	p.unitOrigin = p.cfg.Intro
	p.committed = 0
	p.cycles = 0
	p.phase = PhaseIntro
	p.solveDone = false
}

// Update advances the timeline by dt seconds, committing every turn whose window
// has been passed. Committing from the schedule rather than from "the frame where
// progress reached 1" is what keeps the state correct when a frame is long. When
// the solved hold runs out the cycle repeats: the schedule shifts forward by one
// cycle length and the cube starts again from a guaranteed-solved state, while the
// spin clock (and therefore the global orientation) keeps running.
func (p *Player) Update(dt float64) {
	p.t += dt
	p.spinT += dt
	for {
		lt := p.t - p.unitOrigin
		for p.committed < len(p.segs) && p.segs[p.committed].end <= lt {
			p.cube.Apply(p.segs[p.committed].mv)
			p.committed++
			if p.committed == len(p.segs) {
				p.solveDone = true
			}
		}
		if lt < p.cycleEnd {
			break
		}
		p.unitOrigin += p.cycleEnd
		p.cube = cube.New()
		p.committed = 0
		p.solveDone = false
		p.cycles++
	}
	p.phase = p.phaseAt(p.t - p.unitOrigin)
}

// phaseAt takes the time within the current cycle; it is negative only during the
// first cycle's intro.
func (p *Player) phaseAt(lt float64) Phase {
	switch {
	case lt < 0:
		return PhaseIntro
	case lt < p.scrambleEnd:
		return PhaseScramble
	case lt < p.pauseEnd:
		return PhasePaused
	case lt < p.solveEnd:
		return PhaseSolving
	default:
		return PhaseOutro
	}
}

// Turn is the in-flight face turn, or nil when nothing is rotating.
func (p *Player) Turn() *cube.LayerTurn {
	lt := p.t - p.unitOrigin
	for i := range p.segs {
		s := &p.segs[i]
		if lt < s.start || lt >= s.end {
			continue
		}
		progress := (lt - s.start) / (s.end - s.start)
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
	lt := p.t - p.unitOrigin
	for i := range p.segs {
		s := &p.segs[i]
		if lt >= s.start && lt < s.end {
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

// Spin is the whole-cube orientation for the current time. It reads spinT rather
// than the cycle-local clock precisely so the rotation survives the wrap at a
// cycle boundary.
func (p *Player) Spin() math3d.Mat4 { return p.cfg.Spin.Matrix(float32(p.spinT)) }

// Cycles is the number of scramble/solve cycles completed so far.
func (p *Player) Cycles() int { return p.cycles }

// Timeline returns the first cycle's phase boundaries on the absolute clock, for
// the debug overlay and the README; every later cycle is the same shape shifted by
// a whole number of cycle lengths.
func (p *Player) Timeline() (intro, scrambleEnd, pauseEnd, solveEnd float64) {
	i := p.cfg.Intro
	return i, i + p.scrambleEnd, i + p.pauseEnd, i + p.solveEnd
}
