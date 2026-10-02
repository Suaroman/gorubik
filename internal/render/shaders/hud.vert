#version 430 core
// Debug overlay glyphs. Positions arrive in pixels from the top left, which keeps the
// text crisp at any DPI and lets the overlay code be written without thinking about
// clip space at all.

layout(location = 0) in vec2 aPos;
layout(location = 1) in vec2 aUV;
layout(location = 2) in vec4 aColor;

uniform vec2 uScreen;

out vec2 vUV;
flat out vec4 vColor;

void main() {
    gl_Position = vec4(aPos.x / uScreen.x * 2.0 - 1.0, 1.0 - aPos.y / uScreen.y * 2.0, 0.0, 1.0);
    vUV = aUV;
    vColor = aColor;
}
