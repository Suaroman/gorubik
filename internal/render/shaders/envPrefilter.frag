#version 430 core
// Prefiltered specular environment: one mip per roughness level, built by GGX
// importance sampling. This is what lets a rough sticker show a soft, wide studio
// reflection while polished plastic shows a tight one, without any runtime cost.

in vec2 vUV;
out vec4 outColor;

uniform samplerCube uEnv;
uniform vec3 uFwd;
uniform vec3 uRight;
uniform vec3 uUp;
uniform float uRoughness;
uniform int uSamples;

float radicalInverse(uint bits) {
    bits = (bits << 16u) | (bits >> 16u);
    bits = ((bits & 0x55555555u) << 1u) | ((bits & 0xAAAAAAAAu) >> 1u);
    bits = ((bits & 0x33333333u) << 2u) | ((bits & 0xCCCCCCCCu) >> 2u);
    bits = ((bits & 0x0F0F0F0Fu) << 4u) | ((bits & 0xF0F0F0F0u) >> 4u);
    bits = ((bits & 0x00FF00FFu) << 8u) | ((bits & 0xFF00FF00u) >> 8u);
    return float(bits) * 2.3283064365386963e-10;
}

void basis(vec3 n, out vec3 t, out vec3 b) {
    vec3 up = vec3(0.0, 1.0, 0.0);
    if (abs(n.y) >= 0.95) {
        up = vec3(1.0, 0.0, 0.0);
    }
    t = normalize(cross(up, n));
    b = cross(n, t);
}

vec3 importanceSampleGGX(vec2 xi, float rough, vec3 N, vec3 T, vec3 B) {
    float a = rough * rough;
    float phi = 2.0 * PI * xi.x;
    float cosTheta = sqrt((1.0 - xi.y) / (1.0 + (a * a - 1.0) * xi.y));
    float sinTheta = sqrt(max(0.0, 1.0 - cosTheta * cosTheta));
    vec3 h = vec3(cos(phi) * sinTheta, sin(phi) * sinTheta, cosTheta);
    return normalize(T * h.x + B * h.y + N * h.z);
}

void main() {
    vec2 p = vUV * 2.0 - 1.0;
    vec3 N = normalize(uFwd + p.x * uRight + p.y * uUp);
    vec3 T, B;
    basis(N, T, B);

    // Roughness 0 must stay a perfect mirror, otherwise the sharpest highlights get
    // blurred by a convolution that has nothing to average.
    float rough = max(uRoughness, 0.002);
    vec3 sum = vec3(0.0);
    float weight = 0.0;
    for (int i = 0; i < uSamples; i++) {
        vec2 xi = vec2(float(i) / float(uSamples), radicalInverse(uint(i)));
        vec3 H = importanceSampleGGX(xi, rough, N, T, B);
        vec3 L = 2.0 * dot(N, H) * H - N;
        float NoL = dot(N, L);
        if (NoL > 0.0) {
            sum += texture(uEnv, L).rgb * NoL;
            weight += NoL;
        }
    }
    outColor = vec4(sum / max(weight, 1e-4), 1.0);
}
