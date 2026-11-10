package quest

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed data/*.json
var dataFS embed.FS

type rawDef struct {
	ID      string  `json:"id"`
	Version int     `json:"version"`
	Title   string  `json:"title"`
	Text    string  `json:"text"`
	Kind    string  `json:"kind"`
	Target  string  `json:"target"`
	System  string  `json:"system"`
	Marker  string  `json:"marker"`
	Radius  float64 `json:"radius"`
	Goal    int     `json:"goal"`
	After   string  `json:"after"`
	OrAfter string  `json:"or_after"`
	Reward  struct {
		Credits int64 `json:"credits"`
	} `json:"reward"`
}

type rawChoice struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Next string `json:"next"`
}

type rawNode struct {
	Speaker string      `json:"speaker"`
	Text    string      `json:"text"`
	Choices []rawChoice `json:"choices"`
}

type rawScene struct {
	Start   string             `json:"start"`
	Closing string             `json:"closing"`
	Skip    string             `json:"skip"`
	Nodes   map[string]rawNode `json:"nodes"`
}

var (
	loadOnce     sync.Once
	loadedDefs   []Def
	loadedScenes map[string]Scene
	loadErr      error
)

func loadData() {
	loadOnce.Do(func() {
		b, err := dataFS.ReadFile("data/quests.json")
		if err != nil {
			loadErr = fmt.Errorf("quests.json: %w", err)
			return
		}
		var raws []rawDef
		if err := json.Unmarshal(b, &raws); err != nil {
			loadErr = fmt.Errorf("quests.json parse: %w", err)
			return
		}
		defs := make([]Def, 0, len(raws))
		seen := map[string]bool{}
		for _, r := range raws {
			if r.ID == "" || r.Title == "" || r.Kind == "" || r.Goal <= 0 {
				loadErr = fmt.Errorf("bad quest def: %+v", r)
				return
			}
			if seen[r.ID] {
				loadErr = fmt.Errorf("duplicate quest id %s", r.ID)
				return
			}
			seen[r.ID] = true
			defs = append(defs, Def{
				ID: r.ID, Version: r.Version, Title: r.Title, Text: r.Text,
				Kind: r.Kind, Target: r.Target, System: r.System, Marker: r.Marker,
				Radius: r.Radius, Goal: r.Goal, After: r.After, OrAfter: r.OrAfter,
				Reward: Reward{Credits: r.Reward.Credits},
			})
		}
		ids := map[string]bool{}
		for _, d := range defs {
			ids[d.ID] = true
		}
		for _, d := range defs {
			if d.After != "" && !ids[d.After] {
				loadErr = fmt.Errorf("quest %s after unknown %s", d.ID, d.After)
				return
			}
			if d.OrAfter != "" && !ids[d.OrAfter] {
				loadErr = fmt.Errorf("quest %s or_after unknown %s", d.ID, d.OrAfter)
				return
			}
		}
		loadedDefs = defs

		b, err = dataFS.ReadFile("data/dialogues.json")
		if err != nil {
			loadErr = fmt.Errorf("dialogues.json: %w", err)
			return
		}
		var rawScenes map[string]rawScene
		if err := json.Unmarshal(b, &rawScenes); err != nil {
			loadErr = fmt.Errorf("dialogues.json parse: %w", err)
			return
		}
		scenes := map[string]Scene{}
		for qid, rs := range rawScenes {
			nodes := map[string]Node{}
			for nid, rn := range rs.Nodes {
				chs := make([]Choice, len(rn.Choices))
				for i, c := range rn.Choices {
					chs[i] = Choice{ID: c.ID, Text: c.Text, Next: c.Next}
				}
				nodes[nid] = Node{Speaker: rn.Speaker, Text: rn.Text, Choices: chs}
			}
			for _, node := range nodes {
				for _, c := range node.Choices {
					if c.ID == "skip" {
						loadErr = fmt.Errorf("dialogue %s uses reserved choice skip", qid)
						return
					}
				}
			}
			scenes[qid] = Scene{Start: rs.Start, Closing: rs.Closing, Skip: rs.Skip, Nodes: nodes}
		}
		loadedScenes = scenes
		for _, d := range defs {
			if d.Kind == "talk" {
				if _, ok := scenes[d.ID]; !ok {
					loadErr = fmt.Errorf("talk quest %s has no dialogue", d.ID)
					return
				}
			}
		}
	})
}

// Catalog возвращает определения из data/quests.json (go:embed).
// При ошибке загрузки падает с panic — лучше сразу, чем молча работать со старым хардкодом.
func Catalog() []Def {
	loadData()
	if loadErr != nil {
		panic(loadErr)
	}
	out := make([]Def, len(loadedDefs))
	copy(out, loadedDefs)
	return out
}

// sceneFor возвращает сцену диалога из data/dialogues.json.
func sceneFor(questID string) (Scene, bool) {
	loadData()
	if loadErr != nil {
		panic(loadErr)
	}
	s, ok := loadedScenes[questID]
	return s, ok
}
