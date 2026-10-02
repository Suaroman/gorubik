#version 430 core
// One vertex shader for cubie bodies and stickers. Both are instanced with the
// same layout, so the two draws share a program and differ only in their vertex
// array and instance data.
//
// Instance data:
//   locations 2..5 : model matrix, column-major
//   location  6    : linear albedo rgb, roughness in w
//   location  7    : x = F0, y = clear-coat strength, z = debug highlight, w = spare
//
// The model matrix already contains the cubie's grid position, its committed
// orientation, the animated layer turn if it belongs to the turning layer, and the
// whole-cube spin, in that order.

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec4 iM0;
layout(location = 3) in vec4 iM1;
layout(location = 4) in vec4 iM2;
layout(location = 5) in vec4 iM3;
layout(location = 6) in vec4 iColor;
layout(location = 7) in vec4 iMat;

uniform mat4 uViewProj;

out vec3 vWorld;
out vec3 vNormal;
flat out vec4 vColor;
flat out vec4 vMat;

void main() {
    mat4 M = mat4(iM0, iM1, iM2, iM3);
    vec4 world = M * vec4(aPos, 1.0);
    vWorld = world.xyz;
    // Every instance matrix is a pure rotation plus translation, so the upper
    // 3x3 is its own normal matrix.
    vNormal = mat3(M) * aNormal;
    vColor = iColor;
    vMat = iMat;
    gl_Position = uViewProj * world;
}
