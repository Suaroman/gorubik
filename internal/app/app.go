// Package app owns the native window and the frame loop. It deliberately knows
// nothing about cubes or shading: it creates a context, feeds the renderer a
// description of what should be on screen, and swaps buffers.
package app

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/go-gl/glfw/v3.3/glfw"

	"github.com/Suaroman/gorubik/internal/render"
)

// Options is everything the caller can say about the window and the run.
type Options struct {
	Width, Height int
	Title         string
	Debug         bool
	Visible       bool
	VSync         bool
	Quiet         bool
	// Bench runs the windowed loop for this many seconds and then prints the
	// measured frame rate and closes; zero runs until the window is closed.
	Bench float64

	// Settings is the visual configuration; nil selects render.DefaultSettings.
	Settings *render.Settings

	// Timeline overrides; zero keeps the animation package's defaults.
	Intro, Turn, Pause float64

	// CaptureTimes, when non-empty, renders those timeline moments offscreen to PNG
	// and exits. It is what makes visual inspection repeatable: the same times
	// produce the same pixels, and nothing depends on screen capture timing.
	CaptureTimes []float64
	CaptureDir   string
	CaptureW     int32
	CaptureH     int32
}

// settings returns the visual configuration, defaulting when the caller had no
// opinion.
func (o Options) settings() render.Settings {
	if o.Settings != nil {
		return *o.Settings
	}
	return render.DefaultSettings()
}

// Run opens the window and drives the animation until the window is closed.
func Run(o Options) error {
	// GLFW (and OpenGL on Windows) is bound to one OS thread for the life of the
	// context, so the goroutine that creates the window must be the one that draws.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := glfw.Init(); err != nil {
		return err
	}
	defer glfw.Terminate()

	vs := glfw.GetVersionString()
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Samples, int(o.settings().MSAA))
	glfw.WindowHint(glfw.SRGBCapable, glfw.True)
	glfw.WindowHint(glfw.Visible, boolHint(o.Visible))

	win, err := glfw.CreateWindow(o.Width, o.Height, o.Title, nil, nil)
	if err != nil {
		return fmt.Errorf("glfw: create window: %w", err)
	}
	win.MakeContextCurrent()

	glVersion, glRenderer, err := render.InitGL()
	if err != nil {
		return fmt.Errorf("initialise gl: %w", err)
	}
	if !o.Quiet {
		fmt.Printf("opengl: %s\nrenderer: %s\n", glVersion, glRenderer)
		fmt.Printf("glfw: %s\n", vs)
	}

	if o.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}

	r, err := render.NewWithSettings(o.settings())
	if err != nil {
		return err
	}
	defer r.Close()

	fw, fh := win.GetFramebufferSize()
	if err := r.Resize(int32(fw), int32(fh)); err != nil {
		return err
	}
	win.SetFramebufferSizeCallback(func(w *glfw.Window, width, height int) {
		if width > 0 && height > 0 {
			_ = r.Resize(int32(width), int32(height))
		}
	})

	closed := false
	win.SetCloseCallback(func(w *glfw.Window) { closed = true })
	p := newPlayer(o)
	win.SetKeyCallback(func(w *glfw.Window, key glfw.Key, sc int, action glfw.Action, mods glfw.ModifierKey) {
		if action != glfw.Press {
			return
		}
		switch key {
		case glfw.KeyEscape, glfw.KeyQ:
			closed = true
		case glfw.KeyH:
			r.ToggleDebug()
		case glfw.KeyR:
			p.Restart()
		}
	})

	if len(o.CaptureTimes) > 0 {
		return capture(win, r, p, o)
	}

	if !o.Quiet {
		intro, scr, pause, solve := p.timeline()
		fmt.Printf("timeline: solved %.1fs | scramble to %.1fs | hold to %.1fs | solve to %.1fs\n",
			intro, scr, pause, solve)
		csx, _ := win.GetContentScale()
		// The monitor's own mode is the tell for DPI handling: a process Windows is
		// virtualising reports a shrunken desktop here, and the framebuffer would be
		// stretched rather than rendered.
		if m := glfw.GetPrimaryMonitor(); m != nil {
			vm := m.GetVideoMode()
			fmt.Printf("monitor: %dx%d @ %dHz, content scale %.2fx\n", vm.Width, vm.Height, vm.RefreshRate, csx)
		}
		fmt.Printf("window: %dx%d framebuffer %dx%d content scale %.2f (samples %d) - esc or q quits, h toggles the overlay, r restarts, space freezes\n",
			o.Width, o.Height, fw, fh, csx, r.Samples())
	}

	prev := glfw.GetTime()
	fps := 0.0
	benchStart, benchFrames := glfw.GetTime(), 0
	for !win.ShouldClose() && !closed {
		now := glfw.GetTime()
		dt := float64(now - prev)
		prev = now
		benchFrames++
		if dt > 0 {
			fps += (1/float64(dt) - fps) * 0.05
		}
		if win.GetKey(glfw.KeySpace) == glfw.Press {
			dt = 0 // hold: freeze the simulation while still rendering
		}

		p.Update(dt)
		fw, fh := win.GetFramebufferSize()
		v := p.View(float32(fps), r.Debug())
		v.Width, v.Height = int32(fw), int32(fh)
		if err := r.Draw(v); err != nil {
			return err
		}
		win.SwapBuffers()
		glfw.PollEvents()

		// A timed run turns the window into an instrument: the number it prints is
		// frames divided by wall-clock seconds, not a smoothed estimate.
		if o.Bench > 0 && glfw.GetTime()-benchStart >= o.Bench {
			el := glfw.GetTime() - benchStart
			fmt.Printf("bench: %d frames in %.2fs = %.1f fps at %dx%d (vsync=%v, samples=%d)\n",
				benchFrames, el, float64(benchFrames)/el, fw, fh, o.VSync, r.Samples())
			closed = true
		}
	}
	return nil
}

// capture renders the requested timeline moments to PNG files. Everything is
// simulated from t=0 with a fixed step, so a captured frame does not depend on how
// fast the machine happened to be.
func capture(win *glfw.Window, r *render.Renderer, p *player, o Options) error {
	times := append([]float64(nil), o.CaptureTimes...)
	sort.Float64s(times)
	w, h := o.CaptureW, o.CaptureH
	if w == 0 {
		w, h = 1600, 1000
	}
	if err := r.BeginCapture(w, h); err != nil {
		return err
	}
	const step = 1.0 / 120
	for _, t := range times {
		p.Restart()
		for elapsed := 0.0; elapsed+step/2 < t; elapsed += step {
			p.Update(step)
		}
		v := p.View(0, false)
		v.Width, v.Height = w, h
		if err := r.Draw(v); err != nil {
			return err
		}
		if !o.Quiet {
			eye, fov, dist := r.CameraInfo()
			fmt.Printf("  camera eye=(%.2f,%.2f,%.2f) dist=%.2f fov=%.1f\n", eye[0], eye[1], eye[2], dist, fov)
			for _, p := range [][3]float32{{0, 0, 0}, {1.5, 1.5, 1.5}, {-1.5, -1.5, -1.5}, {0, 1.5, 0}} {
				px, py, dz, ok := r.DebugProject(p[0], p[1], p[2])
				fmt.Printf("  world(%5.2f,%5.2f,%5.2f) -> px(%7.1f,%7.1f) z=%+.3f inside=%v\n", p[0], p[1], p[2], px, py, dz, ok)
			}
			for _, p := range [][3]float32{{0, 0, 0}, {0, -1.5, 0}, {1.5, 1.5, 1.5}, {0, -1.95, 0}, {0, -1.95, 2.5}} {
				u, v, zc, out, far := r.DebugProjectLight(p[0], p[1], p[2])
				fmt.Printf("  light(%5.2f,%5.2f,%5.2f) -> uv(%6.3f,%6.3f) z=%+.3f outside=%v far=%v\n",
					p[0], p[1], p[2], u, v, zc, out, far)
			}
			for _, l := range r.AuditBuffers() {
				fmt.Println(" ", l)
			}
			f := r.LastFrame()
			fmt.Printf("  pose: %d bodies, %d stickers\n", len(f.Bodies), len(f.Stickers))
			for _, i := range []int{0, 13, 25} {
				if i < len(f.Bodies) {
					m := f.Bodies[i].Matrix
					fmt.Printf("  body[%d] id=%d t=(%+.3f,%+.3f,%+.3f) m0=(%+.3f,%+.3f,%+.3f) m1=(%+.3f,%+.3f,%+.3f)\n",
						i, f.Bodies[i].ID, m[12], m[13], m[14], m[0], m[1], m[2], m[4], m[5], m[6])
				}
			}
			sh, err := r.ProbeShadow([][2]float32{{0.5, 0.5}, {0.2, 0.2}, {0.8, 0.8}, {0.02, 0.5}})
			if err != nil {
				return err
			}
			fmt.Printf("  shadow map: centre=%.4f quarter=%.4f far=%.4f corner=%.4f\n", sh[0], sh[1], sh[2], sh[3])
			hdr, err := r.ProbeHDR([][2]float32{
				{0.50, 0.50}, // red face, centre
				{0.50, 0.22}, // white face, top
				{0.30, 0.45}, // green face, left
				{0.50, 0.90}, // floor, near
				{0.08, 0.12}, // backdrop, corner
				{0.50, 0.05}, // backdrop, top edge
			})
			if err != nil {
				return err
			}
			for i, v := range hdr {
				fmt.Printf("  hdr[%d] = %.4f %.4f %.4f\n", i, v[0], v[1], v[2])
			}
		}
		path, err := r.WriteCapturePNG(o.CaptureDir, t)
		if err != nil {
			return err
		}
		vv := p.View(0, false)
		turning := 0
		if vv.Turn != nil {
			turning = len(p.Cube().Layer(vv.Turn.Move.Axis(), vv.Turn.Move.Layer()))
		}
		fmt.Printf("captured t=%.2fs phase=%s move=%-3s turning=%d moves=%d solved=%v -> %s\n",
			t, vv.Phase, vv.Move, turning, p.Cube().MoveCount(), p.Cube().IsSolved(), path)
	}
	return nil
}

func boolHint(b bool) int {
	if b {
		return glfw.True
	}
	return glfw.False
}
