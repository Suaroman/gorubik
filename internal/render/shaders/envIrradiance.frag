#version 430 core
// Lambertian convolution: the diffuse ambient term. Cosine-weighted hemisphere
// sampling means the estimator is just the mean of the samples, with no explicit
// cosine factor left to divide.

in vec2 vUV;
out vec4 outColor;

uniform samplerCube uEnv;
uniform vec3 uFwd;
uniform vec3 uRight;
uniform vec3 uUp;
uniform int uSamples;

void basis(vec3 n, out vec3 t, out vec3 b) {
    vec3 up = vec3(0.0, 1.0, 0.0);
    if (abs(n.y) >= 0.95) {
        up = vec3(1.0, 0.0, 0.0);
    }
    t = normalize(cross(up, n));
    b = cross(n, t);
}

void main() {
    vec2 p = vUV * 2.0 - 1.0;
    vec3 N = normalize(uFwd + p.x * uRight + p.y * uUp);
    vec3 T, B;
    basis(N, T, B);

    vec3 sum = vec3(0.0);
    for (int i = 0; i < uSamples; i++) {
        float u1 = hash13(vec3(gl_FragCoord.xy, float(i) * 13.71));
        float u2 = hash13(vec3(gl_FragCoord.yx + 37.0, float(i) * 7.31 + 1.7));
        float r = sqrt(u1);
        float phi = 2.0 * PI * u2;
        vec3 d = T * (r * cos(phi)) + B * (r * sin(phi)) + N * sqrt(max(0.0, 1.0 - u1));
        sum += texture(uEnv, normalize(d)).rgb;
    }
    outColor = vec4(sum / float(uSamples), 1.0);
}
