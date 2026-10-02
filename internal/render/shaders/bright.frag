#version 430 core
// Bright pass with a soft knee: only the highlights that are genuinely over the
// threshold enter the bloom chain, so the effect stays a lens artefact rather than
// turning the whole cube fuzzy.

in vec2 vUV;
out vec4 outColor;

uniform sampler2D uSrc;
uniform float uThreshold;
uniform float uKnee;

void main() {
    vec3 c = texture(uSrc, vUV).rgb;
    float br = max(c.r, max(c.g, c.b));
    float soft = clamp(br - uThreshold + uKnee, 0.0, 2.0 * uKnee);
    soft = soft * soft / (4.0 * uKnee + 1e-4);
    float contrib = max(soft, br - uThreshold) / max(br, 1e-4);
    outColor = vec4(c * contrib, 1.0);
}
