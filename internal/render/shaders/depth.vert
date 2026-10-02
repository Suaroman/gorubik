#version 430 core
// Depth-only pass for the shadow map. Same instance layout as cube.vert; normals
// and material are ignored, so only the matrix attributes are read.

layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec4 iM0;
layout(location = 3) in vec4 iM1;
layout(location = 4) in vec4 iM2;
layout(location = 5) in vec4 iM3;

uniform mat4 uLightViewProj;

void main() {
    mat4 M = mat4(iM0, iM1, iM2, iM3);
    gl_Position = uLightViewProj * M * vec4(aPos, 1.0);
}
