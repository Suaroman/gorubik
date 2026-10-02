// Command gorubik is a native desktop Rubik's Cube animation: an OpenGL 4.3 core
// application written in Go, with no web layer anywhere in it.
//
// It plays the sequence the project specification asks for - a solved cube turning
// slowly in a studio, a ten-turn scramble, a pause, the exact inverse, and then
// forever - and it can also render chosen moments to PNG files so the picture can
// be inspected instead of assumed.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/suaro/gorubik/internal/app"
	"github.com/suaro/gorubik/internal/cube"
	"github.com/suaro/gorubik/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gorubik:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		width   = flag.Int("width", 1600, "window width in logical pixels")
		height  = flag.Int("height", 1000, "window height in logical pixels")
		title   = flag.String("title", "Go Rubik's Cube - Real-Time 3D", "window title")
		msaa    = flag.Int("msaa", 8, "multisampling samples (0 disables)")
		debug   = flag.Bool("debug", false, "start with the debug overlay on (H toggles it)")
		vsync   = flag.Bool("vsync", true, "swap buffers on the display's vertical blank")
		capt    = flag.String("capture", "", "render these timeline moments to PNG and exit, e.g. \"0.5,3.0,12.5\"")
		out     = flag.String("out", "captures", "directory for -capture PNGs")
		cw      = flag.Int("capture-width", 1600, "capture image width")
		ch      = flag.Int("capture-height", 1000, "capture image height")
		visible = flag.Bool("window", false, "keep the window visible while capturing")
		verify  = flag.Bool("verify", false, "check the cube simulation without opening a window")
		quiet   = flag.Bool("quiet", false, "print less at start-up")
		bench   = flag.Float64("bench", 0, "run the window for N seconds, print the measured fps and exit")
		passtim = flag.Bool("pass-times", false, "print the mean cost of every render pass")

		// Look overrides. Every one of these is also a field in
		// render.DefaultSettings; they exist so a tuning sweep does not need a
		// rebuild.
		fill       = flag.Float64("fill", 0, "fraction of the frame height the cube fills (0 = setting default)")
		fov        = flag.Float64("fov", 0, "vertical field of view in degrees (0 = setting default)")
		exposure   = flag.Float64("exposure", 0, "exposure multiplier (0 = setting default)")
		satur      = flag.Float64("saturation", 0, "saturation after the tone curve (0 = setting default)")
		bloom      = flag.Float64("bloom", -1, "bloom strength (negative = setting default)")
		keyMul     = flag.Float64("key", 1, "key light intensity multiplier")
		fillMul    = flag.Float64("fill-light", 1, "fill light intensity multiplier")
		rimMul     = flag.Float64("rim", 1, "rim light intensity multiplier")
		envMul     = flag.Float64("env", 1, "reflection environment multiplier")
		aoOn       = flag.Bool("ao", true, "screen-space ambient occlusion in the cubie gaps")
		aces       = flag.Bool("aces", false, "use the ACES tone curve instead of extended Reinhard")
		shadowSz   = flag.Int("shadow-size", 0, "shadow map edge length (0 = setting default)")
		shade      = flag.Int("shade", 0, "diagnostic buffer: 1=ao 2=normals 3=shadow 4=roughness 5=flat red")
		nocull     = flag.Bool("no-cull", false, "disable back-face culling on the cube (diagnostic)")
		noShadow   = flag.Bool("no-shadow", false, "bind a fully lit 1x1 depth map (diagnostic)")
		shadowBias = flag.Float64("shadow-bias", 0, "override the shadow normal bias, in texels")
		single     = flag.Bool("single-draw", false, "draw one non-instanced cubie (diagnostic)")
		noDepth    = flag.Bool("no-depth", false, "draw the cube with the depth test off (diagnostic)")
		turn       = flag.Float64("turn", 0, "seconds per face turn (0 = timeline default)")
		intro      = flag.Float64("intro", 0, "seconds of solved cube before the first turn (0 = default)")
		pause      = flag.Float64("pause", 0, "seconds holding the scrambled cube (0 = default)")
	)
	flag.Parse()

	if *verify {
		return verifyState()
	}

	cfg := render.DefaultSettings()
	cfg.MSAA = int32(*msaa)
	cfg.AOEnabled = *aoOn
	cfg.PassTimes = *passtim
	cfg.ACES = *aces
	cfg.DebugShade = *shade
	cfg.NoCull = *nocull
	cfg.NoShadow = *noShadow
	if *shadowBias != 0 {
		cfg.ShadowNormalBias = float32(*shadowBias)
	}
	cfg.SingleDraw = *single
	cfg.NoDepth = *noDepth
	cfg.EnvIntensity *= float32(*envMul)
	cfg.KeyColor = scale(cfg.KeyColor, *keyMul)
	cfg.FillColor = scale(cfg.FillColor, *fillMul)
	cfg.RimColor = scale(cfg.RimColor, *rimMul)
	if *fill > 0 {
		cfg.Fill = float32(*fill)
	}
	if *fov > 0 {
		cfg.FOVDeg = float32(*fov)
	}
	if *exposure > 0 {
		cfg.Exposure = float32(*exposure)
	}
	if *satur > 0 {
		cfg.Saturation = float32(*satur)
	}
	if *bloom >= 0 {
		cfg.BloomStrength = float32(*bloom)
	}
	if *shadowSz > 0 {
		cfg.ShadowSize = int32(*shadowSz)
	}

	times, err := parseTimes(*capt)
	if err != nil {
		return err
	}

	return app.Run(app.Options{
		Width:  *width,
		Height: *height,
		Title:  *title,
		Debug:  *debug,
		// A capture run does not need the window on screen, and hiding it keeps a
		// stray frame from landing in a screenshot of something else.
		Visible:      *visible || len(times) == 0,
		VSync:        *vsync,
		Quiet:        *quiet,
		Bench:        *bench,
		CaptureTimes: times,
		CaptureDir:   *out,
		CaptureW:     int32(*cw),
		CaptureH:     int32(*ch),
		Settings:     &cfg,
		Intro:        *intro,
		Turn:         *turn,
		Pause:        *pause,
	})
}

func scale(c [3]float32, k float64) [3]float32 {
	return [3]float32{c[0] * float32(k), c[1] * float32(k), c[2] * float32(k)}
}

func parseTimes(s string) ([]float64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []float64
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("bad capture time %q: %w", p, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// verifyState exercises the simulator with no GL context at all: the specified
// scramble, then the specified solve, then a bit-exact check that every cubie is
// home and unrotated. It is the fastest possible answer to "is the logic right".
func verifyState() error {
	sc, err := cube.ParseSequence(cube.SpecScramble)
	if err != nil {
		return err
	}
	sv, err := cube.ParseSequence(cube.SpecSolve)
	if err != nil {
		return err
	}
	c := cube.New()
	fmt.Printf("cubies: %d\n", len(c.Cubies))
	if ok, msg := c.SolvedReport(); !ok {
		return fmt.Errorf("a freshly built cube is not solved: %s", msg)
	}
	fmt.Printf("scramble: %s\nsolve:    %s\n", cube.FormatSequence(sc), cube.FormatSequence(sv))

	for _, m := range sc {
		c.Apply(m)
	}
	if c.IsSolved() {
		return fmt.Errorf("the scramble left the cube solved, so it is not really a scramble")
	}
	fmt.Printf("after scramble: solved=%v moves=%d\n", c.IsSolved(), c.MoveCount())
	for _, m := range sv {
		c.Apply(m)
	}
	ok, msg := c.SolvedReport()
	fmt.Printf("after solve:    solved=%v moves=%d\n  %s\n", ok, c.MoveCount(), msg)
	fmt.Print(c.FaceString())
	if !ok {
		return fmt.Errorf("cube did not finish solved")
	}
	fmt.Println("PASS: 20 turns executed, state finished bit-exactly solved")
	return nil
}
