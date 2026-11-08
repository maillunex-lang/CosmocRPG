// Слой складывается с тем, что под ним (звёзды, туманность): intensity масштабирует вклад.
shader_type canvas_item;
render_mode blend_add;

#include "res://ui/space_bg/space_layer_common.gdshaderinc"

void fragment() {
	vec4 c = sample_layer(UV);
	COLOR = vec4(c.rgb * intensity, c.a);
}
