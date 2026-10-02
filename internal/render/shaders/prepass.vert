#version 430 core
// Depth/normal prepass. Screen-space ambient occlusion needs view-space normals and
// the depth buffer; writing them here is cheaper than reconstructing normals from
// depth, and it keeps the crease between a sticker and its chamfer sharp.

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec4 iM0;
layout(location = 3) in vec4 iM1;
layout(location = 4) in vec4 iM2;
layout(location = 5) in vec4 iM3;

uniform mat4 uViewProj;
uniform mat4 uView;

out vec3 vViewNormal;

void main() {
    mat4 M = mat4(iM0, iM1, iM2, iM3);
    vViewNormal = mat3(uView) * (mat3(M) * aNormal);
    gl_Position = uViewProj * M * vec4(aPos, 1.0);
}
