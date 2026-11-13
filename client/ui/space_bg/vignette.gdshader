shader_type canvas_item;
render_mode blend_mix;
uniform float strength : hint_range(0.0, 0.35) = 0.12;
uniform vec4 color : source_color = vec4(0.0, 0.02, 0.05, 1.0);
void fragment() {
 vec2 p = UV * 2.0 - 1.0;
 float v = smoothstep(0.35, 1.2, dot(p,p));
 COLOR = vec4(color.rgb, v * strength);
}
