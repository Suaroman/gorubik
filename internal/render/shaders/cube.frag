#version 430 core
// The cube's material shader: moulded plastic bodies and clear-coated stickers.
//
// Convention: diffuse is albedo (no 1/pi), so a light's colour uniform is the
// radiance it delivers at normal incidence, and the irradiance cubemap stores the
// cosine-weighted *average* radiance, which is what makes the two agree.

in vec3 vWorld;
in vec3 vNormal;
flat in vec4 vColor;
flat in vec4 vMat;

out vec4 outColor;

uniform vec3 uCamPos;

// Three-light studio. The directions come from common.glsl and point from the
// surface toward each light; they are shared with the reflection environment so a
// highlight and its light can never disagree.
uniform vec3 uKeyColor;
uniform vec3 uFillColor;
uniform vec3 uRimColor;


uniform samplerCube uIrradiance;
uniform samplerCube uPrefilter;
uniform float uPrefilterMips;
uniform float uEnvIntensity;

uniform sampler2D uAO;
uniform vec2 uResolution;
uniform float uAOStrength;

uniform int uDebugShade; // 0 shaded, 1 AO, 2 normals, 3 shadow, 4 coat

void main() {
    vec3 N = normalize(vNormal);
    vec3 V = normalize(uCamPos - vWorld);
    float NoV = max(dot(N, V), 1e-4);
    vec3 albedo = vColor.rgb;
    float rough = vColor.w;
    vec3 f0 = vec3(vMat.x);

    float ao = mix(1.0, texture(uAO, gl_FragCoord.xy / uResolution).r, uAOStrength);

    float kNL = max(dot(N, uKeyLightDir), 0.0);
    float fNL = max(dot(N, uFillLightDir), 0.0);
    float rNL = max(dot(N, uRimLightDir), 0.0);
    float keyVis = sampleShadow(vWorld, N, uKeyLightDir);

    vec3 Lo = vec3(0.0);
    Lo += (albedo + SpecularGGX(N, V, uKeyLightDir, f0, rough)) * uKeyColor * kNL * keyVis;
    Lo += (albedo + SpecularGGX(N, V, uFillLightDir, f0, rough)) * uFillColor * fNL;
    // The rim light is deliberately specular-heavy: it is there to draw a bright
    // hairline along the silhouette, not to lift the diffuse.
    Lo += (albedo * 0.35 + SpecularGGX(N, V, uRimLightDir, f0, max(rough * 0.6, 0.05))) * uRimColor * rNL;

    vec3 R = reflect(-V, N);
    vec3 irradiance = texture(uIrradiance, N).rgb;
    vec3 prefiltered = textureLod(uPrefilter, R, rough * (uPrefilterMips - 1.0)).rgb;
    vec2 ab = envBRDFApprox(NoV, rough);
    vec3 ambient = (irradiance * albedo * ao + prefiltered * (f0 * ab.x + ab.y) * ao) * uEnvIntensity;

    // Stickers carry a thin clear coat: a second, much smoother specular lobe that
    // reads as a lacquered surface without making the colour look metallic.
    float coat = vMat.y;
    if (coat > 0.0) {
        float cr = 0.07;
        vec3 cf0 = vec3(0.04);
        vec3 cs = SpecularGGX(N, V, uKeyLightDir, cf0, cr) * uKeyColor * kNL * keyVis
                + SpecularGGX(N, V, uFillLightDir, cf0, cr) * uFillColor * fNL
                + SpecularGGX(N, V, uRimLightDir, cf0, cr) * uRimColor * rNL;
        vec3 cpre = textureLod(uPrefilter, R, cr * (uPrefilterMips - 1.0)).rgb;
        vec2 cab = envBRDFApprox(NoV, cr);
        cs += cpre * (cf0 * cab.x + cab.y) * ao;
        ambient *= 1.0 - 0.18 * coat;
        Lo += cs * coat;
    }

    vec3 color = ambient + Lo;

    if (uDebugShade == 5) {
        // Unconditional output: proves whether this program's fragments reach the
        // frame at all, which is the first question when a mesh is missing.
        outColor = vec4(1.0, 0.0, 0.0, 1.0);
        return;
    }
    if (uDebugShade == 1) {
        color = vec3(ao);
    } else if (uDebugShade == 2) {
        color = N * 0.5 + 0.5;
    } else if (uDebugShade == 3) {
        color = vec3(keyVis);
    } else if (uDebugShade == 4) {
        color = vec3(rough);
    }
    // vMat.z marks cubies in the layer that is currently turning (overlay only).
    color += vec3(0.0, 0.35, 0.5) * vMat.z;

    outColor = vec4(color, 1.0);
}
