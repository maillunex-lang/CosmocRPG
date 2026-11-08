extends RefCounted
## Передача шлюза данных между сценами (без автозагрузок). Экран подключения кладёт сюда шлюз
## перед открытием космоса; если его нет, сцена космоса создаёт явный демо-шлюз.

const UiGateway := preload("res://ui/gateway/ui_gateway.gd")

static var gateway: UiGateway = null
