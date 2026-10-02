#version 430 core
// Bilateral blur for the ambient occlusion term: it smooths the sampling noise while
// the depth test stops occlusion from bleeding across a cubie edge into the gap
// beside it.

in vec2 vUV;
out vec4 outColor;

uniform sampler2D uAO;
uniform sampler2D uDepth;
uniform vec2 uTexel;
uniform float uRangeCheck;

void main() {
    float center = texture(uAO, vUV).r;
    float cd = texture(uDepth, vUV).r;
    float sum = center;
    float wsum = 1.0;
    for (int y = -2; y <= 2; y++) {
        for (int x = -2; x <= 2; x++) {
            if (x == 0 && y == 0) {
                continue;
            }
            vec2 uv = vUV + vec2(float(x), float(y)) * uTexel;
            float d = texture(uDepth, uv).r;
            float w = exp(-abs(d - cd) * uRangeCheck);
            sum += texture(uAO, uv).r * w;
            wsum += w;
        }
    }
    outColor = vec4(sum / wsum);
}
