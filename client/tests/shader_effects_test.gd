extends SceneTree
const Core := preload("res://ui/graphics/shader_core.gd")
const Planet := preload("res://scripts/space/planet.gdshader")
const Shield := preload("res://ui/ships/shaders/shield_fresnel.gdshader")
const EngineShader := preload("res://ui/ships/shaders/engine_glow.gdshader")
const Gate := preload("res://ui/space_bg/warp_gate.gdshader")
const Vignette := preload("res://ui/space_bg/vignette.gdshader")
const Sun := preload("res://scripts/space/sun.gdshader")
const Shot := preload("res://ui/ships/shaders/shot_beam.gdshader")
const Ability := preload("res://ui/ships/shaders/ability_ring.gdshader")
var n:=0
var f:=0
func _initialize():
 check(Core.preset("low").get("noise_octaves",0)==1,"профиль low ограничивает шум")
 check(Core.preset("medium").get("effect_scale",0.0)>0.5,"профиль medium включает эффекты")
 check(Core.preset("high").get("noise_octaves",0)==3,"профиль high сохраняет детали")
 for pair in [[Planet,"планета"],[Shield,"щит Fresnel"],[EngineShader,"двигатель"],[Gate,"ворота"],[Vignette,"виньетка"],[Sun,"солнце"],[Shot,"обычный выстрел"],[Ability,"умения"]]:
  check(pair[0] != null,"шейдер загружен: "+str(pair[1]))
 var sm:=ShaderMaterial.new(); sm.shader=Shield; sm.set_shader_parameter("pulse",1.0)
 check(float(sm.get_shader_parameter("pulse"))==1.0,"пульс щита управляется параметром")
 print("Итого: проверок %d, провалов %d"%[n,f]); quit(1 if f>0 else 0)
func check(ok:bool,label:String):
 n+=1
 if not ok:f+=1;print("FAIL ",label)
 else:print("PASS ",label)
