package quest

import "context"

// Choice — ответ игрока. Next «done» закрывает разговор и засчитывает задание.
type Choice struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Next string `json:"-"`
}

// Node — реплика NPC и доступные ответы.
type Node struct {
	Speaker string
	Text    string
	Choices []Choice
}

// Scene — короткий разговор одного задания. Это первая игровая сцена, не вся кампания.
type Scene struct {
	Start   string
	Closing string
	Nodes   map[string]Node
}

// Dialogue возвращает сцену задания, если разговор для него написан.
func Dialogue(questID string) (Scene, bool) {
	switch questID {
	case "meridian_voice":
		return Scene{
			Start:   "open",
			Closing: "Тогда ворота на Красный Шторм. Обломки у первого поля помечены как мусор — не верьте этой пометке.",
			Nodes: map[string]Node{
				"open": {
					Speaker: "Вора Кесс",
					Text:    "Доклад приняла. След аномалии не кончается в Меридиане: такой же уходит к воротам Красного Шторма. Караваны там уже пропадали.",
					Choices: []Choice{
						{ID: "ask", Text: "Кто пропадает?", Next: "who"},
						{ID: "go", Text: "Полечу к обломкам.", Next: "done"},
					},
				},
				"who": {
					Speaker: "Вора Кесс",
					Text:    "Свободные системы списывают это на пиратов. Я бы сначала осмотрела обломки у поля, а не совалась в логово одна.",
					Choices: []Choice{
						{ID: "go", Text: "Полечу к обломкам.", Next: "done"},
					},
				},
			},
		}, true
	case "storm_dispatch":
		return Scene{
			Start:   "open",
			Closing: "Контур на карте — аномалия. Снимите служебные метки и вернитесь. Корабль «Арго» там не ищите.",
			Nodes: map[string]Node{
				"open": {
					Speaker: "Кира Роан",
					Text:    "Контейнер у поля — чужой транспорт, не «Арго». Дальше есть старый контур. На карте он числится аномалией.",
					Choices: []Choice{
						{ID: "danger", Text: "Насколько опасен сектор?", Next: "danger"},
						{ID: "go", Text: "Полечу к контуру.", Next: "done"},
					},
				},
				"danger": {
					Speaker: "Кира Роан",
					Text:    "Пираты спрашивают, что можно забрать, а не кто спонсировал экспедицию. К контуру можно подойти, не лезя в логово.",
					Choices: []Choice{
						{ID: "go", Text: "Полечу к контуру.", Next: "done"},
					},
				},
			},
		}, true
	case "storm_report":
		return Scene{
			Start:   "open",
			Closing: "Отчёт закрыт. Метаданные прошли через контур, второй импульс не объяснён. «Арго» по-прежнему не найден.",
			Nodes: map[string]Node{
				"open": {
					Speaker: "Вера Элл",
					Text:    "По индексу есть совпадение: через контур шли метаданные, не обязательно весь пакет. Второй импульс есть, и его время не сходится с журналом.",
					Choices: []Choice{
						{ID: "ship", Text: "Значит, «Арго» был здесь?", Next: "ship"},
						{ID: "who", Text: "Кто отправил импульс?", Next: "who"},
						{ID: "close", Text: "Закрываю отчёт.", Next: "done"},
					},
				},
				"ship": {
					Speaker: "Вера Элл",
					Text:    "Нет. Это след данных, не корабля. Где «Арго» и жив ли экипаж, отсюда не следует.",
					Choices: []Choice{
						{ID: "who", Text: "А импульс?", Next: "who"},
						{ID: "close", Text: "Закрываю отчёт.", Next: "done"},
					},
				},
				"who": {
					Speaker: "Вера Элл",
					Text:    "Отправитель не восстановлен. Это не доказательство разумного сигнала и не вина рубежа.",
					Choices: []Choice{
						{ID: "close", Text: "Закрываю отчёт.", Next: "done"},
					},
				},
			},
		}, true
	default:
		return Scene{}, false
	}
}

// Reply — то, что клиент показывает в окне разговора.
type Reply struct {
	QuestID string   `json:"quest_id"`
	Speaker string   `json:"speaker"`
	Text    string   `json:"text"`
	Choices []Choice `json:"choices"`
	Done    bool     `json:"done"`
}

// Talk открывает или продвигает разговор. Пустой choice только показывает реплику.
// Засчитывается задание только ответом, который ведёт в done, и только в системе задания.
func (s *Service) Talk(ctx context.Context, pid, system, choice string) (Reply, []Completion, error) {
	s.mu.Lock()
	ps := s.players[pid]
	if ps == nil {
		s.mu.Unlock()
		return Reply{}, nil, ErrNoDialogue
	}
	var d Def
	var found bool
	for _, q := range ps.data.Quests {
		if q.State != "active" {
			continue
		}
		def, ok := s.def(q.QuestID, q.Version)
		if ok && def.Kind == "talk" && (def.System == "" || def.System == system) {
			d = def
			found = true
			break
		}
	}
	nodeID := ps.line
	s.mu.Unlock()
	if !found {
		return Reply{}, nil, ErrNoDialogue
	}
	scene, ok := Dialogue(d.ID)
	if !ok {
		return Reply{}, nil, ErrNoDialogue
	}
	if nodeID == "" {
		nodeID = scene.Start
	}
	node, ok := scene.Nodes[nodeID]
	if !ok {
		nodeID = scene.Start
		node = scene.Nodes[nodeID]
	}
	if choice == "" {
		s.setLine(pid, nodeID)
		return replyOf(d.ID, node, false), nil, nil
	}
	next := ""
	matched := false
	for _, c := range node.Choices {
		if c.ID == choice {
			next = c.Next
			matched = true
			break
		}
	}
	if !matched || len(choice) > 32 {
		return Reply{}, nil, ErrBadChoice
	}
	if next == "done" {
		s.setLine(pid, "")
		done := s.EventAt(ctx, pid, system, "talk", d.Target, 1)
		return Reply{QuestID: d.ID, Speaker: node.Speaker, Text: scene.Closing, Choices: []Choice{}, Done: true}, done, nil
	}
	nxt, ok := scene.Nodes[next]
	if !ok {
		return Reply{}, nil, ErrBadChoice
	}
	s.setLine(pid, next)
	return replyOf(d.ID, nxt, false), nil, nil
}

func (s *Service) setLine(pid, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ps := s.players[pid]; ps != nil {
		ps.line = line
	}
}

func replyOf(questID string, node Node, done bool) Reply {
	choices := node.Choices
	if choices == nil {
		choices = []Choice{}
	}
	return Reply{QuestID: questID, Speaker: node.Speaker, Text: node.Text, Choices: choices, Done: done}
}
