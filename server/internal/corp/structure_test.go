package corp

import (
	"context"
	"testing"
)

func TestAllianceOfThreeAndClanWork(t *testing.T) {
	ctx := context.Background()
	s := NewService(NewMemStore(), nil)
	a, err := s.Create(ctx, "p1", "Северный Дозор", "NOR")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "p2", "Ржавый Якорь", "RST"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "p3", "Тихая Стая", "QUI"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join(ctx, "p4", "NOR", "Четвертый"); err != nil {
		t.Fatal(err)
	}
	al, err := s.FoundAlliance(ctx, "p1", "NOR Союз")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.JoinAlliance(ctx, "p2", al.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := s.JoinAlliance(ctx, "p3", al.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "p5", "Лишний Борт", "EXT"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.JoinAlliance(ctx, "p5", al.Name); err != ErrAllianceFull {
		t.Fatalf("четвёртый клан: %v", err)
	}
	if _, err := s.Vote(ctx, "p4", "p1"); err != ErrNotOwner {
		t.Fatalf("член без права голоса: %v", err)
	}
	if _, err := s.Vote(ctx, "p1", "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Vote(ctx, "p2", "p1"); err != nil {
		t.Fatal(err)
	}
	got, _, _, err := mustStore(s).AllianceByCorp(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Governor != "p1" || got.Clans != 3 {
		t.Fatalf("союз: %+v", got)
	}
	for i := 0; i < TaskGoal; i++ {
		if _, err := s.NoteWork(ctx, "p4", 1); err != nil {
			t.Fatal(err)
		}
	}
	info, _, err := s.Info(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Treasury != TaskReward {
		t.Fatalf("казна %d", info.Treasury)
	}
}

func TestAIBuildsClansAndAlliance(t *testing.T) {
	ctx := context.Background()
	s := NewService(NewMemStore(), nil)
	ids := []string{}
	for i := 0; i < 40; i++ {
		id := string(rune('a'+i%26)) + string(rune('a'+i/26)) + "pilot"
		ids = append(ids, id)
		s.SettleAI(ctx, id, id)
	}
	for _, id := range ids {
		s.SettleAI(ctx, id, id)
	}
	clans, err := mustStore(s).ListClans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(clans) == 0 {
		t.Fatal("кланы не созданы")
	}
	allied := 0
	for _, clan := range clans {
		_, seats, has, err := mustStore(s).AllianceByCorp(ctx, clan.CorpID)
		if err != nil {
			t.Fatal(err)
		}
		if has {
			allied++
			if len(seats) > MaxAllianceClans {
				t.Fatalf("в союзе %d кланов", len(seats))
			}
		}
	}
	if allied == 0 {
		t.Fatal("ни один клан не вошёл в союз")
	}
}

func mustStore(s *Service) StructureStore {
	st, ok := s.structure()
	if !ok {
		panic("no structure")
	}
	return st
}
