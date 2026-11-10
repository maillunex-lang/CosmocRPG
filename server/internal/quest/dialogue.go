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

// Dialogue возвращает сцену задания из data/dialogues.json.
func Dialogue(questID string) (Scene, bool) {
	return sceneFor(questID)
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
