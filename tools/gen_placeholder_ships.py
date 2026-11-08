#!/usr/bin/env python3
"""Генератор паспортов и клиентских описаний кораблей-заглушек из ТЗ «Корабли» (SHIPS_SPEC.md, раздел 2.3).
Модели пока временные: действующие GLB «Стрижа» с разными масштабом и окраской. Настоящие GLB (Meshy AI) подставляются
заменой пути в assets/ships/<папка>/ship.json без правок кода. Запускать из корня репозитория; повторный запуск идемпотентен."""
import json, os, copy

SHIPS = [
  # id, папка, имя, класс, роль, специализации {id: (имя, текст)}, по умолчанию, характеристики, пульс, габариты, оружие, экипаж, двигатели, масштаб, окраска, пламя, облики
  dict(id="FTR-002", dir="ftr002", name="Ястреб", cls="Тяжёлый истребитель", role="Боевой", specs={"interceptor": ("Перехватчик", "Перехват: рывок к цели и усиленный выстрел"), "scout": ("Разведчик", "Импульсный сканер: обнаружение скрытых объектов")},
       default="interceptor", hull=1600, shield=700, armor=160, energy=110, cargo=10, speed=13.0, regen=(16, 4, 9), pulse=(28, 2, 32, 1.0, 2), boost=(1.7, 3, 13, 22), dims=[13, 9.2, 3.2], weapons="3 энергетические пушки, 2 ракетные точки", crew=1, engines=2, scale=2.5, tint=[1.0, 0.72, 0.72], flame=[1.0, 0.45, 0.25], skins=["a", "b"], extras=["intercept", "scan"]),
  dict(id="COR-001", dir="cor001", name="Барс", cls="Корвет", role="Боевой", specs={"hunter": ("Охотник", "Усиленное вооружение против малых целей"), "guard": ("Страж", "Повышенная живучесть для сопровождения")},
       default="hunter", hull=2600, shield=1200, armor=220, energy=140, cargo=14, speed=11.5, regen=(20, 4, 10), pulse=(34, 2, 30, 1.1, 3), boost=(1.5, 3, 15, 25), dims=[24, 14, 5], weapons="4 энергетические пушки", crew=2, engines=2, scale=2.9, tint=[0.75, 0.95, 0.8], flame=[0.4, 1.0, 0.55], skins=["a"], extras=[]),
  dict(id="COR-002", dir="cor002", name="Кобра", cls="Корвет", role="Боевой", specs={"raider": ("Рейдер", "Скорость и манёвренность для налётов"), "guard": ("Страж", "Повышенная живучесть для сопровождения")},
       default="raider", hull=2200, shield=1100, armor=200, energy=150, cargo=12, speed=12.5, regen=(20, 4, 11), pulse=(30, 3, 28, 0.9, 3), boost=(1.7, 3, 14, 25), dims=[22, 13, 4.6], weapons="3 энергетические пушки, 2 ракетные точки", crew=2, engines=3, scale=2.8, tint=[0.8, 0.8, 1.0], flame=[0.55, 0.6, 1.0], skins=["a"], extras=[]),
  dict(id="FRG-001", dir="frg001", name="Страж", cls="Фрегат", role="Универсальный", specs={"escort": ("Сопровождение", "Прикрытие эскадры"), "support": ("Поддержка", "Усиленные щиты (ремонтные модули появятся с боевым этапом)")},
       default="escort", hull=5000, shield=2500, armor=350, energy=220, cargo=40, speed=9.0, regen=(30, 5, 12), pulse=(46, 3, 34, 1.2, 4), boost=(1.4, 3, 18, 35), dims=[48, 22, 9], weapons="6 энергетических пушек, 2 ракетные установки", crew=6, engines=3, scale=3.6, tint=[0.85, 0.9, 1.0], flame=[0.35, 0.9, 1.0], skins=["a"], extras=[]),
  dict(id="FRG-002", dir="frg002", name="Бастион", cls="Фрегат", role="Тяжёлая броня", specs={"bulwark": ("Бастион", "Максимальная броня и живучесть"), "escort": ("Сопровождение", "Прикрытие эскадры")},
       default="bulwark", hull=5600, shield=2200, armor=420, energy=200, cargo=36, speed=8.5, regen=(26, 5, 11), pulse=(44, 3, 32, 1.2, 4), boost=(1.35, 3, 18, 35), dims=[52, 24, 10], weapons="5 энергетических пушек, 2 ракетные установки", crew=6, engines=4, scale=3.8, tint=[0.95, 0.85, 0.75], flame=[1.0, 0.7, 0.3], skins=["a"], extras=[]),
  dict(id="DST-001", dir="dst001", name="Молот", cls="Эсминец", role="Ракетные удары", specs={"striker": ("Ударный", "Тяжёлое вооружение и ракетные залпы (ракеты появятся с боевым этапом)"), "siege": ("Осадный", "Дальнобойные орудия")},
       default="striker", hull=9000, shield=3500, armor=600, energy=300, cargo=50, speed=7.0, regen=(34, 6, 13), pulse=(70, 4, 40, 1.4, 5), boost=(1.3, 3, 22, 45), dims=[80, 32, 14], weapons="8 тяжёлых орудий, 4 ракетные установки", crew=12, engines=4, scale=4.4, tint=[1.0, 0.8, 0.65], flame=[1.0, 0.5, 0.2], skins=["a"], extras=[]),
  dict(id="CRU-001", dir="cru001", name="Колосс", cls="Крейсер", role="Командный", specs={"flagship": ("Флагман", "Командная роль и живучесть"), "bombard": ("Бомбардир", "Тяжёлые системы вооружения")},
       default="flagship", hull=16000, shield=6000, armor=900, energy=450, cargo=90, speed=6.0, regen=(45, 7, 15), pulse=(95, 4, 44, 1.5, 6), boost=(1.25, 3, 25, 55), dims=[140, 48, 22], weapons="12 тяжёлых орудий, 6 ракетных установок", crew=18, engines=6, scale=5.2, tint=[0.7, 0.75, 0.95], flame=[0.5, 0.75, 1.0], skins=["a"], extras=[]),
  dict(id="SCT-001", dir="sct001", name="Следопыт", cls="Разведчик", role="Разведка", specs={"scout": ("Разведчик", "Широкий импульсный сканер"), "navigator": ("Навигатор", "Скорость и дальность перелётов")},
       default="scout", hull=900, shield=500, armor=80, energy=160, cargo=12, speed=16.0, regen=(14, 3, 12), pulse=(18, 2, 26, 1.0, 2), boost=(1.9, 4, 12, 20), dims=[9, 6.5, 2.2], weapons="1 лёгкая пушка", crew=1, engines=2, scale=2.0, tint=[0.7, 1.0, 1.0], flame=[0.3, 1.0, 0.9], skins=["a"], extras=["wide_scan"]),
  dict(id="SUP-001", dir="sup001", name="Эгида", cls="Корабль поддержки", role="Поддержка", specs={"shield_support": ("Щитовая поддержка", "Усиленные щиты (поддержка союзников появится с боевым этапом)"), "medic": ("Ремонтник", "Живучесть (ремонт союзников появится с боевым этапом)")},
       default="shield_support", hull=3200, shield=2400, armor=260, energy=260, cargo=30, speed=9.5, regen=(40, 4, 14), pulse=(26, 2, 28, 1.2, 3), boost=(1.4, 3, 16, 30), dims=[36, 18, 8], weapons="2 оборонительные пушки", crew=4, engines=3, scale=3.2, tint=[1.0, 0.95, 0.7], flame=[1.0, 0.9, 0.4], skins=["a"], extras=[]),
]
STRIZ = json.load(open("server/internal/world/data/ships.json"))["ships"][0]
POINTS = json.load(open("client/assets/ships/tmp_haul/ship.json"))["points"]

def passport(s):
    ab = {"pulse": {"name": "Импульсные орудия", "damage": s["pulse"][0], "shots": s["pulse"][1], "range": s["pulse"][2], "period": s["pulse"][3], "energy": s["pulse"][4]},
          "boost": {"name": "Форсаж", "speed_mul": s["boost"][0], "duration": s["boost"][1], "cooldown": s["boost"][2], "energy": s["boost"][3]}}
    for x in s["extras"]:
        if x == "intercept":
            a = copy.deepcopy(STRIZ["abilities"]["intercept"]); ab["intercept"] = a
        if x == "scan":
            ab["scan"] = copy.deepcopy(STRIZ["abilities"]["scan"])
        if x == "wide_scan":
            ab["scan"] = {"name": "Широкий сканер", "spec": "scout", "radius": 95, "reveal": 20, "cooldown": 22, "energy": 30}
    return {"id": s["id"], "name": s["name"], "class": s["cls"], "role": s["role"], "temporary": True, "specs": list(s["specs"]), "default_spec": s["default"],
            "skins": s["skins"], "dimensions_m": s["dims"], "crew": s["crew"], "engines": s["engines"], "weapons": s["weapons"],
            "hull": s["hull"], "shield": s["shield"], "armor": s["armor"], "energy": s["energy"], "cargo": s["cargo"], "speed": s["speed"],
            "shield_regen": s["regen"][0], "shield_regen_delay": s["regen"][1], "energy_regen": s["regen"][2], "abilities": ab}

def client(s):
    skins = {}
    for k in s["skins"]:
        skins[k] = {"name": "Временная окраска" + (" B" if k == "b" else ""), "glb": "res://assets/ships/ftr001/striz_%s.glb" % ("b" if k == "b" else "a"), "flame": s["flame"], "tint": s["tint"]}
    return {"id": s["id"], "name": s["name"], "class": s["cls"], "role": s["role"], "temporary": True, "specializations": [v[0] for v in s["specs"].values()],
            "dimensions_m": s["dims"], "crew": s["crew"], "engines": s["engines"], "weapons": s["weapons"], "model_length": 1.9, "visual_scale": s["scale"], "nose_axis": "-x",
            "skins": skins, "points": POINTS, "specs": {k: {"name": v[0], "ability": v[1]} for k, v in s["specs"].items()}}

path = "server/internal/world/data/ships.json"
doc = json.load(open(path))
have = {x["id"]: i for i, x in enumerate(doc["ships"])}
for s in SHIPS:
    p = passport(s)
    if s["id"] in have: doc["ships"][have[s["id"]]] = p
    else: doc["ships"].append(p)
    os.makedirs("client/assets/ships/" + s["dir"], exist_ok=True)
    json.dump(client(s), open("client/assets/ships/%s/ship.json" % s["dir"], "w"), ensure_ascii=False, indent=2)
json.dump(doc, open(path, "w"), ensure_ascii=False, indent=2)
print("кораблей в каталоге:", len(doc["ships"]))
