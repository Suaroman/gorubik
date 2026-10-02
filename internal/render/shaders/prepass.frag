#version 430 core
in vec3 vViewNormal;
out vec4 outColor;

void main() { outColor = vec4(normalize(vViewNormal), 0.0); }
