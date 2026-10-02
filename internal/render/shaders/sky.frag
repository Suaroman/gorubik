#version 430 core
// The backdrop: the dark blue-black gradient the brief specifies, reconstructed per
// pixel from the view ray so it stays correct at any window size or aspect ratio.
//
// It is deliberately *not* the reflection environment. The softboxes live only in
// the environment map, so the plastic shows studio lights while the background
// behind the cube stays empty.

in vec2 vUV;
out vec4 outColor;

uniform mat4 uInvViewProj;
uniform vec3 uCamPos;

void main() {
    vec4 far = uInvViewProj * vec4(vUV * 2.0 - 1.0, 1.0, 1.0);
    vec3 dir = normalize(far.xyz / far.w - uCamPos);
    outColor = vec4(studioBackdrop(dir, vUV), 1.0);
}
