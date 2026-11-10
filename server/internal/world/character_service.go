package world

import "context"

// AIServiceTokenPrefix — зарезервированный namespace внутренних AI-сессий.
const AIServiceTokenPrefix = "aiciv2_"

// StarterSource — текущий authoritative каталог, общий для людей и AI.
type StarterSource interface{ Starter(string) (Starter, bool) }

// CreatePilot — единые правила первичного выбора персонажа. Хранилище остаётся
// ответственным за уникальность ника и однократное создание.
func CreatePilot(ctx context.Context, store Store, catalog StarterSource, id, nick, gender, pilot string) (Record, error) {
	clean, err := CleanNick(nick)
	if err != nil {
		return Record{}, err
	}
	if !ValidGender(gender) {
		return Record{}, ErrBadGender
	}
	starter, ok := catalog.Starter(pilot)
	if !ok {
		return Record{}, ErrBadPilotSpec
	}
	return store.CreateCharacter(ctx, id, clean, gender, pilot, starter)
}
