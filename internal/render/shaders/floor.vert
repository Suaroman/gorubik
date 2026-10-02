#version 430 core
// A single large quad lying under the cube. It is not really a floor so much as a
// shadow-catching surface: it fades to exactly the backdrop colour before its edge,
// so there is no visible seam and no horizon line to clutter the frame.

uniform mat4 uViewProj;
uniform float uRadius;
uniform float uY;

out vec3 vWorld;

void main() {
    float x = (gl_VertexID >= 2) ? uRadius : -uRadius;
    float z = (gl_VertexID == 1 || gl_VertexID == 3) ? uRadius : -uRadius;
    vec3 p = vec3(x, uY, z);
    vWorld = p;
    gl_Position = uViewProj * vec4(p, 1.0);
}
