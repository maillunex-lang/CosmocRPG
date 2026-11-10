package social

import (
	"context"
	"testing"
)

func TestFriendsAddListRemove(t *testing.T) {
	mem := NewMem()
	mem.AddPerson("a", "Ковбой")
	mem.AddPerson("b", "Вега")
	s := New(mem)
	ctx := context.Background()
	if _, err := s.Add(ctx, "a", "a"); err != ErrSelf {
		t.Fatalf("себя: %v", err)
	}
	if _, err := s.Add(ctx, "a", "нет"); err != ErrNone {
		t.Fatalf("нет такого: %v", err)
	}
	list, err := s.Add(ctx, "a", "вега")
	if err != nil || len(list) != 1 || list[0].ID != "b" {
		t.Fatalf("добавление: %+v %v", list, err)
	}
	list, err = s.Add(ctx, "a", "b")
	if err != nil || len(list) != 1 {
		t.Fatalf("повтор: %+v %v", list, err)
	}
	list, err = s.Remove(ctx, "a", "b")
	if err != nil || len(list) != 0 {
		t.Fatalf("удаление: %+v %v", list, err)
	}
}
