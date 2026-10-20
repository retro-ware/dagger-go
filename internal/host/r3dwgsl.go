package host

// The Hardware 3D shaders. Everything is a palette index: a texel, or a face's solid colour,
// goes through the face's shade row (and per pixel through up to three point lights, as the
// engine's light shaders do), then through the fog table's row at the pixel's depth; flats
// through their shade, fog or translucency tables. The tables are the game's own bytes.

const r3dWGSL = `
struct Cam {
	v0: vec4f, v1: vec4f, v2: vec4f, // the view matrix's rows
	proj: vec4f,                     // near, far, half width / focal x, half height / focal y
	fog: vec4f,                      // start (world units, < 0: none), step, last row, last shade row
	vp: vec4f,                       // target pixel -> screen pixel scale (x, y), flat 0's material
	ndc: vec4f,                      // view -> clip: x scale, x offset, y scale, y offset (times z)
}
struct Inst { r0: vec4f, r1: vec4f, r2: vec4f, base: vec4u }

@group(0) @binding(0) var<uniform> cam: Cam;
@group(0) @binding(1) var<storage, read> insts: array<Inst>;
@group(0) @binding(2) var<storage, read> mats: array<vec4u>;
@group(0) @binding(3) var<storage, read> lights: array<vec4f>;
@group(0) @binding(4) var<storage, read> layers: array<u32>;
@group(0) @binding(5) var<storage, read> falloff: array<u32>;
@group(0) @binding(6) var texs: texture_2d_array<u32>;
@group(0) @binding(7) var tables: texture_2d<u32>;
@group(0) @binding(8) var dest: texture_2d<u32>;
@group(0) @binding(9) var bg: texture_2d<u32>;

struct VIn { @location(0) pos: vec3f, @location(1) uv: vec2f, @location(2) face: u32 }
struct VOut {
	@builtin(position) clip: vec4f,
	@location(0) uv: vec2f,
	@location(1) @interpolate(flat) face: u32,
	@location(2) view: vec3f,
}
struct Lo { @location(0) idx: u32, @location(1) id: u32, @location(2) z: f32 }

// view space has the view's edges at x = +-z, y = +-z (y down); depth runs near..far
fn project(s: vec3f) -> vec4f {
	let n = cam.proj.x;
	let f = cam.proj.y;
	return vec4f(s.x * cam.ndc.x + s.z * cam.ndc.y, s.z * cam.ndc.w - s.y * cam.ndc.z, (s.z - n) * f / (f - n), s.z);
}

@vertex
fn vsGeo(v: VIn, @builtin(instance_index) ii: u32) -> VOut {
	let t = insts[ii];
	let w = vec3f(dot(t.r0.xyz, v.pos) + t.r0.w, dot(t.r1.xyz, v.pos) + t.r1.w, dot(t.r2.xyz, v.pos) + t.r2.w);
	let s = vec3f(dot(cam.v0.xyz, w), dot(cam.v1.xyz, w), dot(cam.v2.xyz, w));
	var o: VOut;
	o.face = t.base.x + v.face;
	o.uv = v.uv;
	o.view = s;
	o.clip = project(s);
	if ((mats[o.face].y & 0x1000000u) != 0u) {
		o.clip = vec4f(0.0, 0.0, -1.0, 1.0); // a face turned away: the engine skips it
	}
	return o;
}

@vertex
fn vsFlat(v: VIn) -> VOut {
	var o: VOut;
	o.face = v.face;
	o.uv = v.uv;
	o.view = v.pos;
	o.clip = project(v.pos);
	return o;
}

fn tab(pos: i32, c: u32) -> u32 {
	let rows = i32(textureDimensions(tables).y);
	let r = pos >> 8u;
	if (r < 0 || r >= rows) { return c; }
	return textureLoad(tables, vec2i(i32(c), r), 0).r;
}

fn fall(h: u32) -> f32 {
	let w = falloff[h >> 1u];
	return f32((w >> ((h & 1u) * 16u)) & 0xFFFFu);
}

fn fogged(c: u32, z: f32) -> u32 {
	if (cam.fog.x < 0.0) { return c; }
	let z8 = min(z, cam.proj.y) / 256.0;
	if (z8 <= cam.fog.x) { return c; }
	return tab(i32(floor(cam.fog.z - cam.fog.y * (z8 - cam.fog.x))), c);
}

fn shadeGeo(i: VOut) -> u32 {
	let m = mats[i.face];
	var c: u32;
	if (m.x == 0xFFFFFFFFu) {
		c = m.y & 0xFFu;
	} else {
		let u = u32(i32(floor(i.uv.x))) & ((m.y >> 8u) & 0xFFu);
		let v = u32(i32(floor(i.uv.y))) & ((m.y >> 16u) & 0xFFu);
		c = textureLoad(texs, vec2i(i32(u), i32(v)), i32(layers[m.x]), 0).r;
	}
	let kind = (m.y >> 25u) & 3u;
	if (kind == 1u) {
		c = tab(i32(m.z), c);
	} else if (kind == 2u) {
		// the engine's light shader: the pixel's distance to each light's foot on the face's
		// plane, through LIGHT.DAT's falloff, adds to the row
		var pos = f32(i32(m.z));
		let p = floor(vec3f(i.view.x * cam.proj.z, i.view.y * cam.proj.w, i.view.z) / 256.0);
		let n = m.w >> 28u;
		let b = m.w & 0xFFFFFFFu;
		for (var k = 0u; k < n; k++) {
			let l = lights[b + 2u * k];
			let f = lights[b + 2u * k + 1u].x;
			let d = l.xyz + p;
			let h = floor(256.0 * dot(d, d) * f / 4294967296.0);
			if (h < 32768.0) {
				pos += floor(fall(u32(h)) * l.w / 4096.0);
			}
		}
		c = tab(i32(min(pos, cam.fog.w)), c);
	}
	return fogged(c, i.view.z);
}

@fragment
fn fsGeo(i: VOut) -> Lo { return Lo(shadeGeo(i), i.face, i.view.z); }

fn texelFlat(i: VOut) -> u32 {
	let m = mats[i.face];
	let w = (m.y >> 8u) & 0x1FFu;
	let h = (m.y >> 17u) & 0x1FFu;
	let u = u32(clamp(i32(floor(i.uv.x)), 0, i32(w) - 1));
	let v = u32(clamp(i32(floor(i.uv.y)), 0, i32(h) - 1));
	return textureLoad(texs, vec2i(i32(u), i32(v)), i32(layers[m.x]), 0).r;
}

fn shadeFlat(i: VOut, c0: u32) -> u32 {
	let m = mats[i.face];
	var c = c0;
	switch (m.y & 3u) {
		case 1u: { c = tab(i32(m.z), c); }
		case 2u: { c = tab(i32(m.w), tab(i32(m.z), c)); }
		case 3u: {
			var d = textureLoad(dest, vec2i(i.clip.xy), 0).r;
			if (d == 0xFFFFFFFFu) {
				d = textureLoad(bg, vec2i(floor(i.clip.xy * cam.vp.xy)), 0).r;
			}
			c = tab(i32(m.w) + i32(c) * 256, d);
		}
		default: {}
	}
	return c;
}

@fragment
fn fsFlat(i: VOut) -> Lo {
	let c = texelFlat(i);
	if (c == 0u) { discard; }
	return Lo(shadeFlat(i, c), 0x80000000u | (i.face - u32(cam.vp.z)), i.view.z);
}
`

// The composite: the game's screen, and inside the 3D view, where the game has not drawn
// over it since, the window-sized view (or the background behind it), through the palette.
const r3dCompositeWGSL = `
struct U {
	scale: vec4f, // window pixel -> screen pixel (x, y), -> window-sized view pixel (z, w)
	pal: array<vec4f, 256>,
}
@group(0) @binding(0) var frame: texture_2d<u32>; // screen, background, the view shows
@group(0) @binding(1) var hi: texture_2d<u32>;
@group(0) @binding(2) var<uniform> u: U;

struct VOut { @builtin(position) pos: vec4f }

@vertex
fn vs(@builtin(vertex_index) id: u32) -> VOut {
	let p = vec2f(f32((id << 1u) & 2u), f32(id & 2u));
	var o: VOut;
	o.pos = vec4f(p * vec2f(2.0, -2.0) + vec2f(-1.0, 1.0), 0.0, 1.0);
	return o;
}

@fragment
fn fs(i: VOut) -> @location(0) vec4f {
	let f = textureLoad(frame, vec2i(floor(i.pos.xy * u.scale.xy)), 0);
	var c = f.r;
	if (f.b != 0u) {
		let h = textureLoad(hi, vec2i(floor(i.pos.xy * u.scale.zw)), 0).r;
		c = select(f.g, h, h != 0xFFFFFFFFu);
	}
	return u.pal[c];
}
`
