package corp

import "context"

func (m *MemStore) ready() {
	if m.alliances == nil {
		m.alliances = map[string]*memAlliance{}
		m.clanAlliance = map[string]string{}
		m.tasks = map[string]Task{}
	}
}

func (m *MemStore) ListClans(_ context.Context) ([]Seat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	var out []Seat
	for _, c := range m.byID {
		out = append(out, m.seat(c))
	}
	return out, nil
}

func (m *MemStore) seat(c *memCorp) Seat {
	ownerName := ""
	if mem, ok := c.members[c.Owner]; ok {
		ownerName = mem.Name
	}
	return Seat{CorpID: c.ID, Name: c.Name, Tag: c.Tag, Owner: c.Owner, OwnerName: ownerName, Members: len(c.members)}
}

func (m *MemStore) AllianceByCorp(_ context.Context, corpID string) (Alliance, []Seat, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	aid, ok := m.clanAlliance[corpID]
	if !ok {
		return Alliance{}, nil, false, nil
	}
	a := m.alliances[aid]
	seats := m.seats(a)
	al := a.Alliance
	al.Clans = len(seats)
	for _, seat := range seats {
		if seat.Owner == al.Governor {
			al.GovernorName = seat.OwnerName
		}
	}
	return al, seats, true, nil
}

func (m *MemStore) seats(a *memAlliance) []Seat {
	var out []Seat
	for id := range a.clans {
		if c := m.byID[id]; c != nil {
			out = append(out, m.seat(c))
		}
	}
	return out
}

func (m *MemStore) FoundAlliance(_ context.Context, id, name, corpID, governor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	if _, ok := m.clanAlliance[corpID]; ok {
		return ErrHasAlliance
	}
	if _, ok := m.alliances[id]; ok {
		return ErrTaken
	}
	m.alliances[id] = &memAlliance{Alliance: Alliance{ID: id, Name: name, Governor: governor}, clans: map[string]bool{corpID: true}, votes: map[string]string{}}
	m.clanAlliance[corpID] = id
	return nil
}

func (m *MemStore) AdmitClan(_ context.Context, allianceID, corpID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	a := m.alliances[allianceID]
	if a == nil {
		return ErrNoAlliance
	}
	if _, ok := m.clanAlliance[corpID]; ok {
		return ErrHasAlliance
	}
	if len(a.clans) >= MaxAllianceClans {
		return ErrAllianceFull
	}
	a.clans[corpID] = true
	m.clanAlliance[corpID] = allianceID
	return nil
}

func (m *MemStore) CastVote(_ context.Context, allianceID, corpID, choice string) (map[string]string, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	a := m.alliances[allianceID]
	if a == nil {
		return nil, nil, ErrNoAlliance
	}
	if !a.clans[corpID] {
		return nil, nil, ErrNotOwner
	}
	a.votes[corpID] = choice
	votes := map[string]string{}
	var owners []string
	for id := range a.clans {
		if c := m.byID[id]; c != nil {
			owners = append(owners, c.Owner)
		}
	}
	for k, v := range a.votes {
		votes[k] = v
	}
	return votes, owners, nil
}

func (m *MemStore) SetGovernor(_ context.Context, allianceID, governor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	a := m.alliances[allianceID]
	if a == nil {
		return ErrNoAlliance
	}
	a.Governor = governor
	return nil
}

func (m *MemStore) Task(_ context.Context, corpID string) (Task, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	t, ok := m.tasks[corpID]
	return t, ok, nil
}

func (m *MemStore) SaveTask(_ context.Context, task Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	m.tasks[task.CorpID] = task
	return nil
}

func (m *MemStore) AddProgress(_ context.Context, corpID string, n int) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready()
	t := m.tasks[corpID]
	t.CorpID = corpID
	t.Progress += n
	m.tasks[corpID] = t
	return t, nil
}

func (m *MemStore) AddTreasury(_ context.Context, corpID string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.byID[corpID]
	if c == nil {
		return ErrNoCorp
	}
	c.Treasury += amount
	return nil
}
