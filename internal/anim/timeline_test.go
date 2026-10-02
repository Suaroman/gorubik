package anim

import (
	"math"
	"testing"

	"github.com/suaro/gorubik/internal/cube"
	"github.com/suaro/gorubik/internal/math3d"
)

// cycleLen returns the length of one scramble/pause/solve/hold cycle, computed
// the same way New schedules it: two sequences of n turns back to back with a
// pause between, then the solved hold.
func cycleLen(n int, cfg Config) float64 {
	span := float64(n)*cfg.Turn + float64(n-1)*cfg.Gap
	solveEnd := 2*span + cfg.Pause
	return solveEnd + cfg.Hold
}

func TestCycleLoopsForever(t *testing.T) {
	sc, err := cube.ParseSequence(cube.SpecScramble)
	if err != nil {
		t.Fatal(err)
	}
	sv, err := cube.ParseSequence(cube.SpecSolve)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	p := New(sc, sv, cfg)

	cycle := cycleLen(len(sc), cfg)
	const dt = 1.0 / 120
	// Run long enough for five full cycles after the intro.
	steps := int((5*cycle + cfg.Intro + 1) / dt)

	prevPhase := p.Phase()
	wantWraps := 0
	var prevSpin math3d.Mat4
	for i := 0; i < steps; i++ {
		prevSpin = p.Spin()
		before := p.Cycles()
		p.Update(dt)
		phase := p.Phase()

		// The cube must be solved at both ends of the solved hold and during it.
		if phase == PhaseOutro && !p.Cube().IsSolved() {
			t.Fatalf("step %d: cube not solved during the solved hold", i)
		}
		// Entering the hold means the inverse sequence just completed.
		if prevPhase == PhaseSolving && phase == PhaseOutro && !p.Cube().IsSolved() {
			t.Fatalf("step %d: solve ended on a scrambled cube", i)
		}

		if wraps := p.Cycles() - before; wraps > 0 {
			wantWraps += wraps
			if wraps != 1 {
				t.Fatalf("step %d: skipped %d cycles in one step", i, wraps)
			}
			// No snap: one 120 Hz frame of spin changes the orientation by a few
			// thousandths; a wrap that reset the spin clock would jump it by
			// YawRate*cycleLen radians worth of matrix elements.
			if d := matDelta(prevSpin, p.Spin()); d > 0.01 {
				t.Fatalf("step %d: spin snapped %.4f across the cycle boundary", i, d)
			}
			if !p.Cube().IsSolved() {
				t.Fatalf("step %d: cube not freshly solved at cycle boundary", i)
			}
		}
		prevPhase = phase
	}
	if wantWraps < 5 {
		t.Fatalf("expected at least 5 cycle repeats, got %d", wantWraps)
	}
}

// TestFirstCycleMatchesLegacySchedule pins the first cycle to the pre-loop
// timeline: intro, then the same absolute boundaries the old one-shot schedule
// printed in the start-up banner.
func TestFirstCycleMatchesLegacySchedule(t *testing.T) {
	sc, _ := cube.ParseSequence(cube.SpecScramble)
	sv, _ := cube.ParseSequence(cube.SpecSolve)
	cfg := DefaultConfig()
	p := New(sc, sv, cfg)

	intro, scr, pause, solve := p.Timeline()
	if intro != cfg.Intro {
		t.Errorf("intro = %v, want %v", intro, cfg.Intro)
	}
	span := float64(len(sc))*cfg.Turn + float64(len(sc)-1)*cfg.Gap
	if math.Abs(scr-(cfg.Intro+span)) > 1e-9 {
		t.Errorf("scrambleEnd = %v, want %v", scr, cfg.Intro+span)
	}
	if math.Abs(pause-(scr+cfg.Pause)) > 1e-9 {
		t.Errorf("pauseEnd = %v, want %v", pause, scr+cfg.Pause)
	}
	if math.Abs(solve-(pause+span)) > 1e-9 {
		t.Errorf("solveEnd = %v, want %v", solve, pause+span)
	}
}

// matDelta is the largest elementwise difference between two matrices.
func matDelta(a, b math3d.Mat4) float64 {
	m := 0.0
	for i := 0; i < 16; i++ {
		d := math.Abs(float64(a[i]) - float64(b[i]))
		if d > m {
			m = d
		}
	}
	return m
}
