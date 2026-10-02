#version 430 core
// Final composite: bloom, tone mapping, a touch of grading, and the one and only
// sRGB encoding in the frame. Everything upstream is linear HDR.

in vec2 vUV;
out vec4 outColor;

uniform sampler2D uHDR;
uniform sampler2D uBloom;
uniform float uExposure;
uniform float uBloomStrength;
uniform float uSaturation;
uniform float uWhite;
uniform float uContrast;
uniform float uVignette;
uniform float uDither;
uniform int uTonemap; // 0 = extended Reinhard, 1 = ACES

void main() {
    vec3 c = texture(uHDR, vUV).rgb + texture(uBloom, vUV).rgb * uBloomStrength;

    if (uTonemap == 1) {
        c = tonemapACES(c, uExposure, uSaturation);
    } else {
        c = tonemapFilmic(c, uExposure, uSaturation, uWhite);
    }

    // A gentle S that is exactly the identity at black and at white, so the
    // specified backdrop values survive untouched while mid-tones keep some punch.
    // Clamped first on purpose: the extended Reinhard curve passes above 1 for
    // radiance well over the white point, and the S term evaluated outside 0..1
    // turns negative hard - at c=3.8 with the default strength it flips the pixel
    // black - which drew a burnt-rimmed black ellipse at the core of the brightest
    // sticker glare. Pixels the clamp touches were destined for white anyway.
    c = clamp(c, 0.0, 1.0);
    c = c + uContrast * c * (1.0 - c) * (c - 0.5);

    vec2 q = vUV - 0.5;
    c *= 1.0 - uVignette * dot(q, q) * 2.0;

    c = linearToSRGB(max(c, vec3(0.0)));

    // A fraction of a least-significant-bit of noise kills banding in a gradient this
    // smooth; ordered dithering is enough and costs one hash.
    c += (hash12(gl_FragCoord.xy) - 0.5) * (uDither / 255.0);

    outColor = vec4(clamp(c, 0.0, 1.0), 1.0);
}
