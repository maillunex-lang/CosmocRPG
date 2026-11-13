shader_type spatial;
render_mode unshaded, cull_disabled, blend_add, depth_draw_never;

uniform vec4 glow_color : source_color = vec4(0.35, 0.85, 1.0, 1.0);
uniform float intensity : hint_range(0.0, 4.0) = 1.5;
uniform float pulse_speed = 8.0;

void fragment() {
 float pulse = 0.82 + 0.18 * sin(TIME * pulse_speed);
 float edge = pow(1.0 - abs(NORMAL.z), 2.0);
 ALBEDO = glow_color.rgb * intensity * pulse;
 EMISSION = glow_color.rgb * intensity * pulse;
 ALPHA = glow_color.a * (0.65 + edge * 0.35);
}
