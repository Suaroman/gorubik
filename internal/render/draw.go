package render

import (
	"math"
	"strconv"

	"github.com/go-gl/gl/v4.3-core/gl"

	"github.com/suaro/gorubik/internal/cube"
	"github.com/suaro/gorubik/internal/gfx"
	"github.com/suaro/gorubik/internal/math3d"
)

// View is one frame's worth of "what should be on screen". It carries the logical
// cube and the animation's current pose inputs, not transforms: the renderer owns
// geometry, so the sticker lift and the cubie size live in exactly one place.
type View struct {
	Width, Height int32

	Cube *cube.Cube
	Spin math3d.Mat4     // whole-cube orientation
	Turn *cube.LayerTurn // in-flight face turn, or nil
	Time float64         // timeline position, seconds

	Phase string
	Move  string
	FPS   float32
	Debug bool
}

// plasticLinear is the dark semi-gloss body colour the brief specifies, in linear
// light. sRGB(24, 24, 30).
var plasticLinear = math3d.SRGBToLinear3(math3d.Vec3{24.0 / 255, 24.0 / 255, 30.0 / 255})

// stickerLinear is the sticker palette in linear light, indexed by cube.Color.
// The values are the specified sRGB triples converted, never "eyeballed darker":
// the tone curve is what brings them back down, and pre-darkening here would
// double-apply it.
var stickerLinear = func() [7]math3d.Vec3 {
	var t [7]math3d.Vec3
	t[cube.ColorNone] = math3d.SRGBToLinear3(math3d.Vec3{0.004, 0.004, 0.005})
	t[cube.ColorWhite] = math3d.SRGBToLinear3(math3d.Vec3{245.0 / 255, 245.0 / 255, 245.0 / 255})
	t[cube.ColorYellow] = math3d.SRGBToLinear3(math3d.Vec3{1.0, 204.0 / 255, 0})
	t[cube.ColorRed] = math3d.SRGBToLinear3(math3d.Vec3{204.0 / 255, 24.0 / 255, 24.0 / 255})
	t[cube.ColorOrange] = math3d.SRGBToLinear3(math3d.Vec3{1.0, 138.0 / 255, 0})
	t[cube.ColorGreen] = math3d.SRGBToLinear3(math3d.Vec3{0, 150.0 / 255, 70.0 / 255})
	t[cube.ColorBlue] = math3d.SRGBToLinear3(math3d.Vec3{0, 74.0 / 255, 204.0 / 255})
	return t
}()

// Draw renders one frame into the window (or into the capture target).
func (r *Renderer) Draw(v View) error {
	if v.Width < 1 || v.Height < 1 || v.Cube == nil {
		return nil
	}
	if err := r.Resize(v.Width, v.Height); err != nil {
		return err
	}
	frame := v.Cube.Pose(v.Spin, v.Turn, r.localStickers)
	r.lastFrame = frame

	r.pt.begin("cpu")
	r.setupCamera(v)
	r.uploadInstances(frame, v)
	r.pt.end()

	r.pt.begin("shadow")
	r.shadowPass()
	r.pt.end()
	r.pt.begin("prepass")
	r.prepass()
	r.pt.end()
	r.pt.begin("ao")
	aoUnit := r.aoPass()
	r.pt.end()
	r.pt.begin("main")
	err := r.mainPass(aoUnit)
	r.pt.end()
	if err != nil {
		return err
	}
	r.pt.begin("bloom")
	r.bloomPass()
	r.pt.end()
	r.pt.begin("composite")
	r.compositePass(v)
	r.pt.end()
	if v.Debug {
		r.pt.begin("hud")
		r.hudPass(v)
		r.pt.end()
	}
	r.pt.report(30)
	gfx.Unbind()
	return gfx.PollErrors("frame")
}

// ---------------------------------------------------------------------------
// Camera
// ---------------------------------------------------------------------------

// silhouetteRadius is the projected half-extent of the cube in grid units. It is
// deliberately smaller than the circumscribed sphere (2.60): a cube seen face-on
// spans 1.5 and corner-on 2.6, and framing on the average is what keeps the shot
// steady as it turns.
const silhouetteRadius = 2.18

// camDir is the direction from the origin to the camera: slightly right and above,
// so the front, top and right faces are all in shot at the start.
var camDir = [3]float32{0.44, 0.40, 1.0}

func (r *Renderer) setupCamera(v View) {
	c := &r.cfg
	aspect := float32(r.w) / float32(r.h)
	r.proj = math3d.Perspective(c.FOVDeg*math.Pi/180, aspect, c.Near, c.Far)

	// A slow, small orbit so the shot breathes. It is a fraction of a degree and
	// much slower than the cube's own rotation, so it reads as a live camera rather
	// than as a second spin.
	t := float64(v.Time) * 2 * math.Pi
	bx := c.BreathAmp * float32(math.Sin(float64(c.BreathRate)*t))
	by := 0.6 * c.BreathAmp * float32(math.Cos(float64(c.BreathRate)*0.8*t))
	d := norm3f(camDir[0]+bx, camDir[1]+by, camDir[2])

	fovRad := float64(c.FOVDeg) * math.Pi / 180
	dist := silhouetteRadius / (float64(c.Fill) * math.Tan(fovRad/2))
	r.camPos = math3d.Vec3{d[0] * float32(dist), d[1] * float32(dist), d[2] * float32(dist)}
	r.view = math3d.LookAt(r.camPos, math3d.Vec3{}, math3d.Vec3{0, 1, 0})
	r.viewProj = r.proj.Mul(r.view)
	inv, ok := r.viewProj.Invert()
	if !ok {
		inv = math3d.Identity()
	}
	r.invViewProj = inv

	// The key light's shadow camera: an orthographic box that contains the cube
	// with room for its cast shadow to fall on the floor.
	ld := math3d.Vec3(c.KeyDir)
	eye := math3d.Vec3{ld[0] * 9, ld[1] * 9, ld[2] * 9}
	target := math3d.Vec3{0, -0.45, 0}
	lv := math3d.LookAt(eye, target, math3d.Vec3{0, 1, 0})
	hr := float64(c.ShadowRadius)
	lp := math3d.Ortho(-float32(hr), float32(hr), -float32(hr), float32(hr), 1, 18)
	r.lightViewProj = lp.Mul(lv)
	r.shadowTexelWorld = float32(2 * hr / float64(c.ShadowSize))
}

func norm3f(x, y, z float32) [3]float32 {
	l := math.Sqrt(float64(x*x + y*y + z*z))
	if l < 1e-6 {
		return [3]float32{0, 0, 1}
	}
	return [3]float32{float32(x / float32(l)), float32(y / float32(l)), float32(z / float32(l))}
}

// ---------------------------------------------------------------------------
// Instance data
// ---------------------------------------------------------------------------

// uploadInstances writes the 26 cubie bodies and the 54 stickers. Every sticker
// transform comes from its own cubie's matrix, which is why a scramble cannot
// repaint a face: there is no code path that assigns a colour by position.
func (r *Renderer) uploadInstances(f cube.Frame, v View) {
	c := &r.cfg

	// Cubies currently in the turning layer, for the overlay's highlight.
	var turning [64]bool
	if v.Debug && v.Turn != nil {
		for _, cb := range v.Cube.Layer(v.Turn.Move.Axis(), v.Turn.Move.Layer()) {
			if cb.ID >= 0 && cb.ID < len(turning) {
				turning[cb.ID] = true
			}
		}
	}

	bodies := r.instCPU[:len(f.Bodies)*24]
	for i, b := range f.Bodies {
		o := i * 24
		copy(bodies[o:o+16], b.Matrix[:])
		bodies[o+16] = plasticLinear[0]
		bodies[o+17] = plasticLinear[1]
		bodies[o+18] = plasticLinear[2]
		bodies[o+19] = c.PlasticRoughness
		bodies[o+20] = c.PlasticF0
		bodies[o+21] = 0 // plastic carries no clear coat
		hl := float32(0)
		if b.ID < len(turning) && turning[b.ID] {
			hl = 1
		}
		bodies[o+22] = hl
		bodies[o+23] = 0
	}
	r.bodyInst.SubData(0, bodies)

	sticks := r.instCPU[len(f.Bodies)*24:][:len(f.Stickers)*24]
	for i, s := range f.Stickers {
		o := i * 24
		copy(sticks[o:o+16], s.Matrix[:])
		col := stickerLinear[s.Color]
		sticks[o+16] = col[0]
		sticks[o+17] = col[1]
		sticks[o+18] = col[2]
		sticks[o+19] = c.StickerRoughness
		sticks[o+20] = c.StickerF0
		sticks[o+21] = c.StickerCoat
		hl := float32(0)
		if s.Cubie < len(turning) && turning[s.Cubie] {
			hl = 1
		}
		sticks[o+22] = hl
		sticks[o+23] = 0
	}
	r.stickInst.SubData(0, sticks)
}

// drawCubeGeometry issues the two instanced draws with whatever program is bound.
func (r *Renderer) drawCubeGeometry() {
	r.bodyVAO.Bind()
	if r.cfg.SingleDraw {
		gfx.DrawIndexed(r.bodyTris)
		return
	}
	gfx.DrawIndexedInstanced(r.bodyTris, 26)
	r.stickVAO.Bind()
	gfx.DrawIndexedInstanced(r.stickTris, 54)
}

// ---------------------------------------------------------------------------
// Passes
// ---------------------------------------------------------------------------

func (r *Renderer) shadowPass() {
	c := &r.cfg
	r.shadowFB.Bind()
	gfx.Viewport(c.ShadowSize, c.ShadowSize)
	// glClear honours the depth write mask, and the AO pass leaves it off, so it
	// has to be re-enabled before the clear or the map keeps last frame's depth.
	gfx.DepthMask(true)
	gfx.ClearDepthBuffer()
	gfx.Enable(gl.DEPTH_TEST)
	gfx.DepthFunc(gl.LESS)
	gfx.Enable(gl.CULL_FACE)
	gfx.CullFace(gl.FRONT) // front-face culling removes most acne without a bias fight
	r.p.depth.Use()
	r.p.depth.SetMat4("uLightViewProj", r.lightViewProj[:])
	r.drawCubeGeometry()
	gfx.CullFace(gl.BACK)
}

// prepass writes view-space-ish normals and depth for the ambient occlusion pass.
// It renders once, at AO resolution, straight into the textures that pass samples
// from: giving it the frame's multisampled target and resolving it afterwards cost
// more than the effect it feeds.
func (r *Renderer) prepass() {
	r.preFB.Bind()
	gfx.Viewport(r.aw, r.ah)
	gfx.DepthMask(true)
	gfx.ClearColor(0, 0, 0, 1)
	gfx.ClearColorBuffer()
	gfx.ClearDepthBuffer()
	gfx.Enable(gl.DEPTH_TEST)
	gfx.DepthFunc(gl.LESS)
	gfx.Enable(gl.CULL_FACE)
	gfx.CullFace(gl.BACK)
	r.p.prepass.Use()
	r.p.prepass.SetMat4("uViewProj", r.viewProj[:])
	r.p.prepass.SetMat4("uView", r.view[:])
	r.drawCubeGeometry()
}

// aoPass computes and blurs screen-space occlusion, returning the texture unit it
// ended up on, or -1 when the effect is switched off.
func (r *Renderer) aoPass() int32 {
	if !r.cfg.AOEnabled || len(r.aoKernel) == 0 {
		return -1
	}
	c := &r.cfg
	gfx.Disable(gl.DEPTH_TEST)
	gfx.DepthMask(false)
	gfx.Disable(gl.CULL_FACE)

	r.preFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.aoTex.ID(), 0)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, 0, 0)
	gfx.Viewport(r.aw, r.ah)
	r.p.ssao.Use()
	r.depthTex.Bind(0)
	r.normalTex.Bind(1)
	r.p.ssao.SetInt("uDepth", 0)
	r.p.ssao.SetInt("uNormal", 1)
	r.p.ssao.SetMat4("uProj", r.proj[:])
	r.p.ssao.SetMat4("uInvProj", r.invViewProj[:])
	r.p.ssao.SetVec2("uResolution", float32(r.aw), float32(r.ah))
	r.p.ssao.SetFloat("uRadius", c.AORadius)
	r.p.ssao.SetFloat("uBias", c.AOBias)
	r.p.ssao.SetFloat("uPower", c.AOPower)
	r.p.ssao.SetFloat("uIntensity", c.AOIntensity)
	n := int32(len(r.aoKernel))
	if n > 24 {
		n = 24
	}
	r.p.ssao.SetInt("uKernelSize", n)
	r.p.ssao.SetVec3Array("uKernel", r.aoFlat[:n*3])
	r.blitTriangle()

	// Bilateral blur.
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.aoBlurTex.ID(), 0)
	r.p.aoBlur.Use()
	r.aoTex.Bind(0)
	r.depthTex.Bind(1)
	r.p.aoBlur.SetInt("uAO", 0)
	r.p.aoBlur.SetInt("uDepth", 1)
	r.p.aoBlur.SetVec2("uTexel", 1/float32(r.aw), 1/float32(r.ah))
	r.p.aoBlur.SetFloat("uRangeCheck", 400)
	r.blitTriangle()
	return 0
}

// LastFrame returns the pose the renderer last uploaded, so a caller can check the
// instance data without a GL round trip.
func (r *Renderer) LastFrame() cube.Frame { return r.lastFrame }

func (r *Renderer) mainPass(aoUnit int32) error {
	c := &r.cfg
	r.mainFB.Bind()
	gl.InvalidateFramebuffer(gl.FRAMEBUFFER, 1, &[]uint32{gl.COLOR_ATTACHMENT0}[0])
	gfx.Viewport(r.w, r.h)
	// Same trap as the shadow pass: the AO pass runs immediately before this and
	// leaves the depth mask off, which silently turns the clear into a no-op and
	// makes the cube fail the depth test against its own prepass depth.
	gfx.DepthMask(true)
	gfx.ClearColorBuffer()
	gfx.ClearDepthBuffer()
	r.pt.mark("clear")

	// The floor is drawn before the backdrop and writes depth, so the backdrop can
	// be rejected against it at the end. Shading every pixel twice - once as
	// backdrop, once as floor - measured 4.5 ms of a 25 ms frame in pure overdraw.
	gfx.Enable(gl.DEPTH_TEST)
	gfx.DepthFunc(gl.LESS)
	gfx.DepthMask(true)
	gfx.Disable(gl.CULL_FACE)
	r.p.floor.Use()
	r.p.floor.SetMat4("uViewProj", r.viewProj[:])
	r.p.floor.SetFloat("uRadius", c.FloorRadius)
	r.p.floor.SetFloat("uY", c.FloorY)
	r.p.floor.SetVec3("uCamPos", r.camPos)
	r.p.floor.SetVec2("uResolution", float32(r.w), float32(r.h))
	r.p.floor.SetVec3("uKeyColor", c.KeyColor)
	r.p.floor.SetVec3("uFillColor", c.FillColor)
	r.p.floor.SetVec3("uFloorAlbedo", grey(c.FloorAlbedo))
	r.p.floor.SetFloat("uFloorRough", c.FloorRough)
	r.p.floor.SetFloat("uFadeStart", c.FloorFadeIn)
	r.p.floor.SetFloat("uFadeEnd", c.FloorFadeOut)
	r.p.floor.SetFloat("uEnvIntensity", c.FloorEnvIntensity)
	r.p.floor.SetFloat("uPrefilterMips", float32(r.envMips))
	r.p.floor.SetInt("uDebugShade", int32(c.DebugShade))
	setShadowUniforms(r.p.floor, r)
	r.envSpec.Bind(2)
	r.p.floor.SetInt("uPrefilter", 2)
	gfx.DrawTriangleArrays(0, 6)
	r.pt.mark("floor")

	// Cube.
	gfx.DepthMask(true)
	if c.NoDepth {
		gfx.Disable(gl.DEPTH_TEST)
	} else {
		gfx.Enable(gl.DEPTH_TEST)
	}
	if c.NoCull {
		gfx.Disable(gl.CULL_FACE)
	} else {
		gfx.Enable(gl.CULL_FACE)
		gfx.CullFace(gl.BACK)
	}
	r.p.cube.Use()
	r.p.cube.SetMat4("uViewProj", r.viewProj[:])
	r.p.cube.SetVec3("uCamPos", r.camPos)
	r.p.cube.SetVec3("uKeyColor", c.KeyColor)
	r.p.cube.SetVec3("uFillColor", c.FillColor)
	r.p.cube.SetVec3("uRimColor", c.RimColor)
	r.p.cube.SetFloat("uEnvIntensity", c.EnvIntensity)
	r.p.cube.SetFloat("uPrefilterMips", float32(r.envMips))
	r.p.cube.SetVec2("uResolution", float32(r.w), float32(r.h))
	r.p.cube.SetFloat("uAOStrength", 1)
	if aoUnit < 0 {
		r.p.cube.SetFloat("uAOStrength", 0)
		r.aoBlurTex.Bind(0) // a 1.0-filled stand-in would be cleaner, this is cheaper
	} else {
		r.aoBlurTex.Bind(0)
	}
	r.p.cube.SetInt("uAO", 0)
	setShadowUniforms(r.p.cube, r)
	r.envIrrad.Bind(1)
	r.p.cube.SetInt("uIrradiance", 1)
	r.envSpec.Bind(2)
	r.p.cube.SetInt("uPrefilter", 2)
	r.p.cube.SetInt("uDebugShade", int32(c.DebugShade))
	r.drawCubeGeometry()
	r.pt.mark("cube")

	// Backdrop last. sky.vert writes a window depth of exactly 1.0 and the test is
	// GL_EQUAL, so this fills only the pixels still holding the cleared depth rather
	// than the whole frame.
	gfx.Enable(gl.DEPTH_TEST)
	gfx.DepthFunc(gl.EQUAL)
	gfx.DepthMask(false)
	gfx.Disable(gl.CULL_FACE)
	r.p.sky.Use()
	r.p.sky.SetMat4("uInvViewProj", r.invViewProj[:])
	r.p.sky.SetVec3("uCamPos", r.camPos)
	r.blitTriangle()
	r.pt.mark("sky")

	// Resolve HDR.
	r.postFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.hdrTex.ID(), 0)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, 0, 0)
	blit(r.mainFB, r.postFB, r.w, r.h, gl.COLOR_BUFFER_BIT)
	r.pt.mark("resolve")
	return gfx.PollErrors("main")
}

func setShadowUniforms(p *gfx.Program, r *Renderer) {
	p.SetMat4("uShadowViewProj", r.lightViewProj[:])
	p.SetFloat("uShadowTexel", float32(1.0/float64(r.cfg.ShadowSize)))
	p.SetFloat("uTexelWorld", float32(r.shadowTexelWorld))
	p.SetFloat("uNormalBias", r.cfg.ShadowNormalBias)
	p.SetFloat("uSoftness", r.cfg.ShadowSoftness)
	p.SetVec3("uKeyLightDir", r.cfg.KeyDir)
	p.SetVec3("uFillLightDir", r.cfg.FillDir)
	p.SetVec3("uRimLightDir", r.cfg.RimDir)
	if r.cfg.NoShadow {
		r.litTex.Bind(3)
	} else {
		r.shadowTex.Bind(3)
	}
	p.SetInt("uShadow", 3)
}

func (r *Renderer) bloomPass() {
	c := &r.cfg
	if c.BloomStrength <= 0 || len(r.bloomTex) == 0 {
		return
	}
	gfx.Disable(gl.DEPTH_TEST)
	gfx.DepthMask(false)
	gfx.Disable(gl.CULL_FACE)
	r.postFB.Bind()

	// Bright pass into the first tier.
	t0 := r.bloomTex[0]
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, t0.ID(), 0)
	gfx.Viewport(t0.Width(), t0.Height())
	r.p.bright.Use()
	r.hdrTex.Bind(0)
	r.p.bright.SetInt("uSrc", 0)
	r.p.bright.SetFloat("uThreshold", c.BloomThreshold)
	r.p.bright.SetFloat("uKnee", c.BloomKnee)
	r.blitTriangle()

	// Step down.
	r.p.bloom.Use()
	r.p.bloom.SetInt("uMode", 0)
	for i := 1; i < len(r.bloomTex); i++ {
		src, dst := r.bloomTex[i-1], r.bloomTex[i]
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, dst.ID(), 0)
		gfx.Viewport(dst.Width(), dst.Height())
		src.Bind(0)
		r.p.bloom.SetInt("uSrc", 0)
		r.p.bloom.SetVec2("uTexel", 1/float32(src.Width()), 1/float32(src.Height()))
		r.blitTriangle()
	}
	// Step back up, each level landing in the one below it.
	r.p.bloom.SetInt("uMode", 1)
	for i := len(r.bloomTex) - 2; i >= 0; i-- {
		src, dst := r.bloomTex[i+1], r.bloomTex[i]
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, dst.ID(), 0)
		gfx.Viewport(dst.Width(), dst.Height())
		src.Bind(0)
		r.p.bloom.SetInt("uSrc", 0)
		r.p.bloom.SetVec2("uTexel", 1/float32(dst.Width()), 1/float32(dst.Height()))
		r.blitTriangle()
	}
}

// compositePass tone maps and encodes to sRGB, drawing to the window or to the
// capture target.
func (r *Renderer) compositePass(v View) {
	c := &r.cfg
	if r.captureFB != nil {
		r.captureFB.Bind()
	} else {
		gfx.BindDefaultFramebuffer(r.w, r.h)
	}
	// The bloom chain left the viewport at the size of the smallest mip, so it has
	// to be restored here or the composite paints a postage stamp in the corner.
	gfx.Viewport(r.w, r.h)
	gfx.Disable(gl.DEPTH_TEST)
	gfx.DepthMask(false)
	gfx.Disable(gl.CULL_FACE)
	r.p.composite.Use()
	r.hdrTex.Bind(0)
	if len(r.bloomTex) > 0 {
		r.bloomTex[0].Bind(1)
	} else {
		r.hdrTex.Bind(1) // nothing to add; the strength below is zero anyway
	}
	r.p.composite.SetInt("uHDR", 0)
	r.p.composite.SetInt("uBloom", 1)
	r.p.composite.SetFloat("uExposure", c.Exposure)
	if len(r.bloomTex) > 0 {
		r.p.composite.SetFloat("uBloomStrength", c.BloomStrength)
	} else {
		r.p.composite.SetFloat("uBloomStrength", 0)
	}
	r.p.composite.SetFloat("uSaturation", c.Saturation)
	r.p.composite.SetFloat("uWhite", c.White)
	r.p.composite.SetFloat("uContrast", c.Contrast)
	r.p.composite.SetFloat("uVignette", c.Vignette)
	r.p.composite.SetFloat("uDither", c.Dither)
	if c.ACES {
		r.p.composite.SetInt("uTonemap", 1)
	} else {
		r.p.composite.SetInt("uTonemap", 0)
	}
	r.blitTriangle()
}

// hudPass draws the debug overlay straight on top of the presented frame.
func (r *Renderer) hudPass(v View) {
	h := r.beginOverlay()
	if h == nil {
		return
	}
	wf, hf := float32(r.w), float32(r.h)
	white := [3]float32{0.92, 0.94, 1.0}
	dim := [3]float32{0.55, 0.60, 0.72}
	accent := [3]float32{0.45, 0.85, 1.0}
	x, y := float32(14), float32(12)
	h.Text(x, y, "GO RUBIK'S CUBE", accent, 0.9)
	y += lineStep
	h.Text(x, y, "FPS "+fix1(v.FPS)+"   T "+fix1(float32(v.Time))+"S", white, 0.85)
	y += lineStep
	h.Text(x, y, "PHASE "+v.Phase, dim, 0.9)
	y += lineStep
	h.Text(x, y, "MOVE "+v.Move, dim, 0.9)
	y += lineStep
	solved := "NO"
	if v.Cube.IsSolved() {
		solved = "YES"
	}
	h.Text(x, y, "CUBIES 26  MOVES "+itoa(v.Cube.MoveCount())+"  SOLVED "+solved, white, 0.85)
	y += lineStep
	if v.Turn != nil {
		h.Text(x, y, "LAYER "+v.Turn.Move.String()+"  9 CUBIES  ANGLE "+
			fix1(float32(float64(v.Turn.Angle)*180/math.Pi))+" DEG", dim, 0.85)
		y += lineStep
	}
	h.Text(x, hf-24, "MSAA "+itoa(int(r.samples))+
		"  "+itoa(int(wf))+"X"+itoa(int(hf))+
		"  ENV "+itoa(int(r.cfg.EnvMapSize))+"X"+itoa(int(r.envMips))+" MIPS", dim, 0.7)

	if r.captureFB != nil {
		r.captureFB.Bind()
	} else {
		gfx.BindDefaultFramebuffer(r.w, r.h)
	}
	gfx.Viewport(r.w, r.h)
	r.p.hud.Use()
	r.p.hud.SetVec2("uScreen", wf, hf)
	r.p.hud.SetInt("uAtlas", 0)
	h.drawOverlay()
}

// blit copies bits between framebuffers, which is how a multisampled target is
// resolved into a texture for the passes that follow.
func blit(src, dst *gfx.Framebuffer, w, h int32, mask uint32) {
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, src.ID())
	gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, dst.ID())
	gl.BlitFramebuffer(0, 0, w, h, 0, 0, w, h, mask, gl.NEAREST)
	gl.BindFramebuffer(gl.FRAMEBUFFER, dst.ID())
}

// grey turns a single-channel albedo into the vec3 the shaders want.
func grey(v float32) [3]float32 { return [3]float32{v, v, v} }

// itoa and fix1 keep the overlay free of fmt, which is worth a little here because
// the overlay is built every frame.
func itoa(v int) string { return strconv.Itoa(v) }

func fix1(v float32) string { return strconv.FormatFloat(float64(v), 'f', 1, 32) }
