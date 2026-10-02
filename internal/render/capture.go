package render

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/go-gl/gl/v4.3-core/gl"

	"github.com/Suaroman/gorubik/internal/gfx"
)

// The capture path renders the scene into an offscreen RGBA8 target and writes a
// PNG. It exists so that "look at the picture" is a repeatable operation: the same
// timeline moment produces the same pixels, and nothing depends on grabbing the
// screen at the right instant on a machine that may be running at 20 fps or 200.

// BeginCapture redirects the final composite into an offscreen target of the given
// size. Call EndCapture to go back to the window.
func (r *Renderer) BeginCapture(w, h int32) error {
	if err := r.Resize(w, h); err != nil {
		return err
	}
	r.dropCapture()
	r.captureTex = gfx.NewTexture2D(gfx.TexSpec{Width: w, Height: h,
		Internal: gl.RGBA8, Format: gl.RGBA, Type: gl.UNSIGNED_BYTE})
	r.captureFB = gfx.NewFramebuffer()
	r.captureFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.captureTex.ID(), 0)
	if err := r.captureFB.Check(); err != nil {
		r.dropCapture()
		return fmt.Errorf("capture target: %w", err)
	}
	gfx.BindDefaultFramebuffer(w, h)
	return nil
}

// EndCapture releases the offscreen target.
func (r *Renderer) EndCapture() { r.dropCapture() }

func (r *Renderer) dropCapture() {
	r.captureFB.Delete()
	r.captureDep.Delete()
	r.captureTex.Delete()
	r.captureFB, r.captureDep, r.captureTex = nil, nil, nil
}

// WriteCapturePNG reads back the offscreen target and writes it as a PNG under dir.
// GL's origin is bottom-left, so the rows are flipped on the way out.
func (r *Renderer) WriteCapturePNG(dir string, t float64) (string, error) {
	if r.captureFB == nil {
		return "", fmt.Errorf("capture target is not active")
	}
	w, h := r.w, r.h
	if err := r.captureFB.Check(); err != nil {
		return "", err
	}
	buf := make([]byte, int(w)*int(h)*4)
	// Read explicitly from the capture target: once anything has bound a
	// READ_FRAMEBUFFER (the probes do), glBindFramebuffer(FRAMEBUFFER, ...) stops
	// steering reads, and the read silently lands on the default framebuffer.
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, r.captureFB.ID())
	gl.ReadPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)

	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	row := int(w) * 4
	for y := 0; y < int(h); y++ {
		src := buf[(int(h)-1-y)*row : (int(h)-y)*row]
		copy(img.Pix[y*img.Stride:y*img.Stride+row], src)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(dir, fmt.Sprintf("gorubik_%06.2fs.png", t))
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		return "", err
	}
	return name, nil
}

// AuditBuffers asks the driver what the cube's vertex arrays actually point at and
// reads back the first bytes of each buffer. Every "the mesh is there but nothing
// draws" bug eventually comes down to one of these three answers.
func (r *Renderer) AuditBuffers() []string {
	out := []string{}
	vb := gfx.AttributeBuffer(r.bodyVAO, 0)
	ib := gfx.AttributeBuffer(r.bodyVAO, 2)
	out = append(out, fmt.Sprintf("body attr0 buffer=%d attr2 buffer=%d", vb, ib))
	head := make([]float32, 12)
	r.bodyVBO.ReadBack(0, head)
	out = append(out, fmt.Sprintf("body vbo bytes=%d head %v", r.bodyVBO.Size(), head))
	inst := make([]float32, 24)
	r.bodyInst.ReadBack(0, inst)
	out = append(out, fmt.Sprintf("body inst bytes=%d head %v", r.bodyInst.Size(), inst))
	out = append(out, fmt.Sprintf("uViewProj loc cube=%d floor=%d prepass=%d depth=%d",
		r.p.cube.Loc("uViewProj"), r.p.floor.Loc("uViewProj"),
		r.p.prepass.Loc("uViewProj"), r.p.depth.Loc("uLightViewProj")))
	out = append(out, fmt.Sprintf("viewProj %v", r.viewProj))
	out = append(out, fmt.Sprintf("uDebugShade loc cube=%d", r.p.cube.Loc("uDebugShade")))
	return out
}

// ProbeHDR reads the resolved HDR buffer at the given normalised points and returns
// the linear radiance there. It is how a missing object gets localised to a pass
// instead of argued about: if the value is non-zero here and zero in the PNG, the
// problem is downstream of this buffer.
func (r *Renderer) ProbeHDR(pts [][2]float32) ([][3]float32, error) {
	// The bloom chain leaves the scratch framebuffer attached to its smallest mip,
	// so re-attach the buffer being probed or the read lands outside it.
	r.postFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.hdrTex.ID(), 0)
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, r.postFB.ID())
	out := make([][3]float32, len(pts))
	for i, p := range pts {
		x := int32(float32(r.w-1) * p[0])
		y := int32(float32(r.h-1) * (1 - p[1])) // GL origin is bottom-left
		var v [4]float32
		gl.ReadPixels(x, y, 1, 1, gl.RGBA, gl.FLOAT, unsafe.Pointer(&v[0]))
		out[i] = [3]float32{v[0], v[1], v[2]}
	}
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	return out, gfx.PollErrors("probe")
}

// ProbeShadow reads the shadow map at the given coordinates so an unexpected
// silhouette on the floor can be traced to the map instead of to speculation.
func (r *Renderer) ProbeShadow(pts [][2]float32) ([]float32, error) {
	r.postFB.Bind()
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.aoTex.ID(), 0)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, r.shadowTex.ID(), 0)
	if err := r.postFB.Check(); err != nil {
		return nil, err
	}
	n := r.cfg.ShadowSize
	out := make([]float32, len(pts))
	for i, p := range pts {
		x := int32(float32(n-1) * p[0])
		y := int32(float32(n-1) * (1 - p[1]))
		var v float32
		gl.ReadPixels(x, y, 1, 1, gl.DEPTH_COMPONENT, gl.FLOAT, unsafe.Pointer(&v))
		out[i] = v
	}
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, 0, 0)
	return out, gfx.PollErrors("probe shadow")
}

// DebugProject maps a world point through the current view-projection and returns
// window pixels plus depth. It answers "is the geometry where I think it is"
// without any guessing.
func (r *Renderer) DebugProject(x, y, z float32) (px, py, dz float32, inside bool) {
	v := r.viewProj.TransformVec4([4]float32{x, y, z, 1})
	if v[3] == 0 {
		return 0, 0, 0, false
	}
	ndc := [3]float32{v[0] / v[3], v[1] / v[3], v[2] / v[3]}
	return (ndc[0]*0.5 + 0.5) * float32(r.w), (1 - (ndc[1]*0.5 + 0.5)) * float32(r.h), ndc[2],
		ndc[0] >= -1 && ndc[0] <= 1 && ndc[1] >= -1 && ndc[1] <= 1 && ndc[2] >= -1 && ndc[2] <= 1
}

// DebugProjectLight maps a world point into shadow-map space and reports the
// texture coordinate and depth the sampler would use, plus the two early-out
// conditions that make a sample unconditionally lit.
func (r *Renderer) DebugProjectLight(x, y, z float32) (u, v, zc float32, outside, farout bool) {
	v4 := r.lightViewProj.TransformVec4([4]float32{x, y, z, 1})
	if v4[3] == 0 {
		return 0, 0, 0, true, true
	}
	ndc := [3]float32{v4[0] / v4[3], v4[1] / v4[3], v4[2] / v4[3]}
	u, v, zc = ndc[0]*0.5+0.5, ndc[1]*0.5+0.5, ndc[2]*0.5+0.5
	outside = u < 0.005 || v < 0.005 || u > 0.995 || v > 0.995
	farout = zc >= 1.0
	return
}

// CameraInfo reports the eye position for the current frame.
func (r *Renderer) CameraInfo() (eye [3]float32, fov, dist float32) {
	return r.camPos, r.cfg.FOVDeg, float32(math.Sqrt(float64(r.camPos[0]*r.camPos[0] + r.camPos[1]*r.camPos[1] + r.camPos[2]*r.camPos[2])))
}

// Sample prints the sRGB value of a pixel so the tuning loop can quote numbers
// instead of adjectives ("the white face reads 238,240,242").
func (r *Renderer) Sample(x, y int32) ([3]uint8, error) {
	if r.captureFB == nil {
		return [3]uint8{}, fmt.Errorf("capture target is not active")
	}
	if x < 0 || y < 0 || x >= r.w || y >= r.h {
		return [3]uint8{}, fmt.Errorf("sample %d,%d outside %dx%d", x, y, r.w, r.h)
	}
	var px [4]uint8
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, r.captureFB.ID())
	gl.ReadPixels(x, r.h-1-y, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	gl.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	return [3]uint8{px[0], px[1], px[2]}, nil
}
