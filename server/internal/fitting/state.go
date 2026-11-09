package fitting

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/maillunex-lang/CosmocRPG/server/internal/rpg"
)

var (
	ErrConflict       = errors.New("fitting_revision_conflict")
	ErrNotInitialized = errors.New("fitting_not_initialized")
	ErrNoOwner        = errors.New("fitting_no_owner")
	ErrInvalidState   = errors.New("fitting_invalid_state")
	ErrCredits        = errors.New("not_enough_credits")
)

const MaxItems = 512
const ShipStrizh = "FTR-001"

// Preliminary specialization progression; existing pilot/ship curves are kept.
var DefaultSpecializationCurve = rpg.Curve{MaxLevel: 20, Base: 250, Growth: 1.20}

type Build struct {
	Name           string            `json:"name"`
	Slots          map[string]string `json:"slots"`
	Active         []string          `json:"active"` // Exactly four; autoattack/ultimate are separate.
	Specialization string            `json:"specialization"`
	Talents        map[string]int    `json:"talents"`
	Choices        map[string]string `json:"choices"`
}
type PilotData struct {
	Version  int            `json:"version"`
	Revision int64          `json:"revision"`
	Learned  map[string]int `json:"learned"`
	Talents  map[string]int `json:"talents"`
	Research map[string]int `json:"research"`
}
type ShipData struct {
	Version          int              `json:"version"`
	ActiveBuild      int              `json:"active_build"`
	Builds           []Build          `json:"builds"`
	SpecializationXP map[string]int64 `json:"specialization_xp"`
}

// Progress is read from the existing real player/ship rows, not profile JSON or
// requests. The store keeps XP/attributes immutable during equipment commands.
type Progress struct {
	PilotXP    int64          `json:"pilot_xp"`
	ShipXP     int64          `json:"ship_xp"`
	Credits    int64          `json:"credits"`
	Attributes map[string]int `json:"attributes"`
}

func (p Progress) Levels() (int, int) {
	a, _, _ := rpg.DefaultPilotCurve.LevelFor(p.PilotXP)
	b, _, _ := rpg.DefaultShipCurve.LevelFor(p.ShipXP)
	return a, b
}

type State struct {
	Owner    string            `json:"owner"`
	Hull     string            `json:"hull"`
	Revision int64             `json:"revision"`
	Items    map[string]Item   `json:"items"`
	Slots    map[string]string `json:"slots"`
	Pilot    PilotData         `json:"pilot"`
	Profile  ShipData          `json:"profile"`
	Progress Progress          `json:"progress"`
}

func (s State) Clone() State { return clone(s) }
func (s State) Build() Build { return clone(s.Profile.Builds[s.Profile.ActiveBuild]) }

func NativeItemID(owner, hull, slot, prototype string) string {
	seed := "CosmocRPG:module:v1:" + owner + ":" + hull + ":" + slot + ":" + prototype
	prefix := "mod_"
	if hull == "FTR-001" && slot == "tech-1" && prototype == "ENG-S-START" {
		seed = "CosmocRPG:bound-strizh-engine:" + owner
		prefix = "eng_"
	}
	h := sha256.Sum256([]byte(seed))
	return prefix + hex.EncodeToString(h[:16])
}
func NativeGrant(hull, slot, prototype string) string {
	if hull == "FTR-001" && slot == "tech-1" && prototype == "ENG-S-START" {
		return "native-engine:v1"
	}
	return "native-kit:v1:" + slot
}
func BoundGrant(grant string) bool {
	return grant == "native-engine:v1" || strings.HasPrefix(grant, "native-kit:v1:")
}

func (c *Catalog) NewProfile(hull, legacySpec string, slots map[string]string) ShipData {
	h := c.hulls[hull]
	spec := h.Specializations[0]
	for _, id := range h.Specializations {
		if strings.HasSuffix(id, ":"+legacySpec) {
			spec = id
		}
	}
	p := ShipData{Version: 1, Builds: []Build{}, SpecializationXP: map[string]int64{}}
	for _, id := range h.Specializations {
		p.SpecializationXP[id] = 0
	}
	for i := 0; i < 3; i++ {
		p.Builds = append(p.Builds, Build{Name: fmt.Sprintf("Сборка %d", i+1), Slots: clone(slots), Active: []string{"M01", "", "", ""}, Specialization: spec, Talents: map[string]int{}, Choices: map[string]string{}})
	}
	return p
}
func NewPilotData() PilotData {
	return PilotData{Version: 1, Learned: map[string]int{}, Talents: map[string]int{}, Research: map[string]int{}}
}

func (c *Catalog) ValidateState(s State) (Result, error) {
	if s.Items == nil || s.Slots == nil || s.Pilot.Learned == nil || s.Pilot.Talents == nil || s.Pilot.Research == nil || s.Profile.SpecializationXP == nil {
		return Result{}, ErrInvalidState
	}
	if s.Revision < 0 || s.Pilot.Revision < 0 || s.Pilot.Version != 1 || s.Profile.Version != 1 || len(s.Profile.Builds) != 3 || s.Profile.ActiveBuild < 0 || s.Profile.ActiveBuild >= 3 || len(s.Items) > MaxItems || s.Progress.PilotXP < 0 || s.Progress.ShipXP < 0 || s.Progress.Credits < 0 {
		return Result{}, ErrInvalidState
	}
	h, ok := c.hulls[s.Hull]
	if !ok {
		return Result{}, ErrInvalidState
	}
	for id, item := range s.Items {
		if !ValidID(id) || id != item.ID || item.Owner != s.Owner || item.Engineering < 0 || item.Engineering > 5 {
			return Result{}, ErrInvalidState
		}
	}
	for id, rank := range s.Pilot.Learned {
		if !SkillID(id) || rank < 1 || rank > 5 {
			return Result{}, ErrInvalidState
		}
	}
	for id, rank := range s.Pilot.Talents {
		if !ValidID(id) || rank < 1 || rank > 5 {
			return Result{}, ErrInvalidState
		}
	}
	for id, rank := range s.Pilot.Research {
		known := false
		for _, list := range families {
			if contains(list, id) {
				known = true
			}
		}
		if !known || rank < 1 || rank > 3 {
			return Result{}, ErrInvalidState
		}
	}
	for id, xp := range s.Profile.SpecializationXP {
		if !contains(h.Specializations, id) || xp < 0 || xp > DefaultSpecializationCurve.TotalXP(20) {
			return Result{}, ErrInvalidState
		}
	}
	for _, b := range s.Profile.Builds {
		if b.Slots == nil || b.Talents == nil || b.Choices == nil || len(b.Slots) > 64 || strings.TrimSpace(b.Name) == "" || len([]rune(b.Name)) > 40 || len(b.Active) != 4 || !contains(h.Specializations, b.Specialization) {
			return Result{}, ErrInvalidState
		}
		seen := map[string]bool{}
		for _, id := range b.Active {
			if id == "" {
				continue
			}
			if !SkillID(id) || seen[id] || !contains(h.Abilities, id) {
				return Result{}, ErrInvalidState
			}
			seen[id] = true
		}
		for id, rank := range b.Talents {
			if !ValidID(id) || rank < 1 || rank > 5 {
				return Result{}, ErrInvalidState
			}
		}
		for id, option := range b.Choices {
			if !ValidID(id) || !ValidID(option) {
				return Result{}, ErrInvalidState
			}
		}
	}
	pl, sl := s.Progress.Levels()
	return c.Validate(s.Owner, s.Hull, pl, sl, s.Items, s.Slots)
}
