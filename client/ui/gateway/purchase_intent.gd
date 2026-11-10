extends RefCounted
## Один незавершённый intent на endpoint/пилота. Не содержит токенов и денег.
## После перезахода проверяется статус; сам по себе файл не запускает покупку.

var _path := ""
var _pending: Dictionary = {}
var _ready := false

func configure(endpoint: String, owner: String) -> bool:
	var next_path := "user://pending_purchase_" + (endpoint + "|" + owner).sha256_text() + ".json"
	if next_path == _path and _ready:
		return true
	_path = next_path
	_pending = {}
	_ready = false
	if owner.is_empty():
		return false
	if FileAccess.file_exists(_path):
		var file := FileAccess.open(_path, FileAccess.READ)
		if file == null:
			return false
		if file.get_length() > 2048:
			file.close()
			return false
		var data: Variant = JSON.parse_string(file.get_as_text())
		file.close()
		if not data is Dictionary or not _valid(data):
			return false
		_pending = data
	_ready = true
	return true

func _valid(data: Dictionary) -> bool:
	if not data.get("resource") is String or not data.get("operation_id") is String:
		return false
	if typeof(data.get("amount")) not in [TYPE_INT, TYPE_FLOAT]:
		return false
	var amount := float(data["amount"])
	var resource := str(data["resource"])
	var key := str(data["operation_id"])
	return is_finite(amount) and amount > 0.0 and amount <= 1000.0 and not resource.is_empty() and resource.to_utf8_buffer().size() <= 24 and key.length() == 32 and key.is_valid_hex_number()

func pending() -> Dictionary:
	return _pending.duplicate(true)

func prepare(resource: String, amount: float) -> Dictionary:
	if not _ready:
		return {}
	if not _pending.is_empty():
		if _pending["resource"] == resource and float(_pending["amount"]) == amount:
			return pending()
		return {}
	var bytes := Crypto.new().generate_random_bytes(16)
	if bytes.size() != 16:
		return {}
	var data := {"resource": resource, "amount": amount, "operation_id": bytes.hex_encode()}
	if not _valid(data):
		return {}
	var temp := _path + ".tmp"
	var file := FileAccess.open(temp, FileAccess.WRITE)
	if file == null:
		return {}
	file.store_string(JSON.stringify(data))
	file.flush()
	var err := file.get_error()
	file.close()
	if err != OK:
		return {}
	if DirAccess.rename_absolute(ProjectSettings.globalize_path(temp), ProjectSettings.globalize_path(_path)) != OK:
		return {}
	_pending = data
	return pending()

func acknowledge(key: String) -> bool:
	if _pending.is_empty() or key != str(_pending.get("operation_id", "")):
		return false
	if FileAccess.file_exists(_path) and DirAccess.remove_absolute(ProjectSettings.globalize_path(_path)) != OK:
		return false
	_pending = {}
	return true
