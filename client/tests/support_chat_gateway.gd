extends "res://ui/gateway/demo_gateway.gd"
## Тестовый шлюз с работающим чатом (как серверный): сообщения возвращаются «от сервера» сразу.

var sent: Array = []
var history_requests := 0


func has_chat() -> bool:
	return true


func own_id() -> String:
	return "me"


func chat_send(channel: String, to: String, text: String) -> void:
	sent.append({"channel": channel, "to": to, "text": text})
	chat_received.emit({"channel": channel, "from_id": "me", "from_name": "Тестовый", "to_name": to, "text": text, "at": int(Time.get_unix_time_from_system() * 1000.0)})


func request_chat_history() -> void:
	history_requests += 1
