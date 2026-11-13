shader_type spatial;
render_mode unshaded, cull_back, depth_draw_never;

uniform vec3 sun_color : source_color = vec3(1.0, 0.48, 0.12);
uniform vec3 corona_color : source_color = vec3(1.0, 0.12, 0.03);
uniform float intensity = 4.0;
uniform float turbulence = 1.0;

float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = smoothstep(0.0, 1.0, fract(p));
    return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x), mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x), f.y);
}

void fragment() {
    float n = noise(UV * 7.0 + vec2(TIME * 0.12, -TIME * 0.08));
    float bands = 0.72 + 0.28 * sin(UV.y * 22.0 + n * 5.0 + TIME * 1.5);
    float rim = pow(1.0 - clamp(dot(NORMAL, VIEW), 0.0, 1.0), 2.0);
    vec3 color = mix(sun_color, corona_color, smoothstep(0.55, 1.0, n) * 0.45);
    ALBEDO = color;
    EMISSION = color * intensity * (0.85 + bands * 0.25 + rim * turbulence);
}