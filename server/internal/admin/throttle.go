package admin

import (
	"sync"
	"time"
)

// throttle ограничивает число неудачных попыток входа по ключу (IP или имя пользователя).
type throttle struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string][]time.Time
	maxKeys  int
}

func newThrottle(max int, window time.Duration) *throttle {
	return &throttle{max: max, window: window, failures: map[string][]time.Time{}, maxKeys: 10000}
}

// blocked сообщает, исчерпан ли лимит для ключа.
func (t *throttle) blocked(key string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.prune(key, now)) >= t.max
}

// fail фиксирует неудачную попытку.
func (t *throttle) fail(key string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.failures) >= t.maxKeys {
		// защита памяти от перебора ключей: убираем устаревшие, при нехватке — всё
		for k := range t.failures {
			t.prune(k, now)
		}
		if len(t.failures) >= t.maxKeys {
			t.failures = map[string][]time.Time{}
		}
	}
	t.failures[key] = append(t.prune(key, now), now)
}

// reset сбрасывает счётчик после успешного входа.
func (t *throttle) reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, key)
}

// prune убирает устаревшие попытки; вызывать под блокировкой.
func (t *throttle) prune(key string, now time.Time) []time.Time {
	list := t.failures[key]
	i := 0
	for i < len(list) && now.Sub(list[i]) >= t.window {
		i++
	}
	list = list[i:]
	if len(list) == 0 {
		delete(t.failures, key)
		return nil
	}
	t.failures[key] = list
	return list
}
