shader_type spatial;
render_mode unshaded, cull_disabled, blend_add, depth_draw_never;

uniform vec4 shield_color : source_color = vec4(0.25, 0.65, 1.0, 0.22);
uniform float fresnel_power : hint_range(1.0, 6.0) = 3.0;
uniform float pulse : hint_range(0.0, 1.0) = 0.0;
uniform float intensity : hint_range(0.0, 3.0) = 1.0;

void fragment() {
 float edge = pow(1.0 - clamp(dot(NORMAL, VIEW), 0.0, 1.0), fresnel_power);
 float scan = 0.82 + 0.18 * sin(TIME * 4.0 + VERTEX.y * 3.0);
 float a = shield_color.a * (0.28 + edge * 0.9 + pulse * 0.7) * scan;
 ALBEDO = shield_color.rgb * intensity;
 EMISSION = shield_color.rgb * (edge + pulse) * intensity;
 ALPHA = a;
}
