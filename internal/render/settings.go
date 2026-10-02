package render

// Settings holds every visual constant in the scene in one place. Nothing in the
// renderer hard-codes a number that a viewer can see; each of these was chosen
// against a rendered frame, and the comment says what it controls.
type Settings struct {
	// --- Cube geometry, in grid units (one grid step = 1.0) -------------------
	CubieHalf    float32 // half extent of a cubie; the gap is 1 - 2*CubieHalf
	BevelRadius  float32 // rounded edge/corner radius; the main lighting lever
	CornerDetail int     // subdivision levels for the spherical corners
	StickerHalf  float32 // half width of a sticker
	StickerRound float32 // corner radius of a sticker
	StickerThick float32 // how far a sticker stands proud of the plastic
	StickerChamf float32 // horizontal run of the sticker's chamfered rim
	StickerSegs  int     // segments per rounded sticker corner

	// --- Materials -----------------------------------------------------------
	PlasticRoughness float32
	PlasticF0        float32
	StickerRoughness float32
	StickerF0        float32
	StickerCoat      float32 // strength of the clear-coat lobe on stickers

	// --- Lighting ------------------------------------------------------------
	KeyDir       [3]float32 // toward the light; the brief asks for roughly (+.3,+.5,+.8)
	KeyColor     [3]float32
	FillDir      [3]float32
	FillColor    [3]float32
	RimDir       [3]float32
	RimColor     [3]float32
	EnvIntensity float32

	// --- Shadow map ----------------------------------------------------------
	ShadowSize       int32
	ShadowRadius     float32 // ortho half-extent around the cube
	ShadowNormalBias float32 // in shadow texels
	ShadowSoftness   float32 // PCF radius in shadow texels

	// --- Ambient occlusion ---------------------------------------------------
	AOEnabled   bool
	AORadius    float32 // world units
	AOBias      float32
	AOPower     float32
	AOIntensity float32
	AOKernel    int

	// --- Floor ---------------------------------------------------------------
	FloorY       float32
	FloorRadius  float32
	FloorFadeIn  float32
	FloorFadeOut float32
	FloorAlbedo  float32
	FloorRough   float32

	// FloorEnvIntensity scales the studio reflection in the floor separately from
	// the cube's. They want different values: the plastic needs the softbox to read
	// as a highlight, and the floor needs it suppressed or it becomes the brightest
	// thing on screen.
	FloorEnvIntensity float32

	// --- Camera --------------------------------------------------------------
	FOVDeg    float32
	Fill      float32 // how much of the frame height the cube should occupy
	Near, Far float32
	// Subtle camera breathing, so the shot is not locked to a tripod.
	BreathAmp  float32
	BreathRate float32

	// --- Post ----------------------------------------------------------------
	MSAA           int32
	BloomTiers     int
	BloomThreshold float32
	BloomKnee      float32
	BloomStrength  float32
	Exposure       float32
	Saturation     float32
	White          float32 // highlight pivot for the extended-Reinhard curve
	Contrast       float32
	Vignette       float32
	Dither         float32
	ACES           bool // true selects the ACES curve instead of Reinhard-filmic

	// DebugShade replaces the cube's shading with a diagnostic buffer:
	// 0 shaded, 1 ambient occlusion, 2 view normals, 3 key-light shadow factor,
	// 4 roughness. It is how a black cube gets diagnosed instead of guessed at.
	DebugShade int

	// NoCull disables back-face culling for the cube. It exists as a diagnostic
	// because a missing mesh and an inverted winding look identical in a frame.
	NoCull bool

	// NoShadow binds a 1x1 fully-lit depth map instead of the real one.
	NoShadow bool

	// SingleDraw draws the body mesh once without instancing (diagnostic).
	SingleDraw bool

	// NoDepth disables the depth test for the cube draw (diagnostic).
	NoDepth bool

	// PassTimes brackets every pass in glFinish and prints the mean cost of each
	// once a second. It costs the frame its pipelining, so it is off by default.
	PassTimes bool

	// EnvMapSize is the reflection cubemap's edge length.
	EnvMapSize int32
}

// DefaultSettings is the tuned configuration.
func DefaultSettings() Settings {
	s := Settings{
		CubieHalf:    0.485,
		BevelRadius:  0.055,
		CornerDetail: 3,
		StickerHalf:  0.375,
		StickerRound: 0.1,
		StickerThick: 0.012,
		StickerChamf: 0.005,
		StickerSegs:  5,

		PlasticRoughness: 0.42,
		PlasticF0:        0.045,
		StickerRoughness: 0.24,
		StickerF0:        0.05,
		StickerCoat:      0.55,

		// The brief's (0.3, 0.5, 0.8) sits within about ten degrees of the camera.
		// That exposes the cube beautifully but sends the whole cast shadow
		// straight behind it, where nothing can see it. Same height above the
		// table, swung out to a wide three-quarter: the key still rakes the three
		// visible faces, and the shadow now falls across the floor on the left of
		// the frame instead of disappearing behind the hero.
		KeyDir:   norm3(0.60, 0.60, 0.52),
		KeyColor: [3]float32{2.65, 2.55, 2.38},
		// The fill comes in from the side the key has left. Without it the face
		// turned away from a wide three-quarter key falls to nothing and a red
		// sticker reads as a black one.
		FillDir:   norm3(-0.72, 0.2, 0.5),
		FillColor: [3]float32{0.58, 0.66, 0.90},
		RimDir:    norm3(-0.25, 0.55, -0.85),
		RimColor:  [3]float32{1.35, 1.50, 1.85},
		// The studio bounce is what keeps a face turned away from the key readable.
		// A directional key cannot light an opposite-facing surface; the irradiance
		// from the softboxes can, and it does it with the right colour and falloff.
		EnvIntensity: 1.6,

		ShadowSize:       2048,
		ShadowRadius:     5.4,
		ShadowNormalBias: 2.2,
		ShadowSoftness:   6.0,

		AOEnabled:   true,
		AORadius:    0.13,
		AOBias:      0.015,
		AOPower:     1.25,
		AOIntensity: 0.8,
		AOKernel:    16,

		FloorY:      -1.62,
		FloorRadius: 18,
		FloorFadeIn: 2.0,
		// Must stay inside FloorRadius: the fade is what hides the disc's far
		// edge, and a fade that is still 0.10 where the geometry stops leaves a straight
		// lit/unlit seam across the floor.
		FloorFadeOut:      17.0,
		FloorAlbedo:       0.012,
		FloorRough:        0.62,
		FloorEnvIntensity: 0.20,

		FOVDeg:     28,
		Fill:       0.72,
		Near:       0.5,
		Far:        80,
		BreathAmp:  0.006,
		BreathRate: 0.055,

		MSAA:           8,
		BloomTiers:     4,
		BloomThreshold: 2.6,
		BloomKnee:      0.6,
		BloomStrength:  0.05,
		Exposure:       1.0,
		Saturation:     1.06,
		// The pivot is what puts a fully lit white sticker at ~240 rather than at
		// 220: the curve is c(1+c/White^2)/(1+c), so lowering it lifts the whole
		// upper half without touching the backdrop, which sits far below it.
		// The curve maps this radiance to exactly 1.0, so it is the ceiling.
		// Measured, the white face delivers 2.66 linear at the start of the run, which
		// a ceiling of 2.4 turned into a flat 255 across the whole face.
		White:    3.2,
		Contrast: 0.12,
		Vignette: 0.16,
		Dither:   1.0,
		ACES:     false,

		EnvMapSize: 256,
	}
	return s
}

func norm3(x, y, z float32) [3]float32 {
	l := float32(0)
	for _, v := range [3]float32{x, y, z} {
		l += v * v
	}
	if l == 0 {
		return [3]float32{}
	}
	return [3]float32{x / l, y / l, z / l}
}
