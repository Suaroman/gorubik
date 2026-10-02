// Package gfx is a thin, checked layer over go-gl: shader programs with readable
// compile errors, cached uniform locations, and owning wrappers for buffers,
// textures and framebuffers. It knows nothing about cubes or lighting; everything
// scene-specific lives in internal/render.
package gfx

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.3-core/gl"
)

// Init initialises the GL bindings and returns the driver's version and renderer
// strings, which the app prints so a screenshot can be traced back to hardware.
func Init() (version, renderer string, err error) {
	if err := gl.Init(); err != nil {
		return "", "", err
	}
	return str(gl.GetString(gl.VERSION)), str(gl.GetString(gl.RENDERER)), nil
}

func str(p *uint8) string {
	if p == nil {
		return ""
	}
	return gl.GoStr(p)
}

// ---------------------------------------------------------------------------
// Programs
// ---------------------------------------------------------------------------

// Program is a linked shader program with a uniform location cache.
type Program struct {
	id       uint32
	name     string
	uniforms map[string]int32
}

// NewProgram compiles and links a vertex/fragment pair. Shader source is expected
// to be GLSL terminated with a NUL byte, which go:embed + gl.StringToBytes provides.
func NewProgram(name, vertSrc, fragSrc string) (*Program, error) {
	vs, err := compileShader(gl.VERTEX_SHADER, name+".vert", vertSrc)
	if err != nil {
		return nil, err
	}
	defer gl.DeleteShader(vs)
	fs, err := compileShader(gl.FRAGMENT_SHADER, name+".frag", fragSrc)
	if err != nil {
		return nil, err
	}
	defer gl.DeleteShader(fs)

	id := gl.CreateProgram()
	gl.AttachShader(id, vs)
	gl.AttachShader(id, fs)
	gl.LinkProgram(id)
	var ok int32
	gl.GetProgramiv(id, gl.LINK_STATUS, &ok)
	if ok == 0 {
		gl.DeleteProgram(id)
		return nil, fmt.Errorf("link %s: %s", name, programLog(id))
	}
	return &Program{id: id, name: name, uniforms: map[string]int32{}}, nil
}

func compileShader(kind uint32, name, src string) (uint32, error) {
	id := gl.CreateShader(kind)
	// gl.Strs does not add terminators, so the source is passed explicitly
	// NUL-terminated.
	csrc, free := gl.Strs(src + "\x00")
	defer free()
	gl.ShaderSource(id, 1, csrc, nil)
	gl.CompileShader(id)
	var ok int32
	gl.GetShaderiv(id, gl.COMPILE_STATUS, &ok)
	if ok == 0 {
		log := shaderLog(id)
		gl.DeleteShader(id)
		// Driver messages are the only way to see what a shader did wrong and are
		// unreadable without the source lines they refer to, so echo them.
		return 0, fmt.Errorf("compile %s:\n%s%s", name, log, annotate(src, log))
	}
	return id, nil
}

func shaderLog(id uint32) string {
	var n int32
	gl.GetShaderiv(id, gl.INFO_LOG_LENGTH, &n)
	if n <= 1 {
		return "(no log)\n"
	}
	buf := make([]uint8, n)
	var written int32
	gl.GetShaderInfoLog(id, n, &written, &buf[0])
	return strings.TrimSpace(string(buf[:written])) + "\n"
}

func programLog(id uint32) string {
	var n int32
	gl.GetProgramiv(id, gl.INFO_LOG_LENGTH, &n)
	if n <= 1 {
		return "(no log)"
	}
	buf := make([]uint8, n)
	var written int32
	gl.GetProgramInfoLog(id, n, &written, &buf[0])
	return strings.TrimSpace(string(buf[:written]))
}

// annotate echoes the source lines a driver error points at. NVIDIA reports
// positions as "0:41", AMD as "ERROR: 41:..." and Intel as "ERROR: 0:41: ..."
// (sometimes with a caret line after), so scan for any "<n>:<n>" prefix and take
// the last number before the message text.
func annotate(src, log string) string {
	lines := strings.Split(src, "\n")
	seen := map[int]bool{}
	var out []string
	for _, l := range strings.Split(log, "\n") {
		ln := parseLogLine(l)
		if ln <= 0 || ln > len(lines) || seen[ln] {
			continue
		}
		seen[ln] = true
		lo, hi := max(1, ln-4), min(len(lines), ln+2)
		for k := lo; k <= hi; k++ {
			mark := "    "
			if k == ln {
				mark = ">>> "
			}
			out = append(out, fmt.Sprintf("%s%3d | %s", mark, k, lines[k-1]))
		}
	}
	if len(out) == 0 {
		return ""
	}
	return "source:\n" + strings.Join(out, "\n") + "\n"
}

// parseLogLine pulls the source line number out of a driver diagnostic.
func parseLogLine(l string) int {
	i := strings.Index(l, ":")
	if i < 0 {
		return 0
	}
	rest := l[i+1:]
	j := strings.Index(rest, ":")
	if j <= 0 {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(rest[:j], "%d", &n); err != nil {
		return 0
	}
	return n
}

// Delete releases the program.
func (p *Program) Delete() { gl.DeleteProgram(p.id) }

// Use binds the program for subsequent draws.
func (p *Program) Use() { gl.UseProgram(p.id) }

// ID exposes the raw program handle for the rare direct call.
func (p *Program) ID() uint32 { return p.id }

func (p *Program) loc(name string) int32 {
	if l, ok := p.uniforms[name]; ok {
		return l
	}
	l := gl.GetUniformLocation(p.id, gl.Str(name+"\x00"))
	p.uniforms[name] = l
	return l
}

// Loc reports the resolved uniform location, or -1 when the program has no such
// uniform. A -1 makes every setter a silent no-op, which is why the diagnostics
// print it rather than trusting that a name is spelled right.
func (p *Program) Loc(name string) int32 { return p.loc(name) }

// SetInt sets a sampler or integer uniform.
func (p *Program) SetInt(name string, v int32) { gl.Uniform1i(p.loc(name), v) }

// SetFloat sets a float uniform.
func (p *Program) SetFloat(name string, v float32) { gl.Uniform1f(p.loc(name), v) }

// SetVec2 sets a vec2 uniform.
func (p *Program) SetVec2(name string, a, b float32) { gl.Uniform2f(p.loc(name), a, b) }

// SetVec3 sets a vec3 uniform.
func (p *Program) SetVec3(name string, v [3]float32) {
	gl.Uniform3f(p.loc(name), v[0], v[1], v[2])
}

// SetVec4 sets a vec4 uniform.
func (p *Program) SetVec4(name string, v [4]float32) {
	gl.Uniform4f(p.loc(name), v[0], v[1], v[2], v[3])
}

// SetMat4 sets a mat4 uniform from 16 column-major floats. A slice keeps this
// package independent of the caller's matrix type; pass m[:] .
func (p *Program) SetMat4(name string, m []float32) {
	gl.UniformMatrix4fv(p.loc(name), 1, false, &m[0])
}

// SetMat3 sets a mat3 uniform from a column-major 9-element block.
func (p *Program) SetMat3(name string, m *[9]float32) {
	gl.UniformMatrix3fv(p.loc(name), 1, false, &m[0])
}

// SetMat4Array uploads a batch of matrices (used for the instance buffers).
func (p *Program) SetMat4Array(name string, m []float32) {
	gl.UniformMatrix4fv(p.loc(name), int32(len(m)/16), false, &m[0])
}

// SetVec3Array uploads a vec3 array, such as the screen-space AO sampling kernel.
func (p *Program) SetVec3Array(name string, v []float32) {
	if len(v) == 0 {
		return
	}
	gl.Uniform3fv(p.loc(name), int32(len(v)/3), &v[0])
}

// ---------------------------------------------------------------------------
// Buffers
// ---------------------------------------------------------------------------

// Buffer is an owning VBO/IBO wrapper.
type Buffer struct {
	id     uint32
	target uint32
}

// NewBuffer uploads data once. data may be any slice of fixed-size values.
func NewBuffer(target uint32, data []float32, usage uint32) *Buffer {
	b := &Buffer{target: target}
	gl.GenBuffers(1, &b.id)
	gl.BindBuffer(target, b.id)
	gl.BufferData(target, len(data)*4, unsafe.Pointer(&data[0]), usage)
	gl.BindBuffer(target, 0)
	return b
}

// NewBufferU uploads an index array.
func NewBufferU(target uint32, data []uint32, usage uint32) *Buffer {
	b := &Buffer{target: target}
	gl.GenBuffers(1, &b.id)
	gl.BindBuffer(target, b.id)
	gl.BufferData(target, len(data)*4, unsafe.Pointer(&data[0]), usage)
	gl.BindBuffer(target, 0)
	return b
}

// Bind makes the buffer current on its target.
func (b *Buffer) Bind() { gl.BindBuffer(b.target, b.id) }

// SubData replaces part of the buffer's store.
func (b *Buffer) SubData(offset int, data []float32) {
	gl.BindBuffer(b.target, b.id)
	gl.BufferSubData(b.target, offset*4, len(data)*4, unsafe.Pointer(&data[0]))
}

// ReadBack copies part of the buffer's store back to the CPU. It exists for the
// diagnostics: the only way to settle "did the upload land" without guessing.
func (b *Buffer) ReadBack(offset int, dst []float32) {
	gl.BindBuffer(b.target, b.id)
	gl.GetBufferSubData(b.target, offset*4, len(dst)*4, unsafe.Pointer(&dst[0]))
}

// Size reports the buffer's store in bytes.
func (b *Buffer) Size() int {
	gl.BindBuffer(b.target, b.id)
	var n int32
	gl.GetBufferParameteriv(b.target, gl.BUFFER_SIZE, &n)
	return int(n)
}

// AttributeBuffer reports which buffer the given attribute of the given VAO reads
// from, and Delete releases the buffer.
func AttributeBuffer(vao *VAO, index int32) uint32 {
	vao.Bind()
	var n int32
	gl.GetVertexAttribiv(uint32(index), gl.VERTEX_ATTRIB_ARRAY_BUFFER_BINDING, &n)
	return uint32(n)
}

// Delete releases the buffer.
func (b *Buffer) Delete() {
	if b != nil && b.id != 0 {
		gl.DeleteBuffers(1, &b.id)
		b.id = 0
	}
}

// ---------------------------------------------------------------------------
// Vertex arrays
// ---------------------------------------------------------------------------

// VAO is an owning vertex-array wrapper.
type VAO struct{ id uint32 }

// NewVAO creates and returns a bound vertex array.
func NewVAO() *VAO {
	v := &VAO{}
	gl.GenVertexArrays(1, &v.id)
	gl.BindVertexArray(v.id)
	return v
}

// Bind makes the array current.
func (v *VAO) Bind() { gl.BindVertexArray(v.id) }

// Unbind releases the vertex array binding.
func Unbind() { gl.BindVertexArray(0) }

// Delete releases the array.
func (v *VAO) Delete() {
	if v != nil && v.id != 0 {
		gl.DeleteVertexArrays(1, &v.id)
		v.id = 0
	}
}

// Attribute describes one vertex attribute.
type Attribute struct {
	Location   int32
	Size       int32
	Type       uint32
	Stride     int32
	Offset     int32
	Divisor    int32 // non-zero for per-instance data
	Normalized bool
}

// SetAttribute declares an attribute on the bound buffer.
func SetAttribute(a Attribute) {
	gl.EnableVertexAttribArray(uint32(a.Location))
	gl.VertexAttribPointerWithOffset(uint32(a.Location), a.Size, a.Type, a.Normalized, a.Stride, uintptr(a.Offset))
	if a.Divisor != 0 {
		gl.VertexAttribDivisor(uint32(a.Location), uint32(a.Divisor))
	}
}

// ---------------------------------------------------------------------------
// Textures
// ---------------------------------------------------------------------------

// Texture is an owning 2D texture wrapper.
type Texture struct {
	id         uint32
	target     uint32
	width      int32
	height     int32
	format     uint32
	miplevels  int32
	ownsMemory bool
}

// TexSpec describes a texture to allocate.
type TexSpec struct {
	Width, Height int32
	Internal      uint32
	Format        uint32
	Type          uint32
	Mips          int32 // 0 means a single level
	Filter        uint32
	Wrap          uint32
	Data          unsafe.Pointer
	Comparison    bool // for shadow maps
}

// NewTexture2D allocates a 2D texture (or a depth texture when Type is
// gl.DEPTH_COMPONENT).
func NewTexture2D(spec TexSpec) *Texture {
	if spec.Filter == 0 {
		spec.Filter = gl.LINEAR
	}
	if spec.Wrap == 0 {
		spec.Wrap = gl.CLAMP_TO_EDGE
	}
	if spec.Mips <= 0 {
		spec.Mips = 1
	}
	t := &Texture{target: gl.TEXTURE_2D, width: spec.Width, height: spec.Height, format: spec.Internal, ownsMemory: true}
	gl.GenTextures(1, &t.id)
	gl.BindTexture(gl.TEXTURE_2D, t.id)
	gl.TexImage2D(gl.TEXTURE_2D, 0, int32(spec.Internal), spec.Width, spec.Height, 0, spec.Format, spec.Type, spec.Data)
	if spec.Mips > 1 {
		for l := int32(1); l < spec.Mips; l++ {
			gl.TexImage2D(gl.TEXTURE_2D, l, int32(spec.Internal), spec.Width>>l, spec.Height>>l, 0, spec.Format, spec.Type, nil)
		}
	}
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, int32(spec.Filter))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, int32(spec.Filter))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, int32(spec.Wrap))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, int32(spec.Wrap))
	if spec.Comparison {
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_MODE, gl.COMPARE_REF_TO_TEXTURE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_COMPARE_FUNC, gl.LEQUAL)
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)
	t.miplevels = spec.Mips
	return t
}

// ID returns the texture handle.
func (t *Texture) ID() uint32 { return t.id }

// Target returns the texture's GL target (TEXTURE_2D or TEXTURE_CUBE_MAP).
func (t *Texture) Target() uint32 { return t.target }

// Width is the texture width in texels.
func (t *Texture) Width() int32 { return t.width }

// Height is the texture height in texels.
func (t *Texture) Height() int32 { return t.height }

// Bind attaches the texture to the given unit and makes it current.
func (t *Texture) Bind(unit int32) {
	gl.ActiveTexture(gl.TEXTURE0 + uint32(unit))
	gl.BindTexture(t.target, t.id)
}

// SetFilter changes the filtering mode (used to switch a resolved texture between
// point and linear sampling).
func (t *Texture) SetFilter(f uint32) {
	gl.BindTexture(t.target, t.id)
	gl.TexParameteri(t.target, gl.TEXTURE_MIN_FILTER, int32(f))
	gl.TexParameteri(t.target, gl.TEXTURE_MAG_FILTER, int32(f))
	gl.BindTexture(t.target, 0)
}

// GenerateMips builds the full mip chain.
func (t *Texture) GenerateMips() {
	gl.BindTexture(t.target, t.id)
	gl.GenerateMipmap(t.target)
	gl.BindTexture(t.target, 0)
}

// Delete releases the texture.
func (t *Texture) Delete() {
	if t != nil && t.id != 0 && t.ownsMemory {
		gl.DeleteTextures(1, &t.id)
		t.id = 0
	}
}

// CubeMap is an owning cube texture wrapper.
type CubeMap struct {
	id     uint32
	size   int32
	mips   int32
	format uint32
}

// NewCubeMap allocates a cube texture with the given mip count.
func NewCubeMap(size int32, mips int32, internal, format, xtype uint32) *CubeMap {
	if mips <= 0 {
		mips = 1
		for s := size; s > 1; s >>= 1 {
			mips++
		}
	}
	c := &CubeMap{id: genTexture(), size: size, mips: mips, format: internal}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, c.id)
	for l := int32(0); l < mips; l++ {
		for f := uint32(0); f < 6; f++ {
			gl.TexImage2D(gl.TEXTURE_CUBE_MAP_POSITIVE_X+f, l, int32(internal),
				size>>l, size>>l, 0, format, xtype, nil)
		}
	}
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
	return c
}

// ID returns the texture handle.
func (c *CubeMap) ID() uint32 { return c.id }

// Size is one edge length in texels.
func (c *CubeMap) Size() int32 { return c.size }

// Mips is the number of mip levels.
func (c *CubeMap) Mips() int32 { return c.mips }

// Bind attaches the cube map to a unit.
func (c *CubeMap) Bind(unit int32) {
	gl.ActiveTexture(gl.TEXTURE0 + uint32(unit))
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, c.id)
}

// Delete releases the cube map.
func (c *CubeMap) Delete() {
	if c != nil && c.id != 0 {
		gl.DeleteTextures(1, &c.id)
		c.id = 0
	}
}

func genTexture() uint32 {
	var id uint32
	gl.GenTextures(1, &id)
	return id
}

// ---------------------------------------------------------------------------
// Renderbuffers and framebuffers
// ---------------------------------------------------------------------------

// Renderbuffer is an owning renderbuffer wrapper.
type Renderbuffer struct {
	id uint32
}

// NewMultisampleRenderbuffer allocates a multisampled renderbuffer.
func NewMultisampleRenderbuffer(samples int32, internal uint32, w, h int32) *Renderbuffer {
	r := &Renderbuffer{}
	gl.GenRenderbuffers(1, &r.id)
	gl.BindRenderbuffer(gl.RENDERBUFFER, r.id)
	gl.RenderbufferStorageMultisample(gl.RENDERBUFFER, samples, internal, w, h)
	gl.BindRenderbuffer(gl.RENDERBUFFER, 0)
	return r
}

// ID returns the renderbuffer handle.
func (r *Renderbuffer) ID() uint32 { return r.id }

// Delete releases the renderbuffer.
func (r *Renderbuffer) Delete() {
	if r != nil && r.id != 0 {
		gl.DeleteRenderbuffers(1, &r.id)
		r.id = 0
	}
}

// Framebuffer is an owning framebuffer wrapper.
type Framebuffer struct {
	id uint32
	// draw is the value to install in DRAW_BUFFER0 on every bind. Zero means the
	// caller never chose one, which resolves to COLOR_ATTACHMENT0.
	draw uint32
}

// NewFramebuffer creates an unbound framebuffer.
func NewFramebuffer() *Framebuffer {
	f := &Framebuffer{}
	gl.GenFramebuffers(1, &f.id)
	return f
}

// Bind makes the framebuffer current for reads and draws.
func (f *Framebuffer) Bind() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, f.id)
	// Say which buffer to write to instead of relying on the default. DRAW_BUFFER0
	// starts as GL_BACK, which names no attachment on a framebuffer object, and
	// drivers disagree about whether that means COLOR_ATTACHMENT0 or an incomplete
	// framebuffer. The value is per-target state held on the struct so that binding
	// one target cannot silently rewrite another: the depth-only shadow map needs
	// NONE, and setting it once at build time was not enough because every pass
	// binds and re-bound it back to a colour attachment, which made the shadow map
	// incomplete and the whole frame unlit-by-shadow.
	gl.DrawBuffer(f.drawBuffer())
}

// drawBuffer resolves the stored selection, defaulting a target nobody configured
// to the first colour attachment.
func (f *Framebuffer) drawBuffer() uint32 {
	if f.draw == 0 {
		return gl.COLOR_ATTACHMENT0
	}
	return f.draw
}

// DrawNone declares that the target has no colour output. A depth-only pass needs
// this; leaving the draw buffer on a colour attachment that does not exist makes
// the framebuffer incomplete and every draw in it a no-op.
func (f *Framebuffer) DrawNone() { f.draw = gl.NONE }

// ID returns the framebuffer handle.
func (f *Framebuffer) ID() uint32 { return f.id }

// AttachTexture binds a texture level as a color or depth attachment.
func (f *Framebuffer) AttachTexture(attachment uint32, t interface{ ID() uint32 }, target uint32) {
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, attachment, target, t.ID(), 0)
}

// AttachRenderbuffer binds a renderbuffer as an attachment.
func (f *Framebuffer) AttachRenderbuffer(attachment uint32, r *Renderbuffer) {
	gl.FramebufferRenderbuffer(gl.FRAMEBUFFER, attachment, gl.RENDERBUFFER, r.id)
}

// Check validates the completeness of the currently bound framebuffer.
func (f *Framebuffer) Check() error {
	s := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	if s != gl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer %d incomplete (status 0x%x)", f.id, s)
	}
	return nil
}

// Delete releases the framebuffer.
func (f *Framebuffer) Delete() {
	if f != nil && f.id != 0 {
		gl.DeleteFramebuffers(1, &f.id)
		f.id = 0
	}
}

// ---------------------------------------------------------------------------
// State helpers
// ---------------------------------------------------------------------------

// Viewport sets the raster area.
func Viewport(w, h int32) { gl.Viewport(0, 0, w, h) }

// ClearColor sets the clear value.
func ClearColor(r, g, b, a float32) { gl.ClearColor(r, g, b, a) }

// ClearColorBuffer clears the current draw buffer with the current clear colour.
func ClearColorBuffer() { gl.Clear(gl.COLOR_BUFFER_BIT) }

// ClearDepthBuffer clears depth.
func ClearDepthBuffer() { gl.Clear(gl.DEPTH_BUFFER_BIT) }

// DrawTriangleArrays issues non-indexed draws.
func DrawTriangleArrays(first, count int32) { gl.DrawArrays(gl.TRIANGLES, first, count) }

// DrawIndexed issues an indexed triangle draw.
func DrawIndexed(count int32) { gl.DrawElements(gl.TRIANGLES, count, gl.UNSIGNED_INT, nil) }

// DrawIndexedInstanced issues an indexed instanced draw.
func DrawIndexedInstanced(count, instances int32) {
	gl.DrawElementsInstanced(gl.TRIANGLES, count, gl.UNSIGNED_INT, nil, instances)
}

// DrawArraysInstanced issues a non-indexed instanced draw.
func DrawArraysInstanced(count, instances int32) {
	gl.DrawArraysInstanced(gl.TRIANGLES, 0, count, instances)
}

// Enable turns a GL feature on.
func Enable(cap uint32) { gl.Enable(cap) }

// Disable turns a GL feature off.
func Disable(cap uint32) { gl.Disable(cap) }

// DepthFunc sets the depth comparison.
func DepthFunc(f uint32) { gl.DepthFunc(f) }

// DepthMask enables or disables depth writes.
func DepthMask(on bool) { gl.DepthMask(on) }

// CullFace sets the facing direction to cull.
func CullFace(f uint32) { gl.CullFace(f) }

// FrontFace sets which winding is considered front-facing.
func FrontFace(f uint32) { gl.FrontFace(f) }

// BlendFunc sets the colour blend factors.
func BlendFunc(src, dst uint32) { gl.BlendFunc(src, dst) }

// ActiveTextureUnit selects a texture unit.
func ActiveTextureUnit(u int32) { gl.ActiveTexture(gl.TEXTURE0 + uint32(u)) }

// BindDefaultFramebuffer routes drawing to the window back buffer.
func BindDefaultFramebuffer(w, h int32) {
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	gl.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.DrawBuffer(gl.BACK)
	Viewport(w, h)
}

// BlitColorAndDepth resolves a multisampled framebuffer into a single-sample one.
// Both must be the same size and have matching formats.
func BlitColorAndDepth(w, h int32) {
	gl.BlitFramebuffer(0, 0, w, h, 0, 0, w, h, uint32(gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT), gl.NEAREST)
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// PollErrors drains the GL error queue and returns the first error seen, tagged
// with the caller's context. Cheap enough to call at pass boundaries while
// developing, and it turns a silent black frame into a sentence.
func PollErrors(ctx string) error {
	var first error
	for i := 0; i < 32; i++ {
		e := gl.GetError()
		if e == gl.NO_ERROR {
			break
		}
		if first == nil {
			first = fmt.Errorf("gl error %s in %s (0x%04x)", errorName(e), ctx, e)
		}
	}
	return first
}

func errorName(e uint32) string {
	switch e {
	case gl.INVALID_ENUM:
		return "INVALID_ENUM"
	case gl.INVALID_VALUE:
		return "INVALID_VALUE"
	case gl.INVALID_OPERATION:
		return "INVALID_OPERATION"
	case gl.INVALID_FRAMEBUFFER_OPERATION:
		return "INVALID_FRAMEBUFFER_OPERATION"
	case gl.OUT_OF_MEMORY:
		return "OUT_OF_MEMORY"
	case gl.STACK_OVERFLOW:
		return "STACK_OVERFLOW"
	case gl.STACK_UNDERFLOW:
		return "STACK_UNDERFLOW"
	default:
		return "UNKNOWN"
	}
}
