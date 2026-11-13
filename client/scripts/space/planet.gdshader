shader_type spatial;
render_mode cull_back, depth_draw_opaque;
// Процедурная планета-заглушка: полосы (газ), пятна (камень), мягкий лёд; атмосферная кайма. Без текстур.

uniform vec3 base_color : source_color = vec3(0.4, 0.55, 0.8);
uniform int style = 0;   // 0 камень, 1 газ, 2 лёд
uniform float seed = 1.0;
uniform float atmosphere_strength : hint_range(0.0, 2.0) = 0.8;
uniform vec3 atmosphere_color : source_color = vec3(0.25, 0.6, 1.0);

float hash(vec2 p) {
	p = fract(p * vec2(123.34, 456.21) + seed);
	p += dot(p, p + 45.32);
	return fract(p.x * p.y);
}

float vnoise(vec2 p) {
	vec2 i = floor(p);
	vec2 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x), mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x), f.y);
}

// шум без шва по долготе: долгота берётся как точка на окружности
float wrapnoise(vec2 uv, float k) {
	float a = uv.x * 6.2831853;
	return vnoise(vec2(cos(a), sin(a)) * k + vec2(uv.y * k * 1.7, 0.0));
}

void fragment() {
	float n = wrapnoise(UV, 1.4) * 0.5 + wrapnoise(UV + 0.13, 3.4) * 0.3 + wrapnoise(UV + 0.31, 8.0) * 0.2;
	vec3 c = base_color;
	if (style == 1) {
		float band = sin(UV.y * 38.0 + wrapnoise(UV, 1.2) * 4.0) * 0.5 + 0.5;
		c = mix(base_color * 0.65, base_color * 1.25, band) * (0.9 + n * 0.2);
	} else if (style == 2) {
		c = mix(base_color, vec3(0.9, 0.95, 1.0), smoothstep(0.35, 0.8, n) * 0.55);
	} else {
		c = mix(base_color * 0.6, base_color * 1.15, n);
	}
	float rim = pow(1.0 - clamp(dot(NORMAL, VIEW), 0.0, 1.0), 3.0);
	ALBEDO = c;
	EMISSION = base_color * rim * 0.9 + atmosphere_color * rim * atmosphere_strength;
	ROUGHNESS = 0.95;
}
