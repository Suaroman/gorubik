package render

import (
	"math"

	"github.com/go-gl/gl/v4.3-core/gl"

	"github.com/Suaroman/gorubik/internal/gfx"
)

// The studio environment is generated once at start-up from the analytic function
// in common.glsl and then convolved into the two maps the lighting pass reads:
// a Lambertian irradiance cube and a GGX-prefiltered specular cube.
//
// faceBases must match the hardware's own cube-map conventions, because the
// convolution passes *sample* the source cube with textureCube while writing it
// with an explicit basis. If the two disagreed, the specular reflection of the key
// light would arrive from a direction the diffuse term insists is dark.
// Order is TEXTURE_CUBE_MAP_POSITIVE_X + i.
var faceBases = [6][9]float32{
	{1, 0, 0, 0, 0, -1, 0, -1, 0},  // +X: fwd +X, right -Z, up -Y
	{-1, 0, 0, 0, 0, 1, 0, -1, 0},  // -X
	{0, 1, 0, 1, 0, 0, 0, 0, 1},    // +Y
	{0, -1, 0, 1, 0, 0, 0, 0, -1},  // -Y
	{0, 0, 1, 1, 0, 0, 0, -1, 0},   // +Z
	{0, 0, -1, -1, 0, 0, 0, -1, 0}, // -Z
}

func fwdOf(f int) [3]float32 { return [3]float32{faceBases[f][0], faceBases[f][1], faceBases[f][2]} }
func rightOf(f int) [3]float32 {
	return [3]float32{faceBases[f][3], faceBases[f][4], faceBases[f][5]}
}
func upOf(f int) [3]float32 { return [3]float32{faceBases[f][6], faceBases[f][7], faceBases[f][8]} }

// setLightDirs pushes the three studio directions into a program. They are shared
// by the analytic environment and the direct-light sum, so a softbox in a
// reflection and the highlight it produces cannot drift apart.
func (r *Renderer) setLightDirs(p *gfx.Program) {
	p.SetVec3("uKeyLightDir", r.cfg.KeyDir)
	p.SetVec3("uFillLightDir", r.cfg.FillDir)
	p.SetVec3("uRimLightDir", r.cfg.RimDir)
}

// buildEnv renders and convolves the environment. It runs once, before any frame.
func (r *Renderer) buildEnv() error {
	size := r.cfg.EnvMapSize
	mips := int32(1)
	for s := size; s > 1; s >>= 1 {
		mips++
	}
	r.envMips = mips

	src := gfx.NewCubeMap(size, 1, gl.RGBA16F, gl.RGBA, gl.HALF_FLOAT)
	defer src.Delete()
	// The same studio without its area lights, for the diffuse ambient term.
	room := gfx.NewCubeMap(size, 1, gl.RGBA16F, gl.RGBA, gl.HALF_FLOAT)
	defer room.Delete()
	r.envIrrad = gfx.NewCubeMap(64, 1, gl.RGBA16F, gl.RGBA, gl.HALF_FLOAT)
	r.envSpec = gfx.NewCubeMap(size, mips, gl.RGBA16F, gl.RGBA, gl.HALF_FLOAT)

	fb := gfx.NewFramebuffer()
	defer fb.Delete()
	defer gfx.BindDefaultFramebuffer(1, 1)

	gfx.Disable(gl.DEPTH_TEST)
	gfx.Disable(gl.CULL_FACE)
	gfx.Disable(gl.BLEND)
	gfx.Viewport(size, size)

	// --- 1. the analytic studio, one face at a time --------------------------
	// Two copies of the same function: with the softboxes and without. The bright
	// one feeds the specular prefilter, the plain one feeds the irradiance, so the
	// analytic key light is not also paid for as ambient. See studioEnv in common.glsl.
	r.p.envGen.Use()
	r.setLightDirs(r.p.envGen)
	for _, pass := range []struct {
		tex       *gfx.CubeMap
		softboxes float32
	}{{src, 1}, {room, 0}} {
		r.p.envGen.SetFloat("uSoftboxes", pass.softboxes)
		for f := 0; f < 6; f++ {
			fb.Bind()
			gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0,
				gl.TEXTURE_CUBE_MAP_POSITIVE_X+uint32(f), pass.tex.ID(), 0)
			r.p.envGen.SetVec3("uFwd", fwdOf(f))
			r.p.envGen.SetVec3("uRight", rightOf(f))
			r.p.envGen.SetVec3("uUp", upOf(f))
			r.blitTriangle()
		}
	}

	// --- 2. irradiance: cosine-weighted hemisphere convolution ---------------
	irradSize := int32(64)
	gfx.Viewport(irradSize, irradSize)
	r.p.envIrrad.Use()
	r.p.envIrrad.SetInt("uEnv", 0)
	r.p.envIrrad.SetInt("uSamples", 96)
	room.Bind(0)
	for f := 0; f < 6; f++ {
		fb.Bind()
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0,
			gl.TEXTURE_CUBE_MAP_POSITIVE_X+uint32(f), r.envIrrad.ID(), 0)
		r.p.envIrrad.SetVec3("uFwd", fwdOf(f))
		r.p.envIrrad.SetVec3("uRight", rightOf(f))
		r.p.envIrrad.SetVec3("uUp", upOf(f))
		r.blitTriangle()
	}

	// --- 3. prefiltered specular: one mip per roughness step -----------------
	r.p.envPrefilt.Use()
	r.p.envPrefilt.SetInt("uEnv", 0)
	r.p.envPrefilt.SetInt("uSamples", 128)
	src.Bind(0)
	for l := int32(0); l < mips; l++ {
		ls := max1(size >> l)
		gfx.Viewport(ls, ls)
		r.p.envPrefilt.SetFloat("uRoughness", float32(l)/float32(mips-1))
		for f := 0; f < 6; f++ {
			fb.Bind()
			gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0,
				gl.TEXTURE_CUBE_MAP_POSITIVE_X+uint32(f), r.envSpec.ID(), l)
			r.p.envPrefilt.SetVec3("uFwd", fwdOf(f))
			r.p.envPrefilt.SetVec3("uRight", rightOf(f))
			r.p.envPrefilt.SetVec3("uUp", upOf(f))
			r.blitTriangle()
		}
	}

	// The mip chain was filled by hand, so trilinear filtering is available without
	// generating anything; roughness then indexes it continuously.
	r.envSpec.Bind(0)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	r.envIrrad.Bind(0)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)

	if e := gfx.PollErrors("env"); e != nil {
		return e
	}
	return nil
}

// blitTriangle runs the current program over the whole viewport using the
// no-attribute big triangle.
func (r *Renderer) blitTriangle() {
	r.emptyVAO.Bind()
	gfx.DrawTriangleArrays(0, 3)
}

// envMipCount is the number of prefiltered levels, exposed for the README and the
// debug overlay.
func (r *Renderer) envMipCount() int { return int(math.Max(1, float64(r.envMips))) }
