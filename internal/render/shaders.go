package render

import (
	"embed"
	"fmt"
	"strings"

	"github.com/suaro/gorubik/internal/gfx"
)

// Shaders are compiled into the binary, so the executable runs from anywhere and a
// shader error can never be a missing-file error at runtime. The whole directory is
// embedded: the .vert and .frag files are the passes, and common.glsl is the shared
// body stitched in front of each fragment shader.
//
//go:embed shaders
var shaderFS embed.FS

// Fragment shaders are built from common.glsl plus the file itself: GLSL has no
// include mechanism, and the alternative - repeating the BRDF in eight files - is how
// a renderer ends up with eight subtly different lighting models. The #version line
// stays first, because GLSL requires it to be.
func loadFrag(name string) (string, error) {
	body, err := shaderFS.ReadFile("shaders/" + name + ".frag")
	if err != nil {
		return "", err
	}
	common, err := shaderFS.ReadFile("shaders/common.glsl")
	if err != nil {
		return "", err
	}
	return stitch(string(common), string(body)), nil
}

func loadVert(name string) (string, error) {
	body, err := shaderFS.ReadFile("shaders/" + name + ".vert")
	if err != nil {
		return "", err
	}
	return stitch("", string(body)), nil
}

// stitch puts the version directive on its own first line, then the shared body, then
// the pass-specific body.
func stitch(common, body string) string {
	version := "#version 430 core\n"
	rest := body
	if i := strings.Index(body, "\n"); i >= 0 && strings.HasPrefix(strings.TrimSpace(body), "#version") {
		rest = body[i+1:]
	}
	if common == "" {
		return version + rest
	}
	return version + strings.TrimPrefix(common, "\ufeff") + "\n" + rest
}

// programs is the full set of compiled passes.
type programs struct {
	cube       *gfx.Program
	depth      *gfx.Program
	prepass    *gfx.Program
	sky        *gfx.Program
	floor      *gfx.Program
	ssao       *gfx.Program
	aoBlur     *gfx.Program
	envGen     *gfx.Program
	envIrrad   *gfx.Program
	envPrefilt *gfx.Program
	bright     *gfx.Program
	bloom      *gfx.Program
	composite  *gfx.Program
	hud        *gfx.Program
}

func (p *programs) close() {
	for _, pr := range []*gfx.Program{p.cube, p.depth, p.prepass, p.sky, p.floor, p.ssao,
		p.aoBlur, p.envGen, p.envIrrad, p.envPrefilt, p.bright, p.bloom, p.composite, p.hud} {
		if pr != nil {
			pr.Delete()
		}
	}
}

// linkPairs builds every program the renderer needs. Errors name the pass, because a
// link failure deep in an initialisation sequence is otherwise guesswork.
func linkPairs() (*programs, error) {
	var p programs
	type spec struct {
		out  **gfx.Program
		name string
		vert string
	}
	specs := []spec{
		{&p.cube, "cube", "cube"},
		{&p.depth, "depth", "depth"},
		{&p.prepass, "prepass", "prepass"},
		{&p.sky, "sky", "sky"},
		{&p.floor, "floor", "floor"},
		{&p.ssao, "ssao", "fullscreen"},
		{&p.aoBlur, "ssaoBlur", "fullscreen"},
		{&p.envGen, "envgen", "fullscreen"},
		{&p.envIrrad, "envIrradiance", "fullscreen"},
		{&p.envPrefilt, "envPrefilter", "fullscreen"},
		{&p.bright, "bright", "fullscreen"},
		{&p.bloom, "bloom", "fullscreen"},
		{&p.composite, "composite", "fullscreen"},
	}
	for _, s := range specs {
		vs, err := loadVert(s.vert)
		if err != nil {
			return nil, err
		}
		fs, err := loadFrag(s.name)
		if err != nil {
			return nil, err
		}
		pr, err := gfx.NewProgram(s.name, vs, fs)
		if err != nil {
			return nil, err
		}
		*s.out = pr
	}
	// The HUD has no lighting code in it at all, so it is stitched without common.
	vs, err := loadVert("hud")
	if err != nil {
		return nil, err
	}
	hudFrag, err := shaderFS.ReadFile("shaders/hud.frag")
	if err != nil {
		return nil, err
	}
	p.hud, err = gfx.NewProgram("hud", vs, stitch("", string(hudFrag)))
	if err != nil {
		return nil, fmt.Errorf("hud: %w", err)
	}
	return &p, nil
}
