package aiagent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
)

// fakeStudio отдаёт сгенерированную конфигурацию AI Studio (движок диалогов).
type fakeStudio struct {
	cfg aistudio.Config
	ver int64
}

func (f fakeStudio) Active(context.Context) (aistudio.Config, int64, error) { return f.cfg, f.ver, nil }

func studioCfg() aistudio.Config {
	c := aistudio.NewConfig()
	aistudio.Generate(&c, aistudio.GenerateOptions{Seed: 3, Count: 8})
	return c
}

// dialogueRig: мир с ИИ, чат в памяти и провайдер реплик.
type dialogueRig struct {
	rig   *rig
	chat  *chat.Service
	dlg   *Dialogue
	store *chat.MemStore
}

func newDialogueRig(t *testing.T, n int) *dialogueRig {
	t.Helper()
	r := newRig(t, n)
	st := chat.NewMemStore()
	cs := chat.NewService(st)
	cst := chat.DefaultSettings()
	cst.MinIntervalMs = 0
	_ = cs.UpdateSettings(context.Background(), cst)
	// как в main.go: адресатом личного сообщения может быть онлайн-капитан
	cs.Directory = func(_ context.Context, q string) (chat.Person, bool) {
		if q == "p1" {
			return chat.Person{ID: "p1", Name: "Пилот"}, true
		}
		c, ok := r.m.Lookup(q)
		if !ok {
			return chat.Person{}, false
		}
		return chat.Person{ID: c.ID, Name: c.Name, AI: true}, true
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewDialogue(fakeStudio{cfg: studioCfg(), ver: 1}, cs, r.m, log)
	d.now = func() time.Time { return r.now }
	d.rng = 12345
	return &dialogueRig{rig: r, chat: cs, dlg: d, store: st}
}

// smallSettings: мало ИИ, мгновенные ответы, вероятность 100%.
func smallSettings(m *Manager) Settings {
	s := DefaultSettings()
	s.MinOnline, s.MaxOnline = 2, 2
	s.ChatReplyChance = 100
	s.ChatDelayMinMs, s.ChatDelayMaxMs = 0, 0
	s.ChatCaptainCooldownSec = 0
	_ = m.SetSettings(context.Background(), s, "test")
	return s
}

// waitAI ждёт ИИ-сообщение в канале (dmTo != "" — личное указанному игроку).
func waitAI(t *testing.T, r *dialogueRig, dmTo string) chat.Message {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if dmTo != "" {
			if msgs, err := r.store.DMs(context.Background(), dmTo, 20); err == nil {
				for _, m := range msgs {
					if m.FromAI {
						return m
					}
				}
			}
		} else if msgs, err := r.store.General(context.Background(), 20); err == nil {
			for _, m := range msgs {
				if m.FromAI {
					return m
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("ИИ не ответил вовремя")
	return chat.Message{}
}

func TestDialogueDMReply(t *testing.T) {
	r := newDialogueRig(t, 5)
	smallSettings(r.rig.m)
	r.rig.join("p1")
	r.rig.tick(1)
	r.rig.m.mu.Lock()
	captain := ""
	for id := range r.rig.m.online {
		captain = id
		break
	}
	r.rig.m.mu.Unlock()
	if captain == "" {
		t.Fatal("капитан не появился")
	}
	msg, err := r.chat.Send(context.Background(), chat.Sender{ID: "p1", Name: "Пилот"}, chat.DM, captain, "привет")
	if err != nil {
		t.Fatal(err)
	}
	r.dlg.Handle(context.Background(), msg)
	ai := waitAI(t, r, "p1")
	if ai.FromID != captain || ai.ToID != "p1" || ai.Text == "" {
		t.Fatalf("плохой ответ: %+v", ai)
	}
	// ИИ помечен флагом и попал в историю личных сообщений
	if !ai.FromAI {
		t.Fatal("реплика ИИ не помечена FromAI")
	}

}

func TestDialogueGeneralReply(t *testing.T) {
	r := newDialogueRig(t, 5)
	smallSettings(r.rig.m)
	r.rig.join("p1")
	r.rig.tick(1)
	msg, err := r.chat.Send(context.Background(), chat.Sender{ID: "p1", Name: "Пилот"}, chat.General, "", "как дела, космос?")
	if err != nil {
		t.Fatal(err)
	}
	r.dlg.Handle(context.Background(), msg)
	ai := waitAI(t, r, "")
	if ai.Channel != chat.General || ai.Text == "" {
		t.Fatalf("плохой общий ответ: %+v", ai)
	}
}

func TestDialogueRateLimit(t *testing.T) {
	r := newDialogueRig(t, 5)
	s := smallSettings(r.rig.m)
	s.ChatMaxPerMin = 1
	_ = r.rig.m.SetSettings(context.Background(), s, "test")
	r.rig.join("p1")
	r.rig.tick(1)
	// два сообщения подряд: вторая реплика обрезана потолком и общим интервалом
	for i := 0; i < 2; i++ {
		msg, err := r.chat.Send(context.Background(), chat.Sender{ID: "p1", Name: "Пилот"}, chat.General, "", fmt.Sprintf("привет все %d", i))
		if err != nil {
			t.Fatal(err)
		}
		r.dlg.Handle(context.Background(), msg)
	}
	first := waitAI(t, r, "")
	_ = first
	deadline := time.Now().Add(700 * time.Millisecond)
	n := 0
	for time.Now().Before(deadline) {
		if msgs, err := r.store.General(context.Background(), 50); err == nil {
			n = 0
			for _, m := range msgs {
				if m.FromAI {
					n++
				}
			}
		}
		if n >= 2 {
			t.Fatalf("потолок реплик нарушен: %d", n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDialogueNoReplyToAIAndDisabled(t *testing.T) {
	r := newDialogueRig(t, 5)
	s := smallSettings(r.rig.m)
	// на своё сообщение ИИ не отвечает
	aiMsg := chat.Message{Channel: chat.General, FromID: "cpt-0001", FromName: "Ник1", FromAI: true, Text: "хай"}
	r.dlg.Handle(context.Background(), aiMsg)
	if n := countAI(r); n != 0 {
		t.Fatalf("ответ на сообщение ИИ: %d", n)
	}
	// отключённый капитан не ставится в мир
	s.DisabledCaptains = []string{"cpt-0001", "cpt-0002", "cpt-0003"}
	_ = r.rig.m.SetSettings(context.Background(), s, "test")
	r.rig.join("p1")
	r.rig.tick(2)
	online := r.rig.m.Status().Online
	if online > 2 {
		t.Fatalf("отключённые капитаны в мире: онлайн %d", online)
	}
	for _, c := range r.rig.m.Captains() {
		for _, off := range s.DisabledCaptains {
			if c.ID == off && !c.Since.IsZero() {
				t.Fatalf("отключённый капитан %s онлайн", off)
			}
		}
	}
}

func TestDialogueAntiRepeat(t *testing.T) {
	r := newDialogueRig(t, 5)
	smallSettings(r.rig.m)
	r.dlg.note("cpt-0001", "уникальная реплика номер раз")
	if !r.dlg.repeats("уникальная реплика номер раз") {
		t.Fatal("повтор не распознан")
	}
	if r.dlg.repeats("другая реплика совсем") {
		t.Fatal("новая реплика принята за повтор")
	}
	// кольцо ограничено: старое вытесняется
	for i := 0; i < 30; i++ {
		r.dlg.note("cpt-0002", "реплика-"+string(rune('a'+i%26)))
	}
	if r.dlg.repeats("уникальная реплика номер раз") {
		t.Fatal("старое не вытеснено из кольца")
	}
}

func countAI(r *dialogueRig) int {
	n := 0
	if msgs, err := r.store.General(context.Background(), 50); err == nil {
		for _, m := range msgs {
			if m.FromAI {
				n++
			}
		}
	}
	return n
}
