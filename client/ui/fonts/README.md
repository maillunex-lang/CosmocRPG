# Шрифты

**Golos Text** (вариативный, ось веса 400–900), автор: Alexandra Korolkova, Vitaly Kuzmin (ParaType).
Лицензия: SIL Open Font License 1.1, текст в `OFL.txt` (копирайт и лицензию нужно сохранять при распространении).
Источник: https://github.com/google/fonts/tree/main/ofl/golostext (файл `GolosText[wght].ttf`).
Поддерживает кириллицу. Используется как основной шрифт интерфейса; насыщенность задаётся через `FontVariation` (`wght`).

## Вес в коде

Ключи `FontVariation.variation_opentype` в коде — числовые теги оси, не строки (строка `"wght"` молча игнорируется):

```gdscript
var tag := TextServerManager.get_primary_interface().name_to_tag("wght")
fv.variation_opentype = {tag: 700}
```
