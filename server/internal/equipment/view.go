package equipment

import (
	"sort"

	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

// ItemView — модуль, который можно показать в ангаре. Характеристики считает каталог, не клиент.
type ItemView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Prototype   string `json:"prototype"`
	Engineering int    `json:"engineering,omitempty"`
}

// SlotView — слот корпуса. Locked: штатный двигатель Стрижа меняется прежней кнопкой.
type SlotView struct {
	ID     string    `json:"id"`
	Item   *ItemView `json:"item,omitempty"`
	Locked bool      `json:"locked"`
}

// View — инвентарь модулей одного корпуса. Ресурсы трюма сюда не входят.
type View struct {
	Hull          string     `json:"hull"`
	Revision      int64      `json:"revision"`
	PilotRevision int64      `json:"pilot_revision"`
	Mass          float64    `json:"mass_tons"`
	MassBudget    float64    `json:"mass_budget_tons"`
	PowerDraw     float64    `json:"power_draw_mw"`
	PowerCapacity float64    `json:"power_capacity_mw"`
	Slots         []SlotView `json:"slots"`
	Bag           []ItemView `json:"bag"`
	CanChange     bool       `json:"can_change"`
	Reason        string     `json:"reason,omitempty"`
}

func PublicView(c *fitting.Catalog, s fitting.State) View {
	out := View{Hull: s.Hull, Revision: s.Revision, PilotRevision: s.Pilot.Revision, Slots: []SlotView{}, Bag: []ItemView{}}
	if c == nil {
		return out
	}
	if res, err := c.ValidateState(s); err == nil {
		out.Mass, out.MassBudget = res.Mass, res.MassBudget
		out.PowerDraw, out.PowerCapacity = res.PowerDraw, res.PowerCapacity
	}
	fitted := map[string]string{}
	h, ok := c.Hull(s.Hull)
	if !ok {
		return out
	}
	for _, slot := range h.Slots {
		row := SlotView{ID: slot.ID, Locked: s.Hull == fitting.ShipStrizh && slot.ID == "tech-1"}
		if id := s.Slots[slot.ID]; id != "" {
			if item, known := s.Items[id]; known {
				view := itemView(c, item)
				row.Item = &view
				fitted[id] = slot.ID
			}
		}
		out.Slots = append(out.Slots, row)
	}
	for id, item := range s.Items {
		if fitted[id] != "" || item.Owner != s.Owner {
			continue
		}
		out.Bag = append(out.Bag, itemView(c, item))
	}
	sort.Slice(out.Bag, func(i, j int) bool { return out.Bag[i].Name < out.Bag[j].Name })
	return out
}

// ModuleCategories — категории надетых модулей в порядке слотов корпуса.
func ModuleCategories(c *fitting.Catalog, s fitting.State) []string {
	if c == nil {
		return nil
	}
	h, ok := c.Hull(s.Hull)
	if !ok {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, slot := range h.Slots {
		id := s.Slots[slot.ID]
		if id == "" {
			continue
		}
		item, ok := s.Items[id]
		if !ok {
			continue
		}
		m, ok := c.Module(item.Prototype)
		if !ok || m.Category == "" || seen[m.Category] {
			continue
		}
		seen[m.Category] = true
		out = append(out, m.Category)
	}
	return out
}

func itemView(c *fitting.Catalog, item fitting.Item) ItemView {
	name, category := item.Prototype, ""
	if m, ok := c.Module(item.Prototype); ok {
		name, category = m.Name, m.Category
	}
	return ItemView{ID: item.ID, Name: name, Category: category, Prototype: item.Prototype, Engineering: item.Engineering}
}
