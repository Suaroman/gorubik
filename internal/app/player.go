package app

import (
	"github.com/Suaroman/gorubik/internal/anim"
	"github.com/Suaroman/gorubik/internal/cube"
	"github.com/Suaroman/gorubik/internal/render"
)

// player adapts the animation timeline to what the renderer wants: a render.View
// describing one frame. Keeping the adapter here means the render package never
// has to know that a timeline exists, and the animation package never has to know
// that a window exists.
type player struct {
	p   *anim.Player
	cfg anim.Config
}

func newPlayer(o Options) *player {
	cfg := anim.DefaultConfig()
	if o.Intro > 0 {
		cfg.Intro = o.Intro
	}
	if o.Turn > 0 {
		cfg.Turn = o.Turn
	}
	if o.Pause > 0 {
		cfg.Pause = o.Pause
	}
	sc, err := cube.ParseSequence(cube.SpecScramble)
	if err != nil {
		panic("spec scramble does not parse: " + err.Error())
	}
	sv, err := cube.ParseSequence(cube.SpecSolve)
	if err != nil {
		panic("spec solve does not parse: " + err.Error())
	}
	return &player{p: anim.New(sc, sv, cfg), cfg: cfg}
}

func (pl *player) Update(dt float64) { pl.p.Update(dt) }
func (pl *player) Restart()          { pl.p.Restart() }
func (pl *player) Cube() *cube.Cube  { return pl.p.Cube() }

// View snapshots the current moment for the renderer.
func (pl *player) View(fps float32, debug bool) render.View {
	return render.View{
		Cube:  pl.p.Cube(),
		Spin:  pl.p.Spin(),
		Turn:  pl.p.Turn(),
		Time:  pl.p.Time(),
		Phase: pl.p.Phase().String(),
		Move:  pl.p.MoveName(),
		FPS:   fps,
		Debug: debug,
	}
}

// timeline is the phase boundary table, for the start-up banner.
func (pl *player) timeline() (intro, scrambleEnd, pauseEnd, solveEnd float64) {
	return pl.p.Timeline()
}
