package rpg

import (
	"errors"
	"fmt"
)

// TalentEffect — вклад одного ранга таланта.
type TalentEffect struct {
	Stat    StatID  `json:"stat"`
	Op      Op      `json:"op"`
	PerRank float64 `json:"per_rank"`
}

// TalentNode — узел дерева талантов.
type TalentNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Branch   string         `json:"branch"`
	Tier     int            `json:"tier"` // ярус: узел выше всех своих требований
	Cost     int            `json:"cost"` // очков за ранг
	MaxRank  int            `json:"max_rank"`
	Requires []string       `json:"requires,omitempty"`
	Effects  []TalentEffect `json:"effects"`
}

// TalentTree — версионируемое дерево талантов (данные, не код).
type TalentTree struct {
	Version  int          `json:"version"`
	Branches []string     `json:"branches"`
	Nodes    []TalentNode `json:"nodes"`
}

// Validate проверяет дерево целиком и возвращает все найденные ошибки (пусто — дерево корректно).
func (t TalentTree) Validate() []error {
	var errs []error
	add := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }
	branches := map[string]bool{}
	for _, b := range t.Branches {
		if b == "" || branches[b] {
			add("пустая или повторная ветвь «%s»", b)
		}
		branches[b] = true
	}
	nodes := map[string]TalentNode{}
	for _, n := range t.Nodes {
		if n.ID == "" {
			add("узел без идентификатора")
			continue
		}
		if _, dup := nodes[n.ID]; dup {
			add("повторный идентификатор узла %s", n.ID)
			continue
		}
		nodes[n.ID] = n
	}
	for _, n := range t.Nodes {
		if n.ID == "" {
			continue
		}
		if !branches[n.Branch] {
			add("узел %s: неизвестная ветвь «%s»", n.ID, n.Branch)
		}
		if n.Cost < 1 || n.MaxRank < 1 || n.Tier < 1 {
			add("узел %s: стоимость, максимальный ранг и ярус должны быть не меньше 1", n.ID)
		}
		if len(n.Effects) == 0 {
			add("узел %s: нет эффектов", n.ID)
		}
		for _, e := range n.Effects {
			if !ValidStat(e.Stat) || !ValidOp(e.Op) {
				add("узел %s: неизвестная характеристика или операция", n.ID)
			}
		}
		for _, r := range n.Requires {
			req, ok := nodes[r]
			if !ok {
				add("узел %s: требуемый узел %s не существует", n.ID, r)
				continue
			}
			if r == n.ID || req.Tier >= n.Tier {
				add("узел %s: ярус должен быть выше яруса требования %s", n.ID, r)
			}
		}
	}
	if len(errs) == 0 {
		if cyc := t.findCycle(nodes); cyc != "" {
			errs = append(errs, errors.New("цикл в требованиях: "+cyc))
		}
	}
	return errs
}

// findCycle — защита от циклов, даже если ярусы заданы неверно.
func (t TalentTree) findCycle(nodes map[string]TalentNode) string {
	const (
		white = iota
		grey
		black
	)
	color := map[string]int{}
	var visit func(id string) string
	visit = func(id string) string {
		color[id] = grey
		for _, r := range nodes[id].Requires {
			switch color[r] {
			case grey:
				return id + " → " + r
			case white:
				if s := visit(r); s != "" {
					return s
				}
			}
		}
		color[id] = black
		return ""
	}
	for _, n := range t.Nodes {
		if color[n.ID] == white {
			if s := visit(n.ID); s != "" {
				return s
			}
		}
	}
	return ""
}
