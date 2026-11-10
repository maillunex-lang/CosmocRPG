# Зависимости (генерируется tools/gen_roadmap.py — не править вручную)

Только рёбра между задачами `tasks.json`. Зависимости от внешних реестров (SR-*, R-*) указаны в карточках задач.

| Задача | Зависит от |
|---|---|
| PROTO-05B | PROTO-12 |
| K-1B | K-1 |
| K-2 | PROTO-03 |
| K-2 | K-1B |
| SR-100C | SR-100B (внешний реестр) |
| SR-100D | SR-100C |
| SR-012 | SR-100D |
| SR-013 | SR-012 |
| SR-014 | SR-013 |
| SR-020 | SR-012 |
| SR-030 | SR-020 |
| SR-040 | SR-014 |
| SR-040 | SR-030 |
| SR-050 | SR-030 |
| SR-050 | SR-040 |
| SR-060 | SR-020 |
| SR-060 | SR-040 |
| SR-070 | SR-050 |
| SR-070 | SR-060 |
| EB-INTEGRATION | EB-FOUNDATION |
| PP-02 | PP-01 |
| PP-03 | PP-02 |
| PP-04 | PP-02 |
| PP-05 | PP-04 |
| PP-06 | PP-03 |
| PP-07 | SR-100C |
| PP-08 | PP-04 |
| PP-08 | PP-06 |
| PP-08 | PP-07 |
| AIC-020 | AIC-010 |
| AIC-030 | AIC-020 |
| AIC-040 | AIC-010 |
| AIC-050 | AIC-040 |
| AIC-060 | AIC-020 |
| AIC-060 | AIC-050 |
| AIC-070 | AIC-050 |
| AIC-070 | AIC-060 |
| AIC-080 | AIC-030 |
| AIC-100 | AIC-030 |
| AIC-100 | AIC-070 |
| AIC-100 | AIC-080 |
| AIC-021 | AIC-010 |
| AIC-022 | AIC-021 |
| AIC-023 | AIC-021 |
| BETA-ECO-001 | PROTO-08 |
| AIC-025 | BETA-ECO-001 |
| AIC-025 | BETA-ECO-003 |
| AIC-025 | BETA-ECO-004 |
| BETA-ECO-003 | BETA-ECO-001 |
| BETA-ECO-004 | BETA-ECO-001 |
| BETA-ECO-004 | BETA-ECO-002 |
| AIC-026 | AIC-022 |
| AIC-026 | BETA-ECO-001 |
