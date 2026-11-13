shader_type spatial;
render_mode unshaded, cull_disabled, blend_add, depth_draw_never;

uniform vec4 beam_color : source_color = vec4(0.25, 0.9, 1.0, 1.0);
uniform float intensity = 3.0;

void fragment() {
    float edge = 1.0 - smoothstep(0.15, 0.5, abs(UV.y - 0.5) * 2.0);
    float pulse = 0.82 + 0.18 * sin(TIME * 24.0 + UV.x * 18.0);
    ALBEDO = beam_color.rgb;
    EMISSION = beam_color.rgb * intensity * edge * pulse;
    ALPHA = beam_color.a * edge;
}