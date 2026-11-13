shader_type canvas_item;
render_mode blend_add;
uniform vec4 gate_color : source_color = vec4(0.9, 0.65, 0.25, 0.8);
uniform float intensity : hint_range(0.0, 3.0) = 1.0;
uniform float speed = 0.45;
void fragment() {
 vec2 p = UV - vec2(0.5);
 float radius = length(p);
 float ring = smoothstep(0.16, 0.13, abs(radius - 0.30 - 0.03 * sin(atan(p.y,p.x)*8.0 + TIME*speed*6.0)));
 float rays = 0.5 + 0.5 * sin(atan(p.y,p.x)*18.0 - TIME*speed*5.0);
 float alpha = ring * (0.45 + rays*0.55) * smoothstep(0.52, 0.12, radius);
 COLOR = vec4(gate_color.rgb * intensity, alpha * gate_color.a);
}
