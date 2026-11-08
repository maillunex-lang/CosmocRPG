// Package aistudio — «AI Civilization Training Studio»: стандарт ZIP-пакетов AI_Civilization_Pack_v1,
// проверка и безопасный импорт, генератор профилей капитанов, версии конфигурации с публикацией и откатом,
// минимальный диалоговый движок на правилах и тесты качества.
//
// В игре запрещены LLM API и облачная генерация текста: ChatGPT используется только вручную, вне игры,
// для подготовки ZIP-пакетов. Здесь нет ни одного сетевого вызова к внешним сервисам.
package aistudio

import "encoding/json"

// Идентификатор и версия формата пакета.
const (
	SchemaName    = "AI_Civilization_Pack_v1"
	SchemaVersion = 1
)

// Captain — профиль AI-капитана.
type Captain struct {
	ID            string              `json:"id"`
	Revision      int                 `json:"revision"`
	Name          string              `json:"name"`
	CallSign      string              `json:"call_sign"`
	Gender        string              `json:"gender"` // m | f
	Age           int                 `json:"age"`
	Bio           string              `json:"bio"`
	Profession    string              `json:"profession"`
	Faction       string              `json:"faction"`
	Corporation   *string             `json:"corporation"` // nil: независимый
	Personality   Personality         `json:"personality"`
	Speech        Speech              `json:"speech"`
	Interests     []string            `json:"interests"`
	Goals         []Goal              `json:"goals"`
	Strategies    []string            `json:"strategies"`
	Relationships []Relationship      `json:"relationships"`
	Replies       map[string][]string `json:"replies"`
	Meta          CaptainMeta         `json:"meta"`
}

// Personality — характер: черты и числовые оценки 0..100.
type Personality struct {
	Traits []string       `json:"traits"`
	Scores map[string]int `json:"scores"` // friendliness, caution, curiosity, greed, loyalty
	Temper string         `json:"temper"` // calm | hot | cold | cheerful
}

// Speech — речевой стиль.
type Speech struct {
	Formality     string   `json:"formality"` // formal | neutral | casual
	Verbosity     string   `json:"verbosity"` // terse | normal | talkative
	SlangLevel    int      `json:"slang_level"`
	Humor         string   `json:"humor"` // none | dry | playful | sarcastic
	Catchphrases  []string `json:"catchphrases"`
	Quirks        []string `json:"quirks"`
	AddressPlayer string   `json:"address_player"`
	AvoidWords    []string `json:"avoid_words"`
}

// Goal — цель капитана.
type Goal struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Priority int    `json:"priority"`
}

// Relationship — отношение к другому капитану.
type Relationship struct {
	CaptainID string `json:"captain_id"`
	Kind      string `json:"kind"` // friend | rival | partner | neutral | enemy
	Note      string `json:"note"`
}

// CaptainMeta — служебные поля профиля.
type CaptainMeta struct {
	Manual bool   `json:"manual"` // правился вручную: генератор и частичные импорты его не перезаписывают без явного согласия
	Source string `json:"source"` // generator | chatgpt | manual
	Seed   int64  `json:"seed"`
}

// Intent — намерение игрока.
type Intent struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Examples []string `json:"examples"`
	Keywords []string `json:"keywords"`
	Clarify  string   `json:"clarify"` // уточняющий вопрос, если уверенность низкая (необязательно)
}

// Synonym — каноническое слово и его варианты.
type Synonym struct {
	Canonical string   `json:"canonical"`
	Variants  []string `json:"variants"`
}

// SlangEntry — сленговое выражение и его обычная форма.
type SlangEntry struct {
	Slang   string `json:"slang"`
	Meaning string `json:"meaning"`
}

// Typo — частая опечатка.
type Typo struct {
	Wrong string `json:"wrong"`
	Right string `json:"right"`
}

// Synonyms — раздел словаря.
type Synonyms struct {
	Synonyms []Synonym    `json:"synonyms"`
	Slang    []SlangEntry `json:"slang"`
	Typos    []Typo       `json:"typos"`
}

// TemplateStyle — стиль, для которого написан шаблон ("" — любой).
type TemplateStyle struct {
	Formality string `json:"formality,omitempty"`
	Verbosity string `json:"verbosity,omitempty"`
	Humor     string `json:"humor,omitempty"`
}

// Template — шаблон ответа. Подстановки: {player}, {captain}, {fact:ID}.
type Template struct {
	ID     string        `json:"id"`
	Intent string        `json:"intent"`
	Style  TemplateStyle `json:"style"`
	Mood   string        `json:"mood,omitempty"`
	Text   string        `json:"text"`
	Weight int           `json:"weight"`
}

// Rule — контекстное правило диалога.
type Rule struct {
	ID   string   `json:"id"`
	When RuleWhen `json:"when"`
	Then RuleThen `json:"then"`
}

// RuleWhen — условия правила.
type RuleWhen struct {
	PrevIntent  string   `json:"prev_intent,omitempty"`
	Intent      string   `json:"intent,omitempty"`
	ContainsAny []string `json:"contains_any,omitempty"`
	Mood        string   `json:"mood,omitempty"`
}

// RuleThen — действие правила.
type RuleThen struct {
	IntentOverride string `json:"intent_override,omitempty"`
	SetMood        string `json:"set_mood,omitempty"`
}

// Emotion — эмоциональная реакция.
type Emotion struct {
	ID       string   `json:"id"`
	Triggers []string `json:"triggers"` // идентификаторы намерений
	Decay    int      `json:"decay"`    // через сколько реплик настроение возвращается к нейтральному
}

// Fact — разрешённый игровой факт: капитаны могут говорить только то, что здесь записано.
type Fact struct {
	ID   string   `json:"id"`
	Text string   `json:"text"`
	Tags []string `json:"tags"`
}

// Faction, Corporation — общество.
type Faction struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	About string `json:"about"`
}

type Corporation struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Faction string `json:"faction"`
	About   string `json:"about"`
}

// FactionRelation — отношения фракций.
type FactionRelation struct {
	A    string `json:"a"`
	B    string `json:"b"`
	Kind string `json:"kind"` // allied | friendly | neutral | tense | hostile
}

// Strategy — поведенческая стратегия.
type Strategy struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// Lesson — заметка обучения (что усвоено и чему научили).
type Lesson struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// MemoryRule — что капитан запоминает о собеседнике.
type MemoryRule struct {
	ID       string `json:"id"`
	Remember string `json:"remember"` // player_name | mood | topic
	TTLTurns int    `json:"ttl_turns"`
}

// Schedule — расписание активности (заготовка для будущих систем; в игре пока не используется).
type Schedule struct {
	CaptainID string `json:"captain_id"` // "*" — всем
	Activity  string `json:"activity"`
	Weight    int    `json:"weight"`
}

// DialogueTest — тестовый диалог: реплики игрока и ожидания.
type DialogueTest struct {
	ID        string     `json:"id"`
	CaptainID string     `json:"captain_id,omitempty"`
	Say       []string   `json:"say"`
	Expect    TestExpect `json:"expect"`
}

// TestExpect — ожидания к последнему ответу.
type TestExpect struct {
	Intent      string   `json:"intent,omitempty"`
	ContainsAny []string `json:"contains_any,omitempty"`
	NotContains []string `json:"not_contains,omitempty"`
	NoClarify   bool     `json:"no_clarify,omitempty"`
}

// Config — полная конфигурация AI-капитанов (то, что публикуется версиями).
type Config struct {
	Captains     map[string]Captain `json:"captains"`
	Intents      []Intent           `json:"intents"`
	Synonyms     Synonyms           `json:"synonyms"`
	Templates    []Template         `json:"templates"`
	Rules        []Rule             `json:"rules"`
	Emotions     []Emotion          `json:"emotions"`
	Facts        []Fact             `json:"facts"`
	Factions     []Faction          `json:"factions"`
	Corporations []Corporation      `json:"corporations"`
	Relations    []FactionRelation  `json:"relations"`
	Strategies   []Strategy         `json:"strategies"`
	Lessons      []Lesson           `json:"lessons"`
	MemoryRules  []MemoryRule       `json:"memory_rules"`
	Schedules    []Schedule         `json:"schedules"`
	Tests        []DialogueTest     `json:"tests"`
}

// NewConfig создаёт пустую конфигурацию.
func NewConfig() Config {
	return Config{Captains: map[string]Captain{}}
}

// Clone — глубокая копия через JSON (конфигурация небольшая).
func (c Config) Clone() Config {
	b, _ := json.Marshal(c)
	out := NewConfig()
	_ = json.Unmarshal(b, &out)
	if out.Captains == nil {
		out.Captains = map[string]Captain{}
	}
	return out
}

// Допустимые значения перечислений.
var (
	Professions = []string{
		"пилот-грузоперевозчик", "шахтёр-астероидщик", "торговец", "инженер-ремонтник", "наёмник",
		"исследователь", "контрабандист", "дипломат", "медик", "охотник за головами", "учёный", "логист",
		"сборщик утиля", "страж сектора", "картограф", "бортмеханик", "навигатор", "разведчик",
		"квартирмейстер", "журналист",
	}
	Formalities  = []string{"formal", "neutral", "casual"}
	Verbosities  = []string{"terse", "normal", "talkative"}
	Humors       = []string{"none", "dry", "playful", "sarcastic"}
	Tempers      = []string{"calm", "hot", "cold", "cheerful"}
	RelationKind = []string{"friend", "rival", "partner", "neutral", "enemy"}
	ScoreKeys    = []string{"friendliness", "caution", "curiosity", "greed", "loyalty"}
)
