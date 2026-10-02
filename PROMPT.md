Create a visually exceptional, native desktop Rubik's Cube animation written in **Go**.

This is not a web project. Do not implement this as HTML, JavaScript, CSS, WebGL in a browser, Electron, or a web wrapper. The primary application and rendering logic must be written in Go and run as a native desktop program.

The goal is not merely to make the cube technically correct. The goal is to create something that looks so polished, cinematic, fluid, and visually convincing that someone watching it would be surprised it was produced as a Go graphics application.

Quality matters more than simplicity.

You are free to use external Go packages, native graphics libraries, GPU acceleration, OpenGL, shaders, GLSL, raylib-go, go-gl, GLFW, Ebitengine, or another appropriate Go-compatible graphics stack.

Choose the graphics architecture that will produce the best result.

Do not deliberately simplify the implementation to reduce code size or development effort.

Do not use a crude approximation when a more convincing graphics technique is practical.

---

# PRIMARY OBJECTIVE

Create a premium-quality animated 3D Rubik's Cube that:

* begins perfectly solved
* slowly rotates in space
* executes a specified scramble
* pauses while scrambled
* executes the exact inverse sequence
* returns perfectly to the solved state
* continues rotating indefinitely afterward

The cube must behave like an actual mechanical Rubik's Cube.

Individual layers must physically rotate.

Cubies must retain their identity, orientation, and stickers throughout every move.

The animation must look polished enough to be suitable for a product demonstration or motion-graphics reel.

---

# GRAPHICS QUALITY

Prioritize visual quality aggressively.

The final result should look closer to a polished 3D product visualization than a programming demo.

Use GPU rendering and shaders where beneficial.

Strongly consider implementing:

* perspective-correct 3D rendering
* smooth antialiasing
* multisample antialiasing if supported
* physically believable lighting
* specular highlights
* ambient illumination
* soft shadows
* directional lighting
* subtle fill lighting
* rim lighting
* Fresnel-style edge response if appropriate
* realistic plastic material
* slightly different material response for stickers
* ambient occlusion or an approximation
* soft contact shadows between cubies
* realistic separation between cubies
* subtle bevels on cube edges
* rounded or chamfered cubie corners
* tone mapping
* gamma-correct rendering
* optional bloom used very conservatively
* high-quality depth buffering
* proper face occlusion
* smooth camera movement
* high-resolution rendering
* high-DPI display support

Do not overdo bloom, reflections, glow, or depth of field.

The cube should remain crisp and easy to inspect.

The aesthetic should resemble premium studio product photography.

---

# VISUAL ENVIRONMENT

Place the cube in a dark cinematic studio environment.

Preferred background:

very dark blue-black / charcoal, with a subtle vertical or radial gradient.

Approximate palette:

top:
#12141c

bottom:
#08090d

Optionally add a subtle floor or shadow-catching surface beneath the cube if it improves visual grounding.

The environment must remain restrained.

The Rubik's Cube is the hero object.

Avoid visual clutter.

---

# CUBE CONSTRUCTION

Model a real 3x3x3 Rubik's Cube.

Represent the cube internally as individual cubies.

The cube contains:

* 8 corner cubies
* 12 edge cubies
* 6 center cubies
* 1 hidden core location

Render the 26 externally visible cubies.

Do not fake the cube by drawing six colored grids on a solid box.

Each cubie must physically exist and have its own transform.

Each cubie should have:

* dark plastic body
* slightly beveled or rounded edges
* stickers attached to appropriate outward-facing surfaces
* a persistent identity
* a persistent orientation

Cubies should be separated by narrow physical gaps.

The gaps should be dark enough to strongly separate sticker colors visually.

---

# CUBIE MATERIAL

Plastic body:

approximately:

RGB(20-28, 20-28, 26-36)

The body should read as premium black or charcoal molded plastic.

It must not look medium gray.

Add subtle specular response.

The plastic should not look like perfect polished chrome.

A semi-gloss injection-molded plastic appearance is preferred.

Consider:

* roughness around 0.3-0.5
* restrained specular highlights
* very subtle edge highlights
* small bevels that naturally catch light

Beveled edges are strongly encouraged because they greatly improve visual realism.

---

# STICKERS

Each sticker should be a separate visual surface or mesh slightly above the cubie face.

Sticker size should occupy approximately 82-88% of the usable face.

Leave a visible black border around each sticker.

Sticker geometry should preferably have subtly rounded corners.

Sticker material should differ slightly from the plastic.

It may be:

* slightly smoother
* mildly glossy
* clear-coated
* more strongly illuminated than the plastic

Avoid making stickers look metallic.

Sticker color must remain vibrant.

Base colors before lighting:

White:
RGB(245, 245, 245)

Yellow:
RGB(255, 204, 0)

Red:
RGB(204, 24, 24)

Orange:
RGB(255, 138, 0)

Green:
RGB(0, 150, 70)

Blue:
RGB(0, 74, 200)

Do not pre-darken or desaturate these colors.

Lighting and material response should create the variations.

---

# STANDARD COLOR ORIENTATION

Initial solved orientation:

Top:
WHITE

Bottom:
YELLOW

Front:
GREEN

Back:
BLUE

Right:
RED

Left:
ORANGE

This orientation must remain internally consistent with move notation.

---

# LIGHTING

Lighting is extremely important.

The final result must avoid muddy or dull sticker colors.

Use a studio-style lighting arrangement.

A strong key light should come primarily from the camera/front-upper-right area rather than directly overhead.

The camera-facing side of the cube should receive enough light that:

* white genuinely reads as white
* yellow reads as saturated bright yellow
* red remains red rather than burgundy
* green remains vivid
* blue remains richly saturated

A useful conceptual key-light direction is approximately:

x = +0.3
y = +0.5
z = +0.8

assuming positive Z points generally toward the viewer.

Add additional subtle lighting if it improves the scene, such as:

* soft fill light
* rim light from behind
* very weak environmental illumination

Lighting must still preserve clear shape definition.

The front, top, and right visible surfaces should not all have exactly the same brightness.

Use physically plausible or artistically controlled diffuse/specular shading.

Do not flatten the scene using excessive ambient illumination.

---

# SHADOWS

If the chosen rendering stack allows it, implement convincing shadows.

Strongly preferred:

* soft directional or area-light-style shadows
* self-shadowing between cubies
* contact shadows in the gaps
* optional soft floor shadow beneath the cube

Avoid extremely hard, aliased shadow edges.

Shadow quality should support the premium studio aesthetic.

---

# GEOMETRY

Do not render cubies as perfectly sharp boxes unless the graphics library makes better geometry impractical.

Preferred:

beveled rectangular cubies.

Even a small bevel radius will dramatically improve lighting.

Generate proper normals for beveled geometry.

Corners should catch narrow highlights as the cube rotates.

This is one of the most important visual-quality improvements.

---

# CAMERA

Use a true perspective camera.

The cube must exhibit convincing perspective.

Near geometry must appear slightly larger than distant geometry.

Use a camera position resembling a high-end product shot.

Initial view should show roughly:

* front
* top
* right

The cube should occupy a substantial portion of the window while leaving breathing room.

Avoid extreme wide-angle distortion.

A moderate virtual focal length is preferred.

The camera itself may have extremely subtle cinematic motion if it improves the composition, but the primary movement should come from the cube.

---

# GLOBAL CUBE MOTION

Throughout ALL phases of the animation, the entire cube slowly rotates in space.

This rotation must continue:

* while solved
* during scramble moves
* during pauses
* during solve moves
* after solving

Use smooth combined rotation around multiple axes.

Avoid a simplistic constant single-axis turntable.

The global orientation should evolve slowly enough that viewers can appreciate the cube.

Example concept:

* slow yaw
* slower pitch variation
* very subtle roll

The result should feel cinematic rather than mechanical.

---

# RUBIK'S CUBE STATE MODEL

The Rubik's Cube must be modeled correctly.

Track each cubie using at least:

* original identity
* integer grid position
* current orientation
* sticker assignment
* current transform

Cubie positions occupy logical coordinates:

x ∈ {-1, 0, +1}
y ∈ {-1, 0, +1}
z ∈ {-1, 0, +1}

A move affects exactly one layer containing nine cubie positions.

Sticker colors belong to cubies.

Sticker colors must NEVER be recomputed based only on current position.

They must move and rotate with their cubies.

---

# MOVE NOTATION

Support:

R
L
U
D
F
B

These mean clockwise quarter-turns when viewing that face directly.

A trailing apostrophe means counterclockwise.

Examples:

R'
U'
F'

Carefully establish coordinate conventions before implementing moves.

Do not guess sign directions independently in different parts of the code.

Create a single authoritative mapping between:

* move
* axis
* layer coordinate
* rotation direction

---

# FACE TURN ANIMATION

This is critical.

When a face turn occurs:

only the nine cubies belonging to that layer should rotate.

The other cubies must remain stationary relative to the cube.

The entire cube's slow global rotation must continue simultaneously.

Transform composition should conceptually be:

cubie local transform

then

temporary animated layer rotation, if that cubie belongs to the active layer

then

global whole-cube transform

then

view transform

then

projection

The temporary layer rotation should smoothly progress from:

0 degrees

to

90 degrees

or

-90 degrees

depending on the move.

At completion:

commit the exact quarter-turn to the permanent cubie state.

Avoid accumulating floating-point error in logical cube state.

Logical grid positions should return to exact integer coordinates after every committed move.

Permanent cubie orientation should preferably use exact quarter-turn-compatible rotations or carefully normalized transforms.

---

# TURN MOTION QUALITY

Each face turn should take approximately:

0.45 to 0.60 seconds.

Do not use constant angular velocity.

Use a polished easing curve.

Preferred possibilities:

* cubic ease-in-out
* quintic ease-in-out
* smootherstep
* another high-quality monotonic easing function

The motion should accelerate naturally and settle smoothly.

Do not use excessive spring/bounce behavior.

A physical Rubik's Cube turn should feel decisive.

If appropriate, add an extremely small finishing ease or mechanical settle, but keep it subtle.

---

# TIMELINE

Implement one continuous automatic animation.

No user interaction is necessary.

## Phase 1

Solved cube.

Slow global rotation.

Duration:

2 seconds.

## Phase 2

Scramble.

Execute EXACTLY these ten moves:

R
U
F'
D
L'
B
R'
U'
F
D'

Execute them sequentially.

One animated move at a time.

Approximately 0.5 seconds per move.

## Phase 3

Scrambled pause.

Duration:

2 seconds.

Global cube rotation continues.

## Phase 4

Solve.

Execute EXACTLY:

D
F'
U
R
B'
L
D'
F
U'
R'

This is the exact mathematical inverse of the scramble.

At completion the cube MUST be exactly solved.

## Phase 5

Remain solved indefinitely.

Continue the cinematic slow rotation forever.

---

# MOVE CORRECTNESS

Do not merely create an animation that visually appears solved at the end.

The underlying cube state itself must return exactly to the initial solved state.

Implement a programmatic solved-state verification.

After the solve sequence completes:

compare all cubie logical positions and orientations with their original solved configuration.

If they do not match:

treat this as a bug and fix the move system.

Do not conceal state errors by repainting stickers.

---

# OPTIONAL DEBUG MODE

The production visual experience should remain clean.

However, add a debug mode accessible through a compile-time flag, constant, or keyboard toggle if useful.

Possible debug information:

* FPS
* current animation phase
* current move
* move number
* cubie axes
* layer membership
* cubie IDs
* camera coordinates
* cube solved-state result

Debug overlays must be OFF by default.

---

# RENDERING QUALITY

Target a smooth:

60 FPS minimum

on a modern desktop GPU.

Support window resizing.

Support high-DPI displays.

Render correctly at different aspect ratios.

The cube should remain centered and appropriately scaled.

Strongly consider:

* MSAA
* high-resolution depth buffer
* sRGB framebuffer
* gamma correction
* anisotropic filtering if textures are introduced
* proper alpha blending
* high-quality shader precision

If rendering to an intermediate framebuffer materially improves quality, use one.

---

# POST PROCESSING

Post processing is allowed.

Use it only when it improves the image.

Possible effects:

* subtle filmic tone mapping
* extremely restrained bloom
* subtle vignette
* color grading
* FXAA/SMAA if MSAA is insufficient
* slight exposure adjustment

Do NOT make the scene look like a video game HUD.

Do NOT bury the cube under effects.

The cube itself must remain extremely crisp.

---

# WINDOW

Create a native desktop window.

Suggested starting resolution:

1600 × 1000

or another similar high-resolution landscape window.

Allow resizing.

Use a descriptive title such as:

Go Rubik's Cube - Real-Time 3D

The animation should begin automatically when the program launches.

---

# AUDIO

Audio is NOT required.

Do not add generic background music.

If tasteful mechanical turn sounds are easy to create or synthesize and genuinely improve the final presentation, they may be added, but visual quality is the priority.

If audio complicates portability, omit it.

---

# CODE QUALITY

Although visual quality is the main priority, structure the project professionally.

Do not place an entire complex renderer into one enormous main.go file unless there is a compelling reason.

A possible organization:

cmd/
rubiks/
main.go

internal/
app/
cube/
render/
math3d/
animation/
graphics/

shaders/
assets/

However, choose the structure that best fits the selected graphics stack.

Use clear responsibilities.

Examples:

cube state
move engine
animation timeline
renderer
camera
lighting
materials
shader management
application lifecycle

---

# DOCUMENTATION

Create a README.md explaining:

1. what the project demonstrates
2. graphics stack chosen
3. why that graphics stack was chosen
4. how to install dependencies
5. how to run it
6. how to build a release executable
7. project architecture
8. cube-state representation
9. face-turn mathematics
10. rendering architecture
11. lighting system
12. shader system
13. animation timeline
14. verification that the final cube state is solved

Include exact commands.

---

# BUILD AND EXECUTION

Actually build and run the program.

Do not stop after writing source code.

Resolve:

* compilation failures
* missing packages
* shader errors
* runtime panics
* window initialization issues
* coordinate mistakes
* move-direction mistakes
* rendering artifacts

Continue until the program runs correctly.

---

# VISUAL INSPECTION

Do not assume that successful compilation means success.

Actually inspect the rendered application.

If the environment provides screenshot capture, framebuffer capture, screen inspection, or another visual inspection mechanism, use it.

Look specifically for:

* ugly proportions
* weak lighting
* dull colors
* clipping
* incorrect camera framing
* excessive empty space
* sticker z-fighting
* cubie overlap
* missing gaps
* poor bevels
* broken normals
* incorrect shadowing
* incorrect layer turns
* stickers floating away from cubies
* distracting aliasing
* excessive bloom
* incorrect material response
* animation that feels robotic

Refine the rendering based on what is actually visible.

Do not stop at the first technically working version.

---

# REQUIRED VISUAL CHARACTER

The final cube should feel:

premium

precise

dimensional

cinematic

vibrant

mechanically believable

smooth

substantial

The visual reference is not a browser coding exercise.

Think instead of:

a premium product commercial

a high-end 3D product configurator

a GPU demo

a polished motion-design shot

or a professional real-time rendering showcase.

---

# IMPORTANT FAILURE MODES TO AVOID

Do not:

* rotate the entire cube when performing a face move
* move fewer or more than nine cubies during a layer rotation
* repaint stickers according to position
* allow cubie logical coordinates to drift
* use incorrect clockwise/counterclockwise interpretation
* end in an unsolved cube
* make white stickers appear gray
* make yellow appear mustard
* use medium-gray plastic
* use perfectly flush stickers with no black border
* render cubies without visible gaps
* use completely sharp cube edges if beveling is feasible
* use flat unlit colors
* use excessive ambient lighting
* use excessive bloom
* use an orthographic camera
* create wide-angle distortion
* stop the global cube spin during face turns
* animate multiple scramble moves simultaneously
* fake a layer turn with textures
* fake the final solved state
* sacrifice rendering quality merely to reduce implementation complexity

---

# FINAL VERIFICATION

Before considering the project complete, verify all of the following.

The project is genuinely written in Go.

It launches as a native graphics application.

The rendered cube consists of individual cubies.

The cubies have dark plastic bodies.

Edges are beveled or otherwise visually softened.

Stickers are inset from cubie edges.

Dark gaps clearly separate stickers.

Sticker colors are vivid.

White looks white.

Yellow looks yellow.

Perspective is visible.

Lighting clearly defines form.

Specular response is visible but tasteful.

The cube casts or exhibits convincing shadows if supported.

The cube globally rotates throughout the entire sequence.

Each face move affects exactly nine cubies.

Face turns and global rotation occur simultaneously.

The scramble consists of exactly ten specified moves.

The solve consists of exactly ten specified inverse moves.

All twenty moves execute.

The underlying cube state finishes solved.

Animation remains smooth.

The final solved cube continues rotating indefinitely.

No visible geometry artifacts remain.

No shader errors remain.

No compilation or runtime errors remain.

---

# MOST IMPORTANT INSTRUCTION

Do not optimize this project for the smallest amount of code.

Do not optimize it for the fastest implementation.

Optimize for the **best final result**.

If a more sophisticated implementation produces substantially better geometry, lighting, animation, materials, shadows, antialiasing, or visual presentation, use the more sophisticated implementation.

Treat this as a graphics showcase built in Go, not merely as a demonstration that Go can open a window and draw a cube.

The finished result should make someone ask:

"That was written in Go?"
