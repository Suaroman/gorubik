#version 430 core
// Bloom blur chain. One program, two modes: a 13-tap filter when stepping down (the
// wide kernel is what makes the glow large rather than merely soft) and a
// bilinear-weighted tent when stepping back up.

in vec2 vUV;
out vec4 outColor;

uniform sampler2D uSrc;
uniform vec2 uTexel;
uniform int uMode; // 0 = downsample, 1 = upsample

void main() {
    vec2 t = uTexel;
    if (uMode == 0) {
        vec3 c = texture(uSrc, vUV).rgb * 0.125;
        c += texture(uSrc, vUV + t * vec2(-1.0, 1.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(0.0, 1.0)).rgb * 0.125;
        c += texture(uSrc, vUV + t * vec2(1.0, 1.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(-1.0, 0.0)).rgb * 0.125;
        c += texture(uSrc, vUV + t * vec2(1.0, 0.0)).rgb * 0.125;
        c += texture(uSrc, vUV + t * vec2(-1.0, -1.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(0.0, -1.0)).rgb * 0.125;
        c += texture(uSrc, vUV + t * vec2(1.0, -1.0)).rgb * 0.0625;
        // Second, wider ring at half weight.
        c += texture(uSrc, vUV + t * vec2(-2.0, 2.0)).rgb * 0.03125;
        c += texture(uSrc, vUV + t * vec2(0.0, 2.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(2.0, 2.0)).rgb * 0.03125;
        c += texture(uSrc, vUV + t * vec2(-2.0, 0.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(2.0, 0.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(-2.0, -2.0)).rgb * 0.03125;
        c += texture(uSrc, vUV + t * vec2(0.0, -2.0)).rgb * 0.0625;
        c += texture(uSrc, vUV + t * vec2(2.0, -2.0)).rgb * 0.03125;
        outColor = vec4(c, 1.0);
        return;
    }
    // Upsample: a 3x3 tent over the half-resolution source. The four bilinear taps
    // each tent corner would otherwise need collapse into one sample via the half
    // texel offset. A 5x5 tent measured 4 ms of a 26 ms frame and, at the strength
    // this bloom runs at, is not distinguishable from this one.
    vec2 o = t * 0.5;
    vec3 c = texture(uSrc, vUV + vec2(-o.x, -o.y)).rgb * 0.0625;
    c += texture(uSrc, vUV + vec2(0.0, -o.y)).rgb * 0.125;
    c += texture(uSrc, vUV + vec2(o.x, -o.y)).rgb * 0.0625;
    c += texture(uSrc, vUV + vec2(-o.x, 0.0)).rgb * 0.125;
    c += texture(uSrc, vUV).rgb * 0.25;
    c += texture(uSrc, vUV + vec2(o.x, 0.0)).rgb * 0.125;
    c += texture(uSrc, vUV + vec2(-o.x, o.y)).rgb * 0.0625;
    c += texture(uSrc, vUV + vec2(0.0, o.y)).rgb * 0.125;
    c += texture(uSrc, vUV + vec2(o.x, o.y)).rgb * 0.0625;
    outColor = vec4(c, 1.0);
}
