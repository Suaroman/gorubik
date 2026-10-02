package cube

// SpecScramble and SpecSolve are the two sequences the project specification asks
// the animation to perform. They are here rather than in the animation package so
// the simulator's own tests, the timeline and the command line all read the same
// string: a typo in one place becomes a failing test instead of a cube that
// quietly ends in the wrong state.
const (
	// SpecScramble is the 10-turn scramble, played first.
	SpecScramble = "R U F' D L' B R' U' F D'"
	// SpecSolve is its exact inverse, so the cube must finish solved.
	SpecSolve = "D F' U R B' L D' F U' R'"
)
