package cube

import "github.com/suaro/gorubik/internal/math3d"

// LayerTurn is an in-flight, not yet committed face turn.
type LayerTurn struct {
	Move Move
	// Angle is the fraction of the turn already animated, in radians, already
	// carrying the sign of Move.Angle(). At |Angle| == pi/2 the move is due to be
	// committed and the pose is continuous across that boundary.
	Angle float32
}

// BodyPose is one cubie's world transform for a frame.
type BodyPose struct {
	Matrix math3d.Mat4
	ID     int
	Kind   Kind
}

// StickerPose is one sticker's world transform. Stickers are posed relative to
// their own cubie, which is what makes them travel with it.
type StickerPose struct {
	Matrix math3d.Mat4
	Color  Color
	Cubie  int
	// Local is the cubie-local face the sticker is glued to, and Grid the grid
	// direction that face currently points along. Only the debug overlay uses them.
	Local Dir
	Grid  Dir
}

// Frame is a complete pose: 26 bodies and 54 stickers, in world space.
type Frame struct {
	Bodies   []BodyPose
	Stickers []StickerPose
}

// InLayer reports whether a cubie takes part in the given turn.
func (t *LayerTurn) InLayer(cb *Cubie) bool {
	return t != nil && cb.Pos[t.Move.Axis()] == t.Move.Layer()
}

// Pose composes the transforms for one frame.
//
// The order is exactly the one the specification asks for:
//
//	cubie local transform          T(pos) * Rot
//	  then animated layer turn     RotAxis(axis, angle) * (...)   [layer members only]
//	  then global whole-cube spin  spin * (...)
//
// view and projection are applied by the renderer.
//
// localStickers holds, per local face, the transform from sticker model space
// onto that face of the cubie (offset outward so the sticker floats just above
// the plastic).
func (c *Cube) Pose(spin math3d.Mat4, turn *LayerTurn, localStickers [6]math3d.Mat4) Frame {
	f := Frame{
		Bodies:   make([]BodyPose, 0, len(c.Cubies)),
		Stickers: make([]StickerPose, 0, 54),
	}
	for _, cb := range c.Cubies {
		local := math3d.Translation(cb.Pos.ToVec3()).Mul(cb.Rot.Mat4())
		if turn.InLayer(cb) {
			local = math3d.RotAxis(turn.Move.Axis(), turn.Angle).Mul(local)
		}
		world := spin.Mul(local)
		f.Bodies = append(f.Bodies, BodyPose{Matrix: world, ID: cb.ID, Kind: cb.Kind})
		for _, localDir := range cb.LocalStickerDirs() {
			f.Stickers = append(f.Stickers, StickerPose{
				Matrix: world.Mul(localStickers[localDir]),
				Color:  cb.Stickers[localDir],
				Cubie:  cb.ID,
				Local:  localDir,
				Grid:   cb.GridDirOfSticker(localDir),
			})
		}
	}
	return f
}

// AnimatedLayerCount is the number of cubies the given turn will move, exposed so
// the debug overlay can assert it is nine.
func (c *Cube) AnimatedLayerCount(t *LayerTurn) int {
	if t == nil {
		return 0
	}
	return len(c.Layer(t.Move.Axis(), t.Move.Layer()))
}
