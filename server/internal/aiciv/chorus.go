package aiciv

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

// Voice is one AI captain that can speak in the shared channel.
type Voice struct {
	ID, Name string
	Goal     GoalKind
}

// Chorus runs short AI↔AI exchanges: one voice opens, others answer in their own role.
// The same captain never replies to himself. Exact lines are not repeated.
type Chorus struct {
	Speaker *ChatSpeaker
	Log     *slog.Logger

	mu         sync.Mutex
	voices     []Voice
	lastID     string
	beat       int
	pauseUntil time.Time
}

func NewChorus(s *ChatSpeaker, log *slog.Logger) *Chorus {
	return &Chorus{Speaker: s, Log: log}
}

// Add registers a joined captain. Duplicate IDs are ignored.
func (c *Chorus) Add(v Voice) {
	if c == nil || v.ID == "" {
		return
	}
	if v.Name == "" {
		v.Name = v.ID
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, x := range c.voices {
		if x.ID == v.ID {
			return
		}
	}
	c.voices = append(c.voices, v)
}

func (c *Chorus) snapshot() []Voice {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Voice, len(c.voices))
	copy(out, c.voices)
	return out
}

// Run speaks one turn every interval. A conversation is opener + up to two replies, then a pause.
func (c *Chorus) Run(ctx context.Context, interval time.Duration) {
	if c == nil || c.Speaker == nil {
		return
	}
	if interval < 8*time.Second {
		interval = 12 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c.turn(ctx, now)
		}
	}
}

func (c *Chorus) turn(ctx context.Context, now time.Time) {
	if now.Before(c.pauseUntil) {
		return
	}
	voices := c.snapshot()
	if len(voices) < 2 {
		return
	}
	speaker := pickOther(voices, c.lastID, int(now.Unix()/12))
	if speaker.ID == "" {
		return
	}
	var lines []string
	if c.beat == 0 {
		lines = rotate(openers(speaker.Goal), int(now.Unix()/30)+len(speaker.ID))
	} else {
		_, heard := c.Speaker.Last("general")
		lines = rotate(replies(speaker.Goal, heard), int(now.Unix()/12)+len(speaker.Name))
	}
	if !c.sayFirst(ctx, speaker, lines) {
		return
	}
	c.lastID = speaker.ID
	c.beat++
	if c.beat >= 3 {
		c.beat = 0
		c.pauseUntil = now.Add(50 * time.Second)
		if c.Log != nil {
			c.Log.Info("AI chorus: exchange finished, pausing")
		}
	}
}

func (c *Chorus) sayFirst(ctx context.Context, v Voice, lines []string) bool {
	for _, text := range lines {
		if text == "" {
			continue
		}
		if c.Speaker.Say(ctx, v.ID, v.Name, "general", text) {
			if c.Log != nil {
				c.Log.Info("AI chorus", "from", v.Name, "goal", string(v.Goal), "text", text)
			}
			return true
		}
	}
	return false
}

func pickOther(voices []Voice, except string, salt int) Voice {
	if len(voices) == 0 {
		return Voice{}
	}
	if salt < 0 {
		salt = -salt
	}
	start := salt % len(voices)
	for i := 0; i < len(voices); i++ {
		v := voices[(start+i)%len(voices)]
		if v.ID != except {
			return v
		}
	}
	return Voice{}
}

func rotate(lines []string, salt int) []string {
	if len(lines) == 0 {
		return nil
	}
	if salt < 0 {
		salt = -salt
	}
	salt = salt % len(lines)
	out := make([]string, 0, len(lines))
	out = append(out, lines[salt:]...)
	out = append(out, lines[:salt]...)
	return out
}

func openers(goal GoalKind) []string {
	switch goal {
	case GoalFarmCredits:
		return []string{
			"вижу вражеский корабль, иду на перехват.",
			"если кто рядом — могу прикрыть.",
			"корпус в порядке, продолжаю патруль.",
			"цель снята, ищу следующую.",
		}
	case GoalMineAndSell:
		return []string{
			"нашёл месторождение, начинаю добычу.",
			"трюм почти полный, скоро на станцию.",
			"если кто знает хорошую жилу — пишите.",
			"продал груз, ищу следующее.",
		}
	case GoalExplore:
		return []string{
			"иду проверить сигнал, в чат пока не пишу координаты.",
			"ничего опасного по курсу пока не вижу.",
			"отметил точку, двигаюсь дальше.",
		}
	case GoalTradeRun:
		return []string{
			"если трюм полный — могу забрать груз по пути на станцию.",
			"на причале цена нормальная, не держите руду долго.",
			"маршрут пока пустой, жду контракт.",
		}
	default:
		return []string{
			"стою у причала. если корпус сел — пишите, подлечу.",
			"не геройствуйте в одиночку, сектор не пустой.",
			"я на станции, могу принять повреждённый корабль.",
		}
	}
}

func topicOf(heard string) string {
	switch {
	case containsAny(heard, "враг", "атак", "бой", "патрул", "перехват", "фланг", "пират"):
		return "combat"
	case containsAny(heard, "руд", "добы", "месторож", "жил", "трюм", "груз"):
		return "mine"
	case containsAny(heard, "сигнал", "точк", "курс", "развед"):
		return "explore"
	case containsAny(heard, "станц", "причал", "корпус", "ремонт"):
		return "dock"
	default:
		return "chat"
	}
}

func replies(goal GoalKind, heard string) []string {
	switch topicOf(heard) {
	case "combat":
		switch goal {
		case GoalFarmCredits:
			return []string{"беру его на себя.", "не лезь в лоб, я зайду с фланга.", "если меня собьют — добей, я отойду чинить корпус."}
		case GoalMineAndSell:
			return []string{"я не боевой, ухожу с линии огня.", "скажите, когда сектор чистый — вернусь к жиле.", "груз в трюме, в драку не полезу."}
		case GoalExplore:
			return []string{"вижу его, но в чат координаты не кидаю — идите по моему курсу.", "обхожу бой по дуге, сигнал важнее.", "если он сменит курс — напишу."}
		case GoalTradeRun:
			return []string{"обхожу бой, груз дороже геройства.", "скажите, когда коридор свободен, пойду на станцию.", "пиратов не кормлю, держусь в стороне."}
		default:
			return []string{"если корпус сядет — не добивайте, отходите ко мне.", "прикрою отход, сам в бой не лезу.", "живыми важнее, чем ещё один фраг."}
		}
	case "mine":
		switch goal {
		case GoalFarmCredits:
			return []string{"копайте, я присмотрю подходы к жиле.", "пираты любят гружёных шахтёров, не зевайте.", "если засветит красным — бросайте жилу, я прикрою."}
		case GoalMineAndSell:
			return []string{"там ещё одна жила чуть дальше по курсу.", "не столкнитесь у одной точки, трюм не резиновый.", "я эту уже обирал, берите следующую."}
		case GoalTradeRun:
			return []string{"полный трюм — зовите, заберу по пути.", "на станции сейчас нормальная цена, не ждите.", "могу взять излишек, самому вести не обязательно."}
		case GoalExplore:
			return []string{"за жилой был слабый сигнал, гляну потом.", "не шумите на частоте, я слушаю эфир.", "если жила пустая — скажите, не буду гнать вас туда."}
		default:
			return []string{"как набьёте полный — не тяните до станции на последнем корпусе.", "могу принять вас после рейса, только не в бою.", "жила никуда не денется, корабль — да."}
		}
	case "explore":
		switch goal {
		case GoalExplore:
			return []string{"я этот край уже смотрел, там тихо.", "сигнал плавает, не гонитесь все сразу.", "отмечу соседнюю точку и вернусь."}
		case GoalFarmCredits:
			return []string{"если там чужой — не суйтесь, сначала я.", "скиньте точку, проверю подход.", "разведка без прикрытия — плохая привычка."}
		default:
			return []string{"если там пусто — не тратьте ход, вернитесь.", "интересно, но я своё не брошу.", "напишите, если найдёте что-то ценное."}
		}
	default:
		switch goal {
		case GoalFarmCredits:
			return []string{"принял. я на обходе, если что — зовите.", "слышу. пока тихо, но я не сплю."}
		case GoalMineAndSell:
			return []string{"услышал. докопаю и пойду сдавать.", "понял, жилу не бросаю без причины."}
		case GoalExplore:
			return []string{"принял, продолжаю обход.", "слышу. если всплывёт новый сигнал — напишу отдельно."}
		case GoalTradeRun:
			return []string{"принял. если нужен перевоз — скажите куда.", "на связи. груз пока не трогаю."}
		default:
			return []string{"слышу вас. я у причала, если надо — подойду.", "принял. не пропадайте, сектор живой."}
		}
	}
}

// PickCast chooses up to n captains, preferring different professions so the chat has different voices.
func PickCast(in []aiagent.Profile, n int) []aiagent.Profile {
	if n < 2 {
		n = 2
	}
	if n > 4 {
		n = 4
	}
	seenBeh := map[string]bool{}
	var out []aiagent.Profile
	for _, p := range in {
		if p.ID == "" {
			continue
		}
		b := aiagent.BehaviorFor(p.Profession)
		if seenBeh[b] {
			continue
		}
		seenBeh[b] = true
		out = append(out, p)
		if len(out) == n {
			return out
		}
	}
	have := map[string]bool{}
	for _, p := range out {
		have[p.ID] = true
	}
	for _, p := range in {
		if p.ID == "" || have[p.ID] {
			continue
		}
		out = append(out, p)
		if len(out) == n {
			break
		}
	}
	return out
}
