// Package render draws the scene. It is a small deferred-style pipeline:
//
//	shadow map  ->  depth/normal prepass  ->  screen-space AO  ->  forward HDR
//	->  bloom  ->  tone map and sRGB encode
//
// Every stage renders into a float buffer and the frame is encoded to sRGB exactly
// once, at the end, which is what keeps sticker colours vivid instead of muddy.
package render

import (
	"fmt"
	"math"
	"time"

	"github.com/go-gl/gl/v4.3-core/gl"

	"github.com/suaro/gorubik/internal/cube"
	"github.com/suaro/gorubik/internal/geo"
	"github.com/suaro/gorubik/internal/gfx"
	"github.com/suaro/gorubik/internal/math3d"
)

// InitGL binds the GL entry points and reports the driver. It must run with a current
// context.
func InitGL() (version, renderer string, err error) {
	return gfx.Init()
}

// Renderer owns every GPU resource in the program.
type Renderer struct {
	cfg Settings
	p   *programs

	debug bool

	// Cube geometry, instanced.
	bodyVBO   *gfx.Buffer
	bodyVAO   *gfx.VAO
	bodyTris  int32
	stickVBO  *gfx.Buffer
	stickVAO  *gfx.VAO
	stickTris int32
	bodyInst  *gfx.Buffer // 26 * (mat4 + vec4 + vec4)
	stickInst *gfx.Buffer
	instCPU   []float32 // scratch, reused every frame
	emptyVAO  *gfx.VAO

	// Reflection environment.
	envSpec  *gfx.CubeMap
	envIrrad *gfx.CubeMap
	envMips  int32

	// Shadow map.
	shadowFB  *gfx.Framebuffer
	shadowTex *gfx.Texture
	litTex    *gfx.Texture // 1x1 far-plane map, bound when shadows are off

	// The shaded pass, multisampled.
	mainFB  *gfx.Framebuffer
	mainHDR *gfx.Renderbuffer
	mainDep *gfx.Renderbuffer

	// The AO prepass: normals and depth at AO resolution, resolved in one fill.
	preFB  *gfx.Framebuffer
	aw, ah int32 // AO-chain extent, half the frame

	// Resolved / auxiliary textures.
	postFB    *gfx.Framebuffer // scratch, colour attachment swapped per pass
	hdrTex    *gfx.Texture
	normalTex *gfx.Texture
	depthTex  *gfx.Texture
	aoTex     *gfx.Texture
	aoBlurTex *gfx.Texture
	bloomTex  []*gfx.Texture

	// Offscreen capture target.
	captureFB  *gfx.Framebuffer
	captureDep *gfx.Renderbuffer
	captureTex *gfx.Texture

	// Debug overlay.
	hud    *hud
	hudTex *gfx.Texture

	// Sticker placement, per cubie-local face: transforms a sticker built on the
	// +Z plane onto that face of the plastic.
	localStickers [6]math3d.Mat4

	// The pose uploaded most recently, kept for diagnostics.
	lastFrame cube.Frame

	// Instance counts and the last uploaded frame, used to skip redundant work.
	w, h     int32
	samples  int32
	aoKernel [][3]float32
	aoFlat   []float32

	// Matrices for the current frame.
	view, proj, viewProj, invViewProj, lightViewProj math3d.Mat4
	camPos                                           math3d.Vec3
	shadowTexelWorld                                 float32

	// Per-pass timing, non-nil only with Settings.PassTimes.
	pt *passTimer
}

// New compiles every shader and builds the environment. Call it with a current GL
// context.
func New() (*Renderer, error) {
	return NewWithSettings(DefaultSettings())
}

// NewWithSettings builds a renderer with an explicit visual configuration.
func NewWithSettings(cfg Settings) (*Renderer, error) {
	p, err := linkPairs()
	if err != nil {
		return nil, err
	}
	r := &Renderer{cfg: cfg, p: p}
	if cfg.PassTimes {
		r.pt = &passTimer{acc: map[string]time.Duration{}}
	}
	r.emptyVAO = gfx.NewVAO()
	gfx.Unbind()

	r.aoKernel = makeAOKernel(cfg.AOKernel)
	// Each stage is checked on its own, because a GL error reported at the end of
	// initialisation names nothing and a shader or target problem is the one class
	// of bug that is otherwise invisible until the window is black.
	for _, s := range []struct {
		name string
		fn   func() error
	}{
		{"geometry", r.buildMeshes},
		{"environment", r.buildEnv},
		{"shadow map", r.buildShadow},
		{"overlay", r.initHUD},
	} {
		if err := s.fn(); err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
		if err := gfx.PollErrors(s.name); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Close releases every GPU resource.
func (r *Renderer) Close() {
	if r.p != nil {
		r.p.close()
		r.p = nil
	}
	r.dropTargets()
	for _, b := range []*gfx.Buffer{r.bodyInst, r.stickInst} {
		b.Delete()
	}
	for _, v := range []*gfx.VAO{r.bodyVAO, r.stickVAO, r.emptyVAO} {
		v.Delete()
	}
	r.envSpec.Delete()
	r.envIrrad.Delete()
	r.shadowFB.Delete()
	r.shadowTex.Delete()
	r.litTex.Delete()
	r.closeHUD()
}

// ToggleDebug flips the overlay; bound to the H key.
func (r *Renderer) ToggleDebug() { r.debug = !r.debug }

// Debug reports whether the overlay is on.
func (r *Renderer) Debug() bool { return r.debug }

// Settings exposes the tuning block so a caller can adjust the look from flags.
func (r *Renderer) Settings() *Settings { return &r.cfg }

// Samples reports the multisampling level actually in use.
func (r *Renderer) Samples() int32 { return r.samples }

// ---------------------------------------------------------------------------
// Geometry
// ---------------------------------------------------------------------------

// buildMeshes uploads the cubie body and the sticker plate once. Both are drawn
// instanced, so 26 cubies and 54 stickers cost two draw calls and one copy of the
// vertex data each.
func (r *Renderer) buildMeshes() error {
	body := geo.RoundedBox(r.cfg.CubieHalf, r.cfg.BevelRadius, r.cfg.CornerDetail)
	stick := geo.StickerPlate(r.cfg.StickerHalf, r.cfg.StickerRound, r.cfg.StickerThick,
		r.cfg.StickerChamf, r.cfg.StickerSegs)
	// Self-checks run on every start, not only in tests: a mesh that is not watertight
	// would show up as light leaking into the cube, and that is a miserable bug to
	// chase through pixels.
	if err := body.Check([3]float32{-r.cfg.CubieHalf, -r.cfg.CubieHalf, -r.cfg.CubieHalf},
		[3]float32{r.cfg.CubieHalf, r.cfg.CubieHalf, r.cfg.CubieHalf}); err != nil {
		return err
	}
	if err := stick.Check([3]float32{-r.cfg.StickerHalf, -r.cfg.StickerHalf, 0},
		[3]float32{r.cfg.StickerHalf, r.cfg.StickerHalf, r.cfg.StickerThick}); err != nil {
		return err
	}

	r.bodyTris = int32(body.IdxCount())
	r.stickTris = int32(stick.IdxCount())

	r.bodyVAO, r.bodyVBO, r.bodyInst = uploadInstanced(body)
	r.stickVAO, r.stickVBO, r.stickInst = uploadInstanced(stick)
	gfx.Unbind()

	// 26 bodies and 54 stickers, 24 floats each.
	r.instCPU = make([]float32, (26+54)*24)
	r.localStickers = geo.FaceTransforms(r.cfg.CubieHalf)
	r.aoFlat = make([]float32, 3*len(r.aoKernel))
	for i, k := range r.aoKernel {
		r.aoFlat[3*i], r.aoFlat[3*i+1], r.aoFlat[3*i+2] = k[0], k[1], k[2]
	}
	return nil
}

// uploadInstanced creates the vertex array for a mesh plus its dynamic instance
// buffer. Attribute locations match cube.vert / depth.vert / prepass.vert.
func uploadInstanced(m *geo.Mesh) (*gfx.VAO, *gfx.Buffer, *gfx.Buffer) {
	vao := gfx.NewVAO()
	vb := gfx.NewBuffer(gl.ARRAY_BUFFER, m.Interleaved(), gl.STATIC_DRAW)
	ib := gfx.NewBufferU(gl.ELEMENT_ARRAY_BUFFER, m.Idx, gl.STATIC_DRAW)
	inst := gfx.NewBuffer(gl.ARRAY_BUFFER, make([]float32, 64*24), gl.DYNAMIC_DRAW)

	// The element buffer binding is recorded in the VAO, so it has to happen here
	// even though no attribute refers to it.
	ib.Bind()
	vb.Bind()
	stride := int32(geo.VertexStride)
	gfx.SetAttribute(gfx.Attribute{Location: 0, Size: 3, Type: gl.FLOAT, Stride: stride})
	gfx.SetAttribute(gfx.Attribute{Location: 1, Size: 3, Type: gl.FLOAT, Stride: stride, Offset: 12})
	const istride = 24 * 4
	inst.Bind()
	for i := 0; i < 4; i++ {
		gfx.SetAttribute(gfx.Attribute{
			Location: int32(2 + i), Size: 4, Type: gl.FLOAT,
			Stride: istride, Offset: int32(i * 16), Divisor: 1,
		})
	}
	gfx.SetAttribute(gfx.Attribute{Location: 6, Size: 4, Type: gl.FLOAT, Stride: istride, Offset: 64, Divisor: 1})
	gfx.SetAttribute(gfx.Attribute{Location: 7, Size: 4, Type: gl.FLOAT, Stride: istride, Offset: 80, Divisor: 1})
	return vao, vb, inst
}

// ---------------------------------------------------------------------------
// Targets
// ---------------------------------------------------------------------------

// buildShadow allocates the depth map. A depth *texture* (not a renderbuffer) is
// required because the lighting pass samples it with hardware compare.
func (r *Renderer) buildShadow() error {
	n := r.cfg.ShadowSize
	r.shadowTex = gfx.NewTexture2D(gfx.TexSpec{
		Width: n, Height: n,
		Internal: gl.DEPTH_COMPONENT32F, Format: gl.DEPTH_COMPONENT, Type: gl.FLOAT,
		Filter: gl.LINEAR, Comparison: true,
	})
	r.shadowFB = gfx.NewFramebuffer()
	r.shadowFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, r.shadowTex.ID(), 0)
	r.shadowFB.DrawNone()
	if err := r.shadowFB.Check(); err != nil {
		return err
	}
	gfx.BindDefaultFramebuffer(1, 1)

	// A 1x1 map filled with the far value, bound instead of the real map when the
	// shadow is switched off. Comparing a frame with and against it is how a shadow
	// artefact gets attributed to the map rather than to the lighting.
	r.litTex = gfx.NewTexture2D(gfx.TexSpec{Width: 1, Height: 1,
		Internal: gl.DEPTH_COMPONENT32F, Format: gl.DEPTH_COMPONENT, Type: gl.FLOAT,
		Filter: gl.NEAREST, Comparison: true})
	litFB := gfx.NewFramebuffer()
	litFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, r.litTex.ID(), 0)
	gl.ClearDepthf(1)
	gl.Clear(gl.DEPTH_BUFFER_BIT)
	litFB.Delete()
	gfx.BindDefaultFramebuffer(1, 1)
	return nil
}

// Resize rebuilds every full-resolution target. It is called on window resize and on
// capture setup.
func (r *Renderer) Resize(w, h int32) error {
	if w == r.w && h == r.h && r.mainFB != nil {
		return nil
	}
	r.dropTargets()
	r.w, r.h = w, h

	// Eight samples is the right answer at 1600x1000; on a 5K display the same
	// setting would cost four times the memory for no visible gain, so taper.
	samples := r.cfg.MSAA
	if px := w * h; px > 6_000_000 {
		samples = 4
	}
	if px := w * h; px > 14_000_000 {
		samples = 2
	}
	if m := maxSamples(); m > 0 && samples > m {
		samples = m
	}
	r.samples = samples

	r.mainFB = gfx.NewFramebuffer()
	r.mainFB.Bind()
	r.mainHDR = gfx.NewMultisampleRenderbuffer(samples, gl.RGBA16F, w, h)
	r.mainDep = gfx.NewMultisampleRenderbuffer(samples, gl.DEPTH_COMPONENT24, w, h)
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.RENDERBUFFER, r.mainHDR.ID())
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, r.mainDep.ID())
	if err := r.mainFB.Check(); err != nil {
		return fmt.Errorf("main target: %w", err)
	}
	gfx.BindDefaultFramebuffer(1, 1)

	r.postFB = gfx.NewFramebuffer()

	r.hdrTex = gfx.NewTexture2D(gfx.TexSpec{Width: w, Height: h,
		Internal: gl.RGBA16F, Format: gl.RGBA, Type: gl.HALF_FLOAT})
	// The whole AO chain runs at half resolution. Occlusion is a low-frequency term
	// that is bilateral-blurred afterwards anyway, and at full resolution it plus
	// the prepass that feeds it measured 24 ms of a 50 ms frame.
	r.aw, r.ah = max1(w/2), max1(h/2)
	r.normalTex = gfx.NewTexture2D(gfx.TexSpec{Width: r.aw, Height: r.ah,
		Internal: gl.RGBA16F, Format: gl.RGBA, Type: gl.HALF_FLOAT})
	r.depthTex = gfx.NewTexture2D(gfx.TexSpec{Width: r.aw, Height: r.ah,
		Internal: gl.DEPTH_COMPONENT24, Format: gl.DEPTH_COMPONENT, Type: gl.UNSIGNED_INT,
		Filter: gl.NEAREST})
	r.aoTex = gfx.NewTexture2D(gfx.TexSpec{Width: r.aw, Height: r.ah,
		Internal: gl.R8, Format: gl.RED, Type: gl.UNSIGNED_BYTE})
	r.aoBlurTex = gfx.NewTexture2D(gfx.TexSpec{Width: r.aw, Height: r.ah,
		Internal: gl.R8, Format: gl.RED, Type: gl.UNSIGNED_BYTE})
	r.preFB = gfx.NewFramebuffer()
	r.preFB.Bind()
	r.preFB.AttachTexture(gl.COLOR_ATTACHMENT0, r.normalTex, gl.TEXTURE_2D)
	r.preFB.AttachTexture(gl.DEPTH_ATTACHMENT, r.depthTex, gl.TEXTURE_2D)
	if err := r.preFB.Check(); err != nil {
		return fmt.Errorf("ao prepass target: %w", err)
	}

	// The chain starts at quarter size rather than half: bloom is a blur whose
	// widest term is several tiers down, so the first tier only has to carry the
	// shape of the highlight, and shading it at 1/16th the pixels is most of the
	// saving.
	r.bloomTex = nil
	tw, th := w/4, h/4
	for i := 0; i < r.cfg.BloomTiers; i++ {
		tw, th = max1(tw/2), max1(th/2)
		r.bloomTex = append(r.bloomTex, gfx.NewTexture2D(gfx.TexSpec{Width: tw, Height: th,
			Internal: gl.RGBA16F, Format: gl.RGBA, Type: gl.HALF_FLOAT}))
	}
	gfx.BindDefaultFramebuffer(w, h)
	return nil
}

func (r *Renderer) dropTargets() {
	if r.preFB != nil {
		r.preFB.Delete()
		r.preFB = nil
	}
	for _, t := range []*gfx.Texture{r.hdrTex, r.normalTex, r.depthTex, r.aoTex, r.aoBlurTex} {
		t.Delete()
	}
	for _, t := range r.bloomTex {
		t.Delete()
	}
	for _, f := range []*gfx.Framebuffer{r.postFB, r.mainFB} {
		f.Delete()
	}
	for _, rb := range []*gfx.Renderbuffer{r.mainHDR, r.mainDep} {
		rb.Delete()
	}
	r.hdrTex, r.normalTex, r.depthTex, r.aoTex, r.aoBlurTex = nil, nil, nil, nil, nil
	r.bloomTex = nil
	r.postFB = nil
	r.mainFB, r.mainHDR, r.mainDep = nil, nil, nil
}

func max1(v int32) int32 {
	if v < 1 {
		return 1
	}
	return v
}

// maxSamples asks the driver what it will actually do, so a request for 8x on hardware
// that only offers 4x degrades instead of failing.
func maxSamples() int32 {
	var n int32
	gl.GetIntegerv(gl.MAX_SAMPLES, &n)
	return n
}

// ---------------------------------------------------------------------------
// Ambient occlusion kernel
// ---------------------------------------------------------------------------

// makeAOKernel builds a hemisphere sampling kernel with samples biased toward the
// surface, which is what makes crevices darken without smearing the whole cubie.
func makeAOKernel(n int) [][3]float32 {
	k := make([][3]float32, 0, n)
	seed := uint32(0x1234abcd)
	rnd := func() float32 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return float32(seed) / float32(1<<32)
	}
	for i := 0; i < n; i++ {
		var v [3]float32
		for len(v) > 0 {
			v = [3]float32{rnd()*2 - 1, rnd()*2 - 1, rnd()*2 - 1}
			l := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
			if l > 1e-4 && l <= 1 {
				break
			}
		}
		l := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
		v = [3]float32{v[0] / l, v[1] / l, v[2] / l}
		f := float32(i) / float32(n)
		scale := 0.1 + 0.9*f*f
		k = append(k, [3]float32{v[0] * scale, v[1] * scale, v[2] * scale})
	}
	return k
}
