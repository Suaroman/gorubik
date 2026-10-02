package render

import (
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.3-core/gl"

	"github.com/Suaroman/gorubik/internal/gfx"
)

// The debug overlay is drawn from a hand-built 5x7 bitmap font rather than a text
// shaping library. The overlay only ever prints identifiers, numbers and a handful
// of punctuation marks, and a built-in atlas means the executable has no font to
// find at runtime - which matters most for the Windows build.

const (
	glyphW    = 5
	glyphH    = 7
	cellW     = 6 // one pixel of slack per glyph so neighbouring cells never touch
	cellH     = 8
	atlasCols = 128 // printable ASCII
	textScale = 2   // on-screen magnification; nearest filtering keeps it crisp
	glyphAdv  = float32(cellW * textScale)
	lineStep  = float32((glyphH + 3) * textScale)

	// hudQuads bounds one frame of overlay text.
	hudQuads = 4096
)

// font5x7 maps a rune to seven rows of five cells, "#" inked and "." empty.
var font5x7 = map[rune]string{
	' ':  "...../...../...../...../...../...../.....",
	'A':  ".###./#...#/#...#/#####/#...#/#...#/#...#",
	'B':  "####./#...#/#...#/####./#...#/#...#/####.",
	'C':  ".###./#...#/#..../#..../#..../#...#/.###.",
	'D':  "####./#...#/#...#/#...#/#...#/#...#/####.",
	'E':  "#####/#..../#..../####./#..../#..../#####",
	'F':  "#####/#..../#..../####./#..../#..../#....",
	'G':  ".###./#...#/#..../#.###/#...#/#...#/.###.",
	'H':  "#...#/#...#/#...#/#####/#...#/#...#/#...#",
	'I':  "#####/..#../..#../..#../..#../..#../#####",
	'J':  "..###/...#./...#./...#./...#./#..#./.##..",
	'K':  "#...#/#..#./#.#../##.../#.#../#..#./#...#",
	'L':  "#..../#..../#..../#..../#..../#..../#####",
	'M':  "#...#/##.##/#.#.#/#.#.#/#...#/#...#/#...#",
	'N':  "#...#/##..#/##..#/#.#.#/#..##/#..##/#...#",
	'O':  ".###./#...#/#...#/#...#/#...#/#...#/.###.",
	'P':  "####./#...#/#...#/####./#..../#..../#....",
	'Q':  ".###./#...#/#...#/#...#/#.#.#/#..#./.##.#",
	'R':  "####./#...#/#...#/####./#.#../#..#./#...#",
	'S':  ".####/#..../#..../.###./....#/....#/####.",
	'T':  "#####/..#../..#../..#../..#../..#../..#..",
	'U':  "#...#/#...#/#...#/#...#/#...#/#...#/.###.",
	'V':  "#...#/#...#/#...#/#...#/#...#/.#.#./..#..",
	'W':  "#...#/#...#/#...#/#.#.#/#.#.#/##.##/#...#",
	'X':  "#...#/#...#/.#.#./..#../.#.#./#...#/#...#",
	'Y':  "#...#/#...#/.#.#./..#../..#../..#../..#..",
	'Z':  "#####/....#/...#./..#../.#.../#..../#####",
	'0':  ".###./#...#/#..##/#.#.#/##..#/#...#/.###.",
	'1':  "..#../.##../..#../..#../..#../..#../.###.",
	'2':  ".###./#...#/....#/...#./..#../.#.../#####",
	'3':  "####./....#/....#/.###./....#/....#/####.",
	'4':  "...#./..##./.#.#./#..#./#####/...#./...#.",
	'5':  "#####/#..../#..../####./....#/....#/####.",
	'6':  ".###./#..../#..../####./#...#/#...#/.###.",
	'7':  "#####/....#/...#./..#../.#.../.#.../.#...",
	'8':  ".###./#...#/#...#/.###./#...#/#...#/.###.",
	'9':  ".###./#...#/#...#/.####/....#/....#/.###.",
	'.':  "...../...../...../...../...../.##../.##..",
	',':  "...../...../...../...../.##../.##../.#...",
	':':  "...../.##../.##../...../.##../.##../.....",
	';':  "...../.##../.##../...../.##../.##../.#...",
	'!':  "..#../..#../..#../..#../..#../...../..#..",
	'?':  ".###./#...#/....#/...#./..#../...../..#..",
	'\'': ".#.../.#.../...../...../...../...../.....",
	'"':  ".#.#./.#.#./...../...../...../...../.....",
	'(':  "...#./..#../.#.../.#.../.#.../..#../...#.",
	')':  ".#.../..#../...#./...#./...#./..#../.#...",
	'[':  ".###./.#.../.#.../.#.../.#.../.#.../.###.",
	']':  ".###./...#./...#./...#./...#./...#./.###.",
	'-':  "...../...../...../.###./...../...../.....",
	'+':  "...../..#../..#../#####/..#../..#../.....",
	'=':  "...../...../#####/...../#####/...../.....",
	'/':  "....#/....#/...#./..#../.#.../#..../#....",
	'%':  "##..#/##..#/...#./..#../.#.../#..##/#..##",
	'*':  "...../#.#.#/.###./#####/.###./#.#.#/.....",
	'#':  ".#.#./.#.#./#####/.#.#./#####/.#.#./.#.#.",
	'_':  "...../...../...../...../...../...../#####",
	'<':  "...#./..#../.#.../#..../.#.../..#../...#.",
	'>':  ".#.../..#../...#./....#/...#./..#../.#...",
	'&':  ".##../#..#./#..#./.##../#.#.#/#..#./.##.#",
	'@':  ".###./#...#/#.###/#.#.#/#.###/#..../.###.",
	'|':  "..#../..#../..#../..#../..#../..#../..#..",
}

// hud owns the glyph atlas and the geometry buffer for the overlay.
type hud struct {
	tex   *gfx.Texture
	vao   *gfx.VAO
	vbo   *gfx.Buffer
	cols  map[rune]int // rune -> atlas column
	cap   int          // quads the current allocation holds
	verts []float32
}

// initHUD rasterises the font into a single-channel atlas.
func (r *Renderer) initHUD() error {
	width := cellW * atlasCols
	pix := make([]byte, width*cellH)
	cols := make(map[rune]int, len(font5x7))
	col := 0
	// Iterate in a stable order so the atlas is byte-identical run to run.
	for rr := rune(32); rr < atlasCols; rr++ {
		rows, ok := font5x7[rr]
		if !ok {
			continue
		}
		cols[rr] = col
		x0 := col * cellW
		for y := 0; y < glyphH; y++ {
			row := rows[y*(glyphW+1) : y*(glyphW+1)+glyphW]
			for x := 0; x < glyphW; x++ {
				if row[x] == '#' {
					pix[(y+1)*width+x0+x] = 255
				}
			}
		}
		col++
	}
	r.hudTex = gfx.NewTexture2D(gfx.TexSpec{
		Width: int32(width), Height: cellH,
		Internal: gl.R8, Format: gl.RED, Type: gl.UNSIGNED_BYTE,
		Filter: gl.NEAREST,
		Data:   unsafe.Pointer(&pix[0]),
	})
	h := &hud{tex: r.hudTex, cols: cols, cap: hudQuads}
	h.verts = make([]float32, 0, hudQuads*12)
	h.vao = gfx.NewVAO()
	h.vbo = gfx.NewBuffer(gl.ARRAY_BUFFER, make([]float32, hudQuads*12), gl.DYNAMIC_DRAW)
	// The buffer has to be bound while the attributes are declared: a vertex
	// attribute pointer is recorded against whatever ARRAY_BUFFER is current.
	h.vbo.Bind()
	const stride = 8 * 4
	gfx.SetAttribute(gfx.Attribute{Location: 0, Size: 2, Type: gl.FLOAT, Stride: stride})
	gfx.SetAttribute(gfx.Attribute{Location: 1, Size: 2, Type: gl.FLOAT, Stride: stride, Offset: 8})
	gfx.SetAttribute(gfx.Attribute{Location: 2, Size: 4, Type: gl.FLOAT, Stride: stride, Offset: 16})
	gfx.Unbind()
	r.hud = h
	return nil
}

func (r *Renderer) closeHUD() {
	if r.hud == nil {
		return
	}
	r.hud.vbo.Delete()
	r.hud.vao.Delete()
	r.hud.tex.Delete()
	r.hud = nil
	r.hudTex = nil
}

// beginOverlay starts a batch of overlay text.
func (r *Renderer) beginOverlay() *hud {
	if r.hud == nil {
		return nil
	}
	r.hud.verts = r.hud.verts[:0]
	return r.hud
}

// Text draws s with its top-left at (x, y) in pixels from the top left of the
// window and returns the pen position after the last glyph. Lower case is folded
// to upper case; a rune the font does not carry still advances the pen, so a
// missing glyph reads as a gap rather than a shifted line.
func (h *hud) Text(x, y float32, s string, rgb [3]float32, a float32) float32 {
	if h == nil {
		return x
	}
	invW := 1 / float32(cellW*atlasCols)
	for _, rr := range strings.ToUpper(s) {
		c, ok := h.cols[rr]
		if !ok {
			x += glyphAdv
			continue
		}
		u0 := float32(c*cellW) * invW
		u1 := float32(c*cellW+glyphW) * invW
		v0 := float32(1) / float32(cellH)
		v1 := float32(1+glyphH) / float32(cellH)
		h.quad(x, y, x+glyphW*textScale, y+glyphH*textScale, u0, v0, u1, v1,
			rgb[0], rgb[1], rgb[2], a)
		x += glyphAdv
	}
	return x
}

func (h *hud) quad(x0, y0, x1, y1, u0, v0, u1, v1, r, g, b, a float32) {
	if len(h.verts)+48 > cap(h.verts)*4 {
		return // overlay longer than the allocation; drop the tail rather than crash
	}
	push := func(px, py, u, v float32) {
		h.verts = append(h.verts, px, py, u, v, r, g, b, a)
	}
	push(x0, y0, u0, v0)
	push(x1, y0, u1, v0)
	push(x1, y1, u1, v1)
	push(x0, y0, u0, v0)
	push(x1, y1, u1, v1)
	push(x0, y1, u0, v1)
}

// drawOverlay renders the batch. The hud program must already be bound with the
// screen size in uScreen.
func (h *hud) drawOverlay() {
	if h == nil || len(h.verts) == 0 {
		return
	}
	h.vbo.SubData(0, h.verts)
	h.tex.Bind(0)
	h.vao.Bind()
	gfx.Disable(gl.DEPTH_TEST)
	gfx.DepthMask(false)
	gfx.Enable(gl.BLEND)
	gfx.BlendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
	gfx.DrawTriangleArrays(0, int32(len(h.verts)/8))
	gfx.Disable(gl.BLEND)
	gfx.DepthMask(true)
	gfx.Enable(gl.DEPTH_TEST)
}
