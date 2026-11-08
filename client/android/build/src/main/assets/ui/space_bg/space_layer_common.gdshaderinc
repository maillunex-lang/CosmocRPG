// Общая часть шейдеров слоёв космического фона (звёзды, туманность, газ).
uniform sampler2D layer_texture : source_color, filter_linear, repeat_disable;
uniform vec2 uv_offset = vec2(0.0);     // левый верхний угол видимой области в UV текстуры
uniform vec2 uv_extent = vec2(1.0);     // размер видимой области в UV текстуры
uniform vec2 uv_flip = vec2(1.0);       // 1.0 или -1.0 по осям: отражение текстуры
uniform bool mirror_wrap = false;       // true: зеркальное повторение, false: ограничение краями
uniform float brightness : hint_range(0.0, 3.0) = 1.0;
uniform float intensity : hint_range(0.0, 3.0) = 1.0;
uniform vec4 tint : source_color = vec4(1.0);

vec4 sample_layer(vec2 screen_uv) {
	vec2 uv = uv_offset + screen_uv * uv_extent;
	if (uv_flip.x < 0.0) {
		uv.x = 1.0 - uv.x;
	}
	if (uv_flip.y < 0.0) {
		uv.y = 1.0 - uv.y;
	}
	if (mirror_wrap) {
		uv = 1.0 - abs(1.0 - mod(uv, 2.0));
	} else {
		uv = clamp(uv, 0.0, 1.0);
	}
	vec4 c = texture(layer_texture, uv);
	c.rgb *= tint.rgb * brightness;
	return c;
}
