shader_type spatial;
render_mode unshaded, cull_disabled, blend_add, depth_draw_never;
uniform vec4 ability_color : source_color = vec4(0.3, 0.8, 1.0, 0.9);
uniform float intensity = 2.4;
uniform float pulse_speed = 6.0;
void fragment() {
    float radial = smoothstep(1.0, 0.12, length(UV - vec2(0.5)) * 2.0);
    float arc = 0.72 + 0.28 * sin(TIME * pulse_speed + UV.x * 24.0);
    ALBEDO = ability_color.rgb;
    EMISSION = ability_color.rgb * intensity * radial * arc;
    ALPHA = ability_color.a * radial;
}