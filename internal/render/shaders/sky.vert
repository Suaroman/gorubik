#version 430 core
// The backdrop's full-screen triangle, identical to fullscreen.vert except that it
// writes a window depth of exactly 1.0. That is what lets it be drawn last with
// GL_EQUAL: it then shades only the pixels no other pass wrote depth into, instead
// of shading the whole frame and having the floor and cube painted over it.
out vec2 vUV;

void main() {
    vec2 p = vec2(gl_VertexID == 1 ? 3.0 : -1.0, gl_VertexID == 2 ? 3.0 : -1.0);
    vUV = p * 0.5 + 0.5;
    gl_Position = vec4(p, 1.0, 1.0);
}
