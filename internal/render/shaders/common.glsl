// common.glsl is prepended to every fragment shader by the shader loader in
// internal/render/shaders.go, so anything shared between passes lives here.
//
// The scene is shaded in linear HDR throughout; sRGB encoding happens exactly
// once, in the final composite.

const float PI = 3.141592653589793;

float saturate1(float v) { return clamp(v, 0.0, 1.0); }

// ---------------------------------------------------------------------------
// Microfacet BRDF (Cook-Torrance with GGX)
// ---------------------------------------------------------------------------

float D_GGX(float NoH, float roughness) {
    float a = roughness * roughness;
    float a2 = a * a;
    float d = NoH * NoH * (a2 - 1.0) + 1.0;
    return a2 / max(PI * d * d, 1e-7);
}

float V_SmithGGX(float NoV, float NoL, float roughness) {
    float r = roughness + 1.0;
    float k = (r * r) / 8.0;
    float gv = NoV / (NoV * (1.0 - k) + k);
    float gl = NoL / (NoL * (1.0 - k) + k);
    return gv * gl;
}

vec3 F_Schlick(float cosTheta, vec3 f0) {
    return f0 + (1.0 - f0) * pow(saturate1(1.0 - cosTheta), 5.0);
}

// Roughness-aware variant: keeps the grazing reflection from exceeding 1 on
// rough surfaces, which would otherwise blow out the plastic edges.
vec3 F_SchlickRoughness(float cosTheta, vec3 f0, float roughness) {
    return f0 + (max(vec3(1.0 - roughness), f0) - f0) * pow(saturate1(1.0 - cosTheta), 5.0);
}

// Full specular term for one light, already divided by the 4*NoV*NoL that the
// rendering equation needs (the NoL cancels against the cosine factor).
vec3 SpecularGGX(vec3 N, vec3 V, vec3 L, vec3 f0, float roughness) {
    vec3 H = normalize(L + V);
    float NoV = max(dot(N, V), 1e-4);
    float NoL = max(dot(N, L), 0.0);
    float NoH = max(dot(N, H), 0.0);
    float VoH = max(dot(V, H), 0.0);
    if (NoL <= 0.0) {
        return vec3(0.0);
    }
    vec3 F = F_Schlick(VoH, f0);
    return (D_GGX(NoH, roughness) * V_SmithGGX(NoV, NoL, roughness)) * F / (4.0 * NoV * NoV);
}

// ---------------------------------------------------------------------------
// Small hashes for sampling rotation, dithering and shadow-kernel rotation
// ---------------------------------------------------------------------------

float hash12(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}

float hash13(vec3 p) {
    p = fract(p * 0.1031);
    p += dot(p, p.zyx + 31.32);
    return fract((p.x + p.y) * p.z);
}


// ---------------------------------------------------------------------------
// Shadowing
//
// One directional shadow map shared by every surface, so the cube's own creases,
// the gap shadows and the pool of shade on the floor all agree.
// ---------------------------------------------------------------------------

uniform mat4 uShadowViewProj;
uniform highp sampler2DShadow uShadow;
uniform float uShadowTexel; // 1 / map size in texels
uniform float uTexelWorld;  // world units covered by one shadow texel
uniform float uNormalBias;  // sample offset along the surface normal, in texels
uniform float uSoftness;    // PCF radius in shadow texels

const vec2 kPoisson[16] = vec2[16](
    vec2(-0.94201624, 0.39901164),
    vec2(-0.09418410, -0.72938870),
    vec2(0.01260554, 0.17554856),
    vec2(-0.43687051, -0.03435325),
    vec2(0.42142420, 0.28264833),
    vec2(-0.58642405, 0.32725688),
    vec2(0.32455753, -0.02966917),
    vec2(-0.23760507, -0.65961820),
    vec2(0.16391616, 0.59654649),
    vec2(0.34773606, 0.47087364),
    vec2(-0.06138267, 0.89719389),
    vec2(-0.53293251, -0.17310847),
    vec2(0.66211373, -0.24561246),
    vec2(0.24426497, 0.33462904),
    vec2(0.09317117, -0.66068926),
    vec2(0.66178615, -0.37658103));

// envBRDFApprox is Karis's analytic fit of the split-sum environment integral,
// which avoids a 2D look-up table at the cost of a little accuracy at grazing
// angles on very rough surfaces - not a regime this cube ever enters.
vec2 envBRDFApprox(float NoV, float roughness) {
    const vec4 c0 = vec4(-1.0, -0.0275, -0.572, 0.022);
    const vec4 c1 = vec4(1.0, 0.0425, 1.04, -0.04);
    vec4 r = roughness * c0 + c1;
    float a004 = min(r.x * r.x, exp2(-9.28 * NoV)) * r.x + r.y;
    return vec2(-1.04, 1.04) * a004 + r.zw;
}

// sampleShadow offsets the sample point along the surface normal by a couple of
// shadow texels before projecting. That removes the stripe pattern self-shadowing
// would otherwise produce on surfaces facing the light, without the floating look
// that a large constant depth bias causes.
float sampleShadow(vec3 world, vec3 N, vec3 L) {
    vec3 p = world + N * (uNormalBias * uTexelWorld) - L * (0.5 * uTexelWorld);
    vec4 clip = uShadowViewProj * vec4(p, 1.0);
    vec3 ndc = clip.xyz / clip.w;
    vec3 uvz = vec3(ndc.xy * 0.5 + 0.5, ndc.z * 0.5 + 0.5);
    if (uvz.z >= 1.0 || any(lessThan(uvz.xy, vec2(-0.02))) || any(greaterThan(uvz.xy, vec2(1.02)))) {
        return 1.0;
    }
    // The map only covers a box around the cube. Returning lit the instant a
    // sample leaves it draws a straight lit/shadowed seam across the floor at the
    // box edge, so the last sixth of the map blends out to fully lit instead.
    vec2 e = abs(uvz.xy - 0.5) * 2.0;
    float border = (1.0 - smoothstep(0.5, 1.0, e.x)) * (1.0 - smoothstep(0.5, 1.0, e.y));
    if (border <= 0.0) {
        return 1.0;
    }
    // Rotated Poisson PCF. The rotation is per pixel, so the residual error reads
    // as noise that a wide kernel averages out rather than as a fixed grid.
    float ang = hash12(gl_FragCoord.xy) * 2.0 * PI;
    float s = sin(ang), c = cos(ang);
    float radius = uSoftness * uShadowTexel;
    // Eight of the sixteen vectors, chosen by a per-pixel phase. Sixteen measured
    // 7.3 ms on the floor alone - it covers half the frame and pays for every tap
    // eight times over because of MSAA - and taking an alternating half of the same
    // disc keeps the estimate unbiased while the per-pixel rotation and phase make
    // the missing taps average out across the penumbra.
    int phase = int(hash12(gl_FragCoord.xy + 17.0) * 2.0) * 2;
    float sum = 0.0;
    for (int i = 0; i < 8; i++) {
        vec2 o = kPoisson[i * 2 + phase];
        vec2 r = vec2(s * o.x - c * o.y, c * o.x + s * o.y) * radius;
        sum += texture(uShadow, vec3(uvz.xy + r, uvz.z));
    }
    return mix(1.0, sum / 8.0, border);
}

// ---------------------------------------------------------------------------
// Colour
// ---------------------------------------------------------------------------

vec3 linearToSRGB(vec3 c) {
    c = max(c, vec3(0.0));
    vec3 lo = c * 12.92;
    vec3 hi = 1.055 * pow(c, vec3(1.0 / 2.4)) - 0.055;
    return mix(hi, lo, lessThanEqual(c, vec3(0.0031308)));
}

vec3 srgbToLinear(vec3 c) {
    c = max(c, vec3(0.0));
    vec3 lo = c / 12.92;
    vec3 hi = pow((c + 0.055) / 1.055, vec3(2.4));
    return mix(hi, lo, lessThanEqual(c, vec3(0.04045)));
}

// ACES filmic response (Narkowicz's fit of the ACES RRT+ODT curve) with an
// exposure control and a saturation recovery. The plain curve desaturates vivid
// hues as they brighten, which is exactly what would make the yellow stickers read
// mustard, so the chroma is restored after the roll-off.
vec3 tonemapACES(vec3 color, float exposure, float saturation) {
    color *= exposure;
    const float a = 2.51;
    const float b = 0.03;
    const float c = 2.43;
    const float d = 0.59;
    const float e = 0.14;
    vec3 mapped = clamp((color * (color * a + b)) / (color * (color * c + d) + e), 0.0, 1.0);
    float luma = dot(mapped, vec3(0.2126, 0.7152, 0.0722));
    return mix(vec3(luma), mapped, saturation);
}

// A softer shoulder for comparison: Reinhard extended, which keeps more saturation
// than ACES but less highlight separation.
vec3 tonemapFilmic(vec3 color, float exposure, float saturation, float white) {
    color *= exposure;
    float w2 = white * white;
    vec3 t = color * (1.0 + color / w2) / (1.0 + color);
    float luma = dot(t, vec3(0.2126, 0.7152, 0.0722));
    return mix(vec3(luma), t, saturation);
}

// ---------------------------------------------------------------------------
// The studio
//
// Two related but different things live here. studioBackdrop is what the camera
// sees directly: the dark blue-black gradient the brief specifies. studioEnv is the
// reflection environment, which additionally contains the softboxes. Keeping them
// apart is what lets the plastic show believable rectangular highlights while the
// picture behind the cube stays empty.
// ---------------------------------------------------------------------------

// softbox returns 0..1 coverage of a rectangular area light seen from direction d.
// c is the direction from the origin toward its centre; extent is its angular
// half-extent in radians; radius rounds the corners; soft controls the edge.
// ("half" is a reserved word in GLSL, so the parameter is called extent.)
float softbox(vec3 d, vec3 c, vec3 x, vec3 y, vec2 extent, float radius, float soft) {
    float along = dot(d, c);
    if (along <= 0.0) {
        return 0.0;
    }
    vec3 l = d / along;
    vec2 p = abs(vec2(dot(l, x), dot(l, y)));
    vec2 q = p - (extent - vec2(radius));
    float sd = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;
    return 1.0 - smoothstep(-soft, soft, sd);
}

// tangentFrame builds an orthonormals basis around a light direction.
void tangentFrame(vec3 c, out vec3 x, out vec3 y) {
    vec3 up = vec3(0.0, 1.0, 0.0);
    if (abs(c.y) >= 0.95) {
        up = vec3(1.0, 0.0, 0.0);
    }
    x = normalize(cross(up, c));
    y = cross(c, x);
}

uniform vec3 uKeyLightDir;
uniform vec3 uFillLightDir;
uniform vec3 uRimLightDir;

vec3 studioBackdrop(vec3 dir, vec2 uv) {
    // The gradient is swept across the frame rather than across the view ray: the
    // camera sits nearly level with the cube, so dir.y only spans about +/-0.13
    // over the whole image and every pixel would land on the same mid-tone.
    float t = clamp(uv.y, 0.0, 1.0);
    // Converted to linear offline instead of calling srgbToLinear here: this runs
    // on every backdrop pixel, and the six pow() the conversion needs measured as
    // a third of the pass. Same colours, #12141c over #08090d.
    vec3 top = vec3(0.006049, 0.006995, 0.011612);
    vec3 bottom = vec3(0.002428, 0.002732, 0.004025);
    vec3 col = mix(bottom, top, smoothstep(0.0, 1.0, t));
    // A faint pool behind the cube so the hero is not sitting on a flat wash.
    float d = length((uv - vec2(0.5, 0.55)) * vec2(1.0, 1.3));
    col *= 1.0 + 0.5 * exp(-d * d * 4.0);
    return col;
}

// studioRoom is the dim enclosure the cube sits in: the gradient and the bounce up
// off the floor. It is what the diffuse ambient term convolves.
vec3 studioRoom(vec3 d) {
    float t = d.y * 0.5 + 0.5;
    vec3 col = mix(vec3(0.0016, 0.0020, 0.0034), vec3(0.0090, 0.0110, 0.0180), t);
    // Weak warm bounce from the floor the cube sits above.
    col += vec3(0.006, 0.0055, 0.005) * smoothstep(0.05, -0.7, d.y);
    return col;
}

// studioSoftboxes adds the three area lights, which is what puts believable
// rectangular highlights in the plastic.
vec3 studioSoftboxes(vec3 d) {
    vec3 col = vec3(0.0);
    vec3 kx, ky;
    tangentFrame(uKeyLightDir, kx, ky);
    col += vec3(1.00, 0.97, 0.92) * 16.0 * softbox(d, uKeyLightDir, kx, ky, vec2(0.40, 0.30), 0.14, 0.22);

    vec3 fx, fy;
    tangentFrame(uFillLightDir, fx, fy);
    col += vec3(0.62, 0.72, 1.00) * 1.5 * softbox(d, uFillLightDir, fx, fy, vec2(0.75, 0.55), 0.30, 0.45);

    vec3 rx, ry;
    tangentFrame(uRimLightDir, rx, ry);
    col += vec3(0.80, 0.88, 1.00) * 7.0 * softbox(d, uRimLightDir, rx, ry, vec2(0.55, 0.09), 0.05, 0.30);
    return col;
}

// softboxes is 1 when generating the map the specular prefilter convolves and 0 for
// the one the irradiance convolution reads. The same three lights are shaded
// analytically in every surface shader, so an irradiance map that also contained
// them charged for the key light twice: measured, that lifted the red face from a
// tonemapped 217 to a clipped 255 and brought the shadow side up to the lit side.
// The room carries the indirect bounce; the lights themselves stay analytic.
vec3 studioEnv(vec3 d, float softboxes) {
    vec3 col = studioRoom(d);
    if (softboxes > 0.0) {
        col += studioSoftboxes(d) * softboxes;
    }
    return col;
}
