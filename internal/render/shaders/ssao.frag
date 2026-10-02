#version 430 core
// Screen-space ambient occlusion. Its purpose here is narrow and specific: the
// narrow gaps between cubies and the crease at a sticker's edge need to go darker
// than any amount of lighting would achieve on its own, because real cubes trap
// light in those crevices.

in vec2 vUV;
out vec4 outColor;

uniform sampler2D uDepth;
uniform sampler2D uNormal;
uniform mat4 uProj;
uniform mat4 uInvProj;
uniform vec2 uResolution;
uniform float uRadius;   // world units
uniform float uBias;     // world units, keeps coplanar samples from self-occluding
uniform float uPower;
uniform float uIntensity;
uniform int uKernelSize;
uniform vec3 uKernel[24];

// viewZ returns the positive distance from the camera plane for a window depth.
float viewZ(float d, vec2 uv) {
    vec4 v = uInvProj * vec4(uv * 2.0 - 1.0, d * 2.0 - 1.0, 1.0);
    return -(v.z / v.w);
}

void main() {
    float d = texture(uDepth, vUV).r;
    if (d >= 1.0) {
        outColor = vec4(1.0);
        return;
    }
    vec4 p4 = uInvProj * vec4(vUV * 2.0 - 1.0, d * 2.0 - 1.0, 1.0);
    vec3 P = p4.xyz / p4.w;
    vec3 N = normalize(texture(uNormal, vUV).xyz);
    float pz = -P.z;

    // Rotate the fixed kernel per pixel so the sampling pattern never lines up with
    // geometry; without this the occlusion reads as a regular grid in flat areas.
    float ang = hash12(gl_FragCoord.xy) * 2.0 * PI;
    float s = sin(ang), c = cos(ang);
    mat3 rot = mat3(c, s, 0.0, -s, c, 0.0, 0.0, 0.0, 1.0);

    float occ = 0.0;
    for (int i = 0; i < uKernelSize; i++) {
        vec3 sv = rot * uKernel[i];
        if (dot(sv, N) < 0.0) {
            sv = reflect(sv, N);
        }
        vec3 sp = P + sv * uRadius;
        vec4 off = uProj * vec4(sp, 1.0);
        vec2 suv = (off.xy / off.w) * 0.5 + 0.5;
        if (suv.x < 0.0 || suv.x > 1.0 || suv.y < 0.0 || suv.y > 1.0) {
            continue;
        }
        float sd = texture(uDepth, suv).r;
        if (sd >= 1.0) {
            continue;
        }
        float sceneZ = viewZ(sd, suv);
        float sampleZ = -sp.z;
        // The scene surface is nearer to the camera than the sample point, so the
        // sample point is buried inside geometry.
        if (sceneZ < sampleZ - uBias) {
            float rangeCheck = smoothstep(0.0, 1.0, uRadius / abs(pz - sceneZ));
            occ += rangeCheck;
        }
    }
    float ao = pow(clamp(1.0 - occ / float(uKernelSize), 0.0, 1.0), uPower);
    outColor = vec4(mix(1.0, ao, uIntensity));
}
