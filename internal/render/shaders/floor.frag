#version 430 core
// Floor shading: a dark, slightly glossy surface whose only jobs are to catch the
// cube's shadow and to give the cube something to sit on.

in vec3 vWorld;
out vec4 outColor;

uniform vec3 uCamPos;
uniform vec2 uResolution;
uniform vec3 uKeyColor;
uniform vec3 uFillColor;
uniform vec3 uFloorAlbedo;
uniform float uFloorRough;
uniform float uFadeStart;
uniform float uFadeEnd;
uniform samplerCube uPrefilter;
uniform float uPrefilterMips;
uniform float uEnvIntensity; // the floor's own: see Settings.FloorEnvIntensity
uniform int uDebugShade;

void main() {
    vec3 N = vec3(0.0, 1.0, 0.0);
    float r = length(vWorld.xz);
    float fade = 1.0 - smoothstep(uFadeStart, uFadeEnd, r);
    if (fade <= 0.002) {
        // The backdrop pass already shaded this pixel, and the floor is the largest
        // surface on screen, so re-deriving a value that is already in the buffer is
        // the most expensive no-op in the frame.
        discard;
    }

    vec3 V = normalize(uCamPos - vWorld);
    vec3 dir = normalize(vWorld - uCamPos);
    vec2 screen = gl_FragCoord.xy / uResolution;
    vec3 backdrop = studioBackdrop(dir, screen);

    float vis = sampleShadow(vWorld, N, uKeyLightDir);
    // The floor is the only large surface the cube can actually shade, so it is
    // where a shadow factor gets inspected.
    if (uDebugShade == 3) {
        outColor = vec4(vec3(vis), 1.0);
        return;
    }
    float kNL = max(dot(N, uKeyLightDir), 0.0);
    float fNL = max(dot(N, uFillLightDir), 0.0);

    vec3 Lo = (uFloorAlbedo + SpecularGGX(N, V, uKeyLightDir, vec3(0.04), uFloorRough))
              * uKeyColor * kNL * vis;
    Lo += (uFloorAlbedo + SpecularGGX(N, V, uFillLightDir, vec3(0.04), uFloorRough))
          * uFillColor * fNL;

    // A blurred reflection of the studio in the floor, which is what stops it
    // reading as a matte card.
    vec3 R = reflect(-V, N);
    vec3 pre = textureLod(uPrefilter, R, uFloorRough * (uPrefilterMips - 1.0)).rgb;
    vec2 ab = envBRDFApprox(max(dot(N, V), 0.0), uFloorRough);
    vec3 ambient = pre * (vec3(0.04) * ab.x + ab.y) * uEnvIntensity;
    // Measured at the shared 1.6, this term alone put the floor at a tonemapped 70
    // against a backdrop of 26: the prefilter map holds a softbox of radiance 16,
    // and a surface this rough integrates most of it. The floor is scenery.

    outColor = vec4(mix(backdrop, ambient + Lo, fade), 1.0);
}
