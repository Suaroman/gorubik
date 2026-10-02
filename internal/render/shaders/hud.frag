#version 430 core
// Glyph blit. The atlas holds coverage in one channel; output is premultiplied so the
// blend state is ONE, ONE_MINUS_SRC_ALPHA.

in vec2 vUV;
flat in vec4 vColor;

out vec4 outColor;

uniform sampler2D uAtlas;

void main() {
    float a = texture(uAtlas, vUV).r * vColor.a;
    if (a <= 0.002) {
        discard;
    }
    outColor = vec4(vColor.rgb * a, a);
}
