#version 430 core
// Renders the analytic studio into one face of the reflection cubemap. The whole
// environment is procedural, so there are no image assets to ship and the lighting
// can be retuned by changing four numbers.

in vec2 vUV;
out vec4 outColor;

uniform vec3 uFwd;
uniform vec3 uRight;
uniform vec3 uUp;
uniform float uSoftboxes;

void main() {
    vec2 p = vUV * 2.0 - 1.0;
    vec3 d = normalize(uFwd + p.x * uRight + p.y * uUp);
    outColor = vec4(studioEnv(d, uSoftboxes), 1.0);
}
