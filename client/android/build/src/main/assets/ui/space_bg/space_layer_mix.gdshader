// Слой накладывается с учётом прозрачности (газ): intensity масштабирует непрозрачность.
shader_type canvas_item;
render_mode blend_mix;

#include "res://ui/space_bg/space_layer_common.gdshaderinc"

void fragment() {
	vec4 c = sample_layer(UV);
	COLOR = vec4(c.rgb, c.a * intensity);
}
