package corp

import (
	"context"
	"errors"
	"strings"
)

const (
	MaxAllianceClans = 3
	TaskGoal         = 10
	TaskReward       = 500
)

var (
	ErrNotOwner     = errors.New("not_owner")
	ErrAllianceFull = errors.New("alliance_full")
	ErrHasAlliance  = errors.New("has_alliance")
	ErrNoAlliance   = errors.New("no_alliance")
	ErrBadVote      = errors.New("bad_vote")
)

// Alliance — объединение не больше чем из трёх кланов.
type Alliance struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Governor     string `json:"governor"`
	GovernorName string `json:"governor_name,omitempty"`
	Clans        int    `json:"clans"`
}

// Seat — клан внутри союза.
type Seat struct {
	CorpID    string `json:"corp_id"`
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	Owner     string `json:"owner"`
	OwnerName string `json:"owner_name"`
	Members   int    `json:"members"`
}

// Task — общее задание клана. Прогресс копится работой членов, награда идёт в казну.
type Task struct {
	CorpID   string `json:"corp_id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Goal     int    `json:"goal"`
	Progress int    `json:"progress"`
}

// StructureStore хранит союзы и задания. Реализуют MemStore и PGStore.
type StructureStore interface {
	ListClans(ctx context.Context) ([]Seat, error)
	AllianceByCorp(ctx context.Context, corpID string) (Alliance, []Seat, bool, error)
	FoundAlliance(ctx context.Context, id, name, corpID, governor string) error
	AdmitClan(ctx context.Context, allianceID, corpID string) error
	CastVote(ctx context.Context, allianceID, corpID, choice string) (map[string]string, []string, error)
	SetGovernor(ctx context.Context, allianceID, governor string) error
	Task(ctx context.Context, corpID string) (Task, bool, error)
	SaveTask(ctx context.Context, t Task) error
	AddProgress(ctx context.Context, corpID string, n int) (Task, error)
	AddTreasury(ctx context.Context, corpID string, amount int64) error
}

func (s *Service) structure() (StructureStore, bool) {
	st, ok := s.store.(StructureStore)
	return st, ok
}

// Snapshot дополняет карточку клана заданием и союзом.
func (s *Service) Snapshot(ctx context.Context, corpID string) (Task, bool, Alliance, []Seat, bool, error) {
	st, ok := s.structure()
	if !ok {
		return Task{}, false, Alliance{}, nil, false, nil
	}
	task, hasTask, err := st.Task(ctx, corpID)
	if err != nil {
		return Task{}, false, Alliance{}, nil, false, err
	}
	if !hasTask {
		task = Task{CorpID: corpID, Kind: "sortie", Title: "Боевой выход клана", Goal: TaskGoal}
		if err := st.SaveTask(ctx, task); err != nil {
			return Task{}, false, Alliance{}, nil, false, err
		}
		hasTask = true
	}
	al, seats, hasAl, err := st.AllianceByCorp(ctx, corpID)
	return task, hasTask, al, seats, hasAl, err
}

// FoundAlliance создаёт союз из клана игрока. Игрок должен быть основателем клана.
func (s *Service) FoundAlliance(ctx context.Context, pid, name string) (Alliance, error) {
	st, ok := s.structure()
	if !ok {
		return Alliance{}, errors.New("structure")
	}
	c, isOwner, err := s.ownerOf(ctx, pid)
	if err != nil || !isOwner {
		if err != nil {
			return Alliance{}, err
		}
		return Alliance{}, ErrNotOwner
	}
	if _, _, has, err := st.AllianceByCorp(ctx, c.ID); err != nil {
		return Alliance{}, err
	} else if has {
		return Alliance{}, ErrHasAlliance
	}
	name = cleanName(name)
	if !nameRe.MatchString(name) || len(name) > 40 {
		return Alliance{}, ErrBadName
	}
	id := "al-" + strings.ToLower(c.Tag)
	if err := st.FoundAlliance(ctx, id, name, c.ID, pid); err != nil {
		return Alliance{}, err
	}
	al, _, _, err := st.AllianceByCorp(ctx, c.ID)
	return al, err
}

// JoinAlliance вводит клан основателя в существующий союз, если в нём меньше трёх кланов.
func (s *Service) JoinAlliance(ctx context.Context, pid, name string) (Alliance, error) {
	st, ok := s.structure()
	if !ok {
		return Alliance{}, errors.New("structure")
	}
	c, isOwner, err := s.ownerOf(ctx, pid)
	if err != nil || !isOwner {
		if err != nil {
			return Alliance{}, err
		}
		return Alliance{}, ErrNotOwner
	}
	if _, _, has, err := st.AllianceByCorp(ctx, c.ID); err != nil {
		return Alliance{}, err
	} else if has {
		return Alliance{}, ErrHasAlliance
	}
	var target string
	clans, err := st.ListClans(ctx)
	if err != nil {
		return Alliance{}, err
	}
	for _, seat := range clans {
		al, _, has, err := st.AllianceByCorp(ctx, seat.CorpID)
		if err != nil || !has {
			continue
		}
		if strings.EqualFold(al.Name, name) || al.ID == name {
			target = al.ID
			break
		}
	}
	if target == "" {
		return Alliance{}, ErrNoAlliance
	}
	if err := st.AdmitClan(ctx, target, c.ID); err != nil {
		return Alliance{}, err
	}
	al, _, _, err := st.AllianceByCorp(ctx, c.ID)
	return al, err
}

// Vote выбирает управляющего союза. Голос есть у основателя каждого клана союза.
func (s *Service) Vote(ctx context.Context, pid, choice string) (Alliance, error) {
	st, ok := s.structure()
	if !ok {
		return Alliance{}, errors.New("structure")
	}
	c, isOwner, err := s.ownerOf(ctx, pid)
	if err != nil || !isOwner {
		if err != nil {
			return Alliance{}, err
		}
		return Alliance{}, ErrNotOwner
	}
	al, seats, has, err := st.AllianceByCorp(ctx, c.ID)
	if err != nil {
		return Alliance{}, err
	}
	if !has {
		return Alliance{}, ErrNoAlliance
	}
	allowed := map[string]bool{}
	for _, seat := range seats {
		allowed[seat.Owner] = true
	}
	if !allowed[choice] {
		return Alliance{}, ErrBadVote
	}
	votes, owners, err := st.CastVote(ctx, al.ID, c.ID, choice)
	if err != nil {
		return Alliance{}, err
	}
	best, okPick := winner(votes, owners)
	if okPick && best != al.Governor {
		if err := st.SetGovernor(ctx, al.ID, best); err != nil {
			return Alliance{}, err
		}
	}
	al, _, _, err = st.AllianceByCorp(ctx, c.ID)
	return al, err
}

func winner(votes map[string]string, owners []string) (string, bool) {
	tally := map[string]int{}
	for _, choice := range votes {
		tally[choice]++
	}
	best, n, tie := "", 0, false
	for id, count := range tally {
		if count > n {
			best, n, tie = id, count, false
		} else if count == n {
			tie = true
		}
	}
	if best == "" || tie {
		return "", false
	}
	for _, id := range owners {
		if id == best {
			return best, true
		}
	}
	return "", false
}

// NoteWork добавляет вклад члена в задание клана. Полное задание пополняет казну и начинается заново.
func (s *Service) NoteWork(ctx context.Context, pid string, n int) (Task, error) {
	st, ok := s.structure()
	if !ok || n <= 0 {
		return Task{}, nil
	}
	c, _, err := s.store.CorporationByPlayer(ctx, pid)
	if err != nil {
		return Task{}, err
	}
	if c.ID == "" {
		return Task{}, ErrNoCorp
	}
	if _, has, err := st.Task(ctx, c.ID); err != nil {
		return Task{}, err
	} else if !has {
		if err := st.SaveTask(ctx, Task{CorpID: c.ID, Kind: "sortie", Title: "Боевой выход клана", Goal: TaskGoal}); err != nil {
			return Task{}, err
		}
	}
	task, err := st.AddProgress(ctx, c.ID, n)
	if err != nil {
		return Task{}, err
	}
	if task.Goal > 0 && task.Progress >= task.Goal {
		if err := st.AddTreasury(ctx, c.ID, TaskReward); err != nil {
			return Task{}, err
		}
		task.Progress = 0
		if err := st.SaveTask(ctx, task); err != nil {
			return Task{}, err
		}
	}
	return task, nil
}

// SettleAI сажает капитана в клан, добирает союз до трёх кланов и отдаёт голос за управляющего.
func (s *Service) SettleAI(ctx context.Context, pid, name string) {
	if s == nil || pid == "" {
		return
	}
	st, ok := s.structure()
	if !ok {
		return
	}
	if _, _, err := s.store.CorporationByPlayer(ctx, pid); err != nil {
		return
	} else if err == nil {
		if _, has, _ := s.store.CorporationByPlayer(ctx, pid); !has {
			s.placeAI(ctx, st, pid, name)
		}
	}
	c, has, err := s.store.CorporationByPlayer(ctx, pid)
	if err != nil || !has {
		return
	}
	_, _ = s.NoteWork(ctx, pid, 1)
	if c.Owner != pid {
		return
	}
	if _, _, allied, _ := st.AllianceByCorp(ctx, c.ID); !allied {
		if !s.joinOpenAlliance(ctx, st, pid) {
			allianceName := c.Tag + " Союз"
			_, _ = s.FoundAlliance(ctx, pid, allianceName)
		}
	}
	al, seats, allied, err := st.AllianceByCorp(ctx, c.ID)
	if err != nil || !allied || len(seats) == 0 {
		return
	}
	choice, members := "", -1
	for _, seat := range seats {
		if seat.Members > members || (seat.Members == members && (choice == "" || seat.Owner < choice)) {
			choice, members = seat.Owner, seat.Members
		}
	}
	if choice != "" && al.Governor != choice {
		_, _ = s.Vote(ctx, pid, choice)
	}
}

func (s *Service) placeAI(ctx context.Context, st StructureStore, pid, name string) {
	clans, err := st.ListClans(ctx)
	if err != nil {
		return
	}
	if len(clans) < 24 && founder(pid) {
		clan, tag := clanIdentity(pid)
		if _, err := s.Create(ctx, pid, clan, tag); err == nil {
			return
		}
	}
	best, n := "", int(^uint(0)>>1)
	for _, seat := range clans {
		if seat.Members < n {
			best, n = seat.Tag, seat.Members
		}
	}
	if best != "" {
		_, _ = s.Join(ctx, pid, best, name)
	}
}

func (s *Service) joinOpenAlliance(ctx context.Context, st StructureStore, pid string) bool {
	clans, err := st.ListClans(ctx)
	if err != nil {
		return false
	}
	seen := map[string]Alliance{}
	for _, seat := range clans {
		al, seats, has, err := st.AllianceByCorp(ctx, seat.CorpID)
		if err != nil || !has || len(seats) >= MaxAllianceClans {
			continue
		}
		seen[al.ID] = al
	}
	for _, al := range seen {
		if _, err := s.JoinAlliance(ctx, pid, al.Name); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) ownerOf(ctx context.Context, pid string) (Corporation, bool, error) {
	c, ok, err := s.store.CorporationByPlayer(ctx, pid)
	if err != nil || !ok {
		if err != nil {
			return Corporation{}, false, err
		}
		return Corporation{}, false, ErrNoCorp
	}
	return c, c.Owner == pid, nil
}

func founder(id string) bool {
	h := 0
	for _, r := range id {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h%7 == 0
}

func clanIdentity(id string) (string, string) {
	names := []string{
		"Северный Дозор", "Ржавый Якорь", "Тихая Стая", "Чёрный Прилив", "Белый Контур",
		"Дальний Корвет", "Синий Предел", "Медный Клин", "Ночной Фарватер", "Сухой Док",
		"Первый Руб", "Серая Верфь", "Красная Дуга", "Вольный Конвой", "Стеклянный Борт",
		"Нижний Пояс", "Прямой Курс", "Старая Вахта", "Холодный Трюм", "Ясный Створ",
		"Малый Рейд", "Лишний Борт", "Тихий Лаг", "Внешний Кольцо",
	}
	h := 0
	for _, r := range id {
		h = h*33 + int(r)
	}
	if h < 0 {
		h = -h
	}
	alpha := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	tag := string([]byte{alpha[h%len(alpha)], alpha[(h/17)%len(alpha)], alpha[(h/29)%len(alpha)]})
	return names[h%len(names)], tag
}
