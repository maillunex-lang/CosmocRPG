shader_type canvas_item;
// Фон стартового экрана: туманность, три слоя мерцающих звёзд с дрейфом, виньетка. Всё процедурно, без текстур.

uniform float time_s = 0.0;
uniform vec2 aspect = vec2(1.78, 1.0);
uniform float quality : hint_range(0.0, 1.0) = 1.0;

float hash(vec2 p) {
	p = fract(p * vec2(123.34, 456.21));
	p += dot(p, p + 45.32);
	return fract(p.x * p.y);
}

float vnoise(vec2 p) {
	vec2 i = floor(p);
	vec2 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);
	return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x), mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x), f.y);
}

float fbm(vec2 p) {
	float v = 0.0;
	float a = 0.5;
	for (int i = 0; i < 4; i++) {
		v += a * vnoise(p);
		p = p * 2.03 + vec2(7.1, 3.7);
		a *= 0.5;
	}
	return v;
}

float stars(vec2 uv, float scale, float speed, float density) {
	vec2 g = uv * scale + vec2(time_s * speed, 0.0);
	vec2 id = floor(g);
	vec2 f = fract(g) - 0.5;
	float h = hash(id);
	float on = step(1.0 - density, h);
	vec2 o = (vec2(hash(id + 1.7), hash(id + 9.2)) - 0.5) * 0.6;
	float d = length(f - o);
	float tw = 0.65 + 0.35 * sin(time_s * (1.2 + h * 3.0) + h * 40.0);
	return on * smoothstep(0.10 + 0.05 * h, 0.0, d) * tw;
}

void fragment() {
	vec2 uv = UV;
	vec2 p = uv * aspect;
	// база: глубокий синий, к правому верху фиолетовее
	vec3 col = mix(vec3(0.02, 0.05, 0.10), vec3(0.06, 0.04, 0.14), clamp(uv.x * 0.6 + (1.0 - uv.y) * 0.5, 0.0, 1.0));
	// туманность: две медленные волны шума, бирюзовая и фиолетовая
	float n1 = fbm(p * 1.4 + vec2(time_s * 0.006, 0.0));
	float n2 = fbm(p * 2.1 + vec2(3.0, -time_s * 0.005));
	float neb = smoothstep(0.35, 0.85, n1) * quality;
	col += vec3(0.07, 0.30, 0.38) * neb * (1.0 - uv.y * 0.5) * 0.9;
	col += vec3(0.30, 0.16, 0.55) * smoothstep(0.40, 0.90, n2) * quality * uv.y * 0.8;
	// звёзды: три слоя (дальний мелкий — ближний крупный)
	float s = stars(p, 22.0, 0.004, 0.10) * 0.55 + stars(p + 5.0, 12.0, 0.008, 0.08) * 0.8 + stars(p + 11.0, 6.0, 0.016, 0.05) * 1.1;
	col += vec3(0.80, 0.90, 1.0) * s;
	// виньетка: центр читаемее
	float v = smoothstep(1.15, 0.25, length((uv - 0.5) * vec2(1.0, 1.15)));
	col *= mix(0.55, 1.0, v);
	COLOR = vec4(col, 1.0);
}
