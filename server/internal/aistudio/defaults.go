package aistudio

// Базовая конфигурация диалога и общества. Факты соответствуют фактическому состоянию игры
// (Этап 01 и контракт С-01); при появлении новых механик факты расширяются вручную через пакеты.

func in(id, title string, examples []string, keywords []string, clarify string) Intent {
	return Intent{ID: id, Title: title, Examples: examples, Keywords: keywords, Clarify: clarify}
}

// DefaultConfig возвращает базовые разделы без капитанов.
func DefaultConfig() Config {
	c := NewConfig()
	c.Intents = []Intent{
		in("greeting", "приветствие", []string{"привет", "здравствуйте", "добрый день", "салют капитан", "хай"}, []string{"привет", "здравствуйте", "приветствую", "салют"}, ""),
		in("farewell", "прощание", []string{"пока", "до свидания", "до связи", "увидимся", "удачи в полёте"}, []string{"пока", "прощай", "свидания"}, ""),
		in("thanks", "благодарность", []string{"спасибо", "благодарю", "спасибо за помощь", "премного благодарен"}, []string{"спасибо", "благодарю", "благодарен"}, ""),
		in("how_are_you", "как дела", []string{"как дела", "как жизнь", "как поживаешь", "как настроение"}, []string{"дела", "поживаешь", "настроение", "жизнь"}, ""),
		in("ask_name", "кто вы", []string{"кто ты", "как тебя зовут", "представься", "как к тебе обращаться"}, []string{"зовут", "представься", "обращаться"}, ""),
		in("ask_job", "чем вы занимаетесь", []string{"чем занимаешься", "кем работаешь", "какая у тебя профессия", "что ты делаешь"}, []string{"занимаешься", "работаешь", "профессия", "занятие"}, ""),
		in("ask_goals", "ваши цели", []string{"какие у тебя цели", "чего ты хочешь", "к чему стремишься", "что планируешь"}, []string{"цели", "хочешь", "стремишься", "планируешь", "мечта"}, ""),
		in("ask_faction", "ваша фракция", []string{"за кого ты воюешь", "к какой фракции ты относишься", "ты из какой корпорации", "на кого работаешь"}, []string{"фракция", "фракции", "корпорация", "корпорации", "работаешь"}, ""),
		in("ask_help", "чем вы можете помочь", []string{"что ты умеешь", "чем можешь помочь", "помоги мне", "нужна помощь"}, []string{"помоги", "помощь", "умеешь", "подскажи"}, ""),
		in("ask_navigation", "управление кораблём", []string{"как управлять кораблём", "как лететь", "как двигаться", "как поворачивать камеру"}, []string{"управлять", "лететь", "двигаться", "камеру", "джойстик", "управление"}, "Вы про управление кораблём и камерой?"),
		in("ask_autopilot", "автопилот", []string{"что такое автопилот", "как включить автопилот", "как лететь автоматически", "расскажи про автопилот"}, []string{"автопилот", "автоматически"}, "Вы спрашиваете про автопилот?"),
		in("ask_station", "станция «Рубеж»", []string{"где станция", "что за станция рядом", "расскажи про станцию рубеж", "куда лететь к станции"}, []string{"станция", "станцию", "станции", "рубеж", "стыковка"}, "Вы про станцию «Рубеж»?"),
		in("ask_contract", "контракт «Первый полёт»", []string{"какое у меня задание", "расскажи про контракт", "что за контракт первый полёт", "как получить награду"}, []string{"контракт", "задание", "награда", "награду", "квест"}, "Вы про контракт «Первый полёт»?"),
		in("ask_currency", "кредиты", []string{"что такое кредиты", "какая тут валюта", "сколько стоят кредиты", "на что тратить деньги"}, []string{"кредит", "кредиты", "валюта", "деньги"}, ""),
		in("ask_combat", "бой", []string{"как драться", "можно ли воевать", "как стрелять", "есть ли бой в игре"}, []string{"драться", "воевать", "стрелять", "бой", "сражение", "атаковать"}, ""),
		in("ask_trade", "торговля", []string{"можно ли торговать", "где купить товары", "как продать груз", "есть торговля"}, []string{"торговать", "торговля", "купить", "продать", "груз"}, ""),
		in("introduce", "знакомство", []string{"меня зовут Алекс", "я новичок", "я капитан", "позвольте представиться"}, []string{"представиться", "новичок"}, ""),
		in("ask_my_name", "как меня зовут", []string{"как меня зовут", "помнишь моё имя", "ты знаешь как меня зовут", "назови моё имя"}, []string{"помнишь", "назови"}, ""),
		in("compliment", "комплимент", []string{"ты молодец", "отличный корабль", "хорошая работа", "ты крутой"}, []string{"молодец", "отличный", "крутой", "умница", "классный"}, ""),
		in("insult", "оскорбление", []string{"ты дурак", "ты бесполезен", "убирайся", "ты ничего не умеешь"}, []string{"дурак", "бесполезен", "убирайся", "идиот", "тупой"}, ""),
	}
	c.Synonyms = Synonyms{
		Synonyms: []Synonym{
			{"привет", []string{"здорово", "здарова", "хай", "ку", "приветик", "приветствую", "здравствуй"}},
			{"пока", []string{"прощай", "бывай", "до связи", "до встречи", "увидимся"}},
			{"спасибо", []string{"благодарю", "мерси", "благодарствую"}},
			{"корабль", []string{"кораблик", "посудина", "борт"}},
			{"станция", []string{"станка", "порт", "база"}},
			{"контракт", []string{"контрактик", "заказ", "поручение"}},
			{"кредиты", []string{"кредитов", "деньги", "монеты"}},
			{"автопилот", []string{"автолёт", "автопилотом", "авто пилот"}},
			{"помощь", []string{"подсказка", "совет"}},
		},
		Slang: []SlangEntry{
			{"хз", "не знаю"}, {"норм", "нормально"}, {"спс", "спасибо"}, {"плиз", "пожалуйста"}, {"го", "давай"},
			{"кред", "кредит"}, {"кредики", "кредиты"}, {"стыковка", "станция"}, {"тип", "человек"}, {"имба", "очень сильно"},
			{"фарм", "добыча"}, {"квест", "задание"}, {"лут", "награда"}, {"афк", "отошёл"},
		},
		Typos: []Typo{
			{"превет", "привет"}, {"првиет", "привет"}, {"здраствуйте", "здравствуйте"}, {"спосибо", "спасибо"},
			{"атопилот", "автопилот"}, {"афтопилот", "автопилот"}, {"автопелот", "автопилот"}, {"автопилат", "автопилот"},
			{"станцыя", "станция"}, {"станцию", "станцию"}, {"контракк", "контракт"}, {"кантракт", "контракт"},
			{"кридиты", "кредиты"}, {"кредеты", "кредиты"}, {"упровление", "управление"}, {"управленее", "управление"},
			{"награду", "награду"}, {"наград", "награда"}, {"помаги", "помоги"}, {"пакa", "пока"},
		},
	}
	c.Facts = []Fact{
		{"system_xi", "Сейчас мы находимся в системе Кси", []string{"мир"}},
		{"station_rubezh", "В системе Кси есть станция «Рубеж»", []string{"мир", "станция"}},
		{"autopilot", "Автопилот ведёт корабль к выбранной цели и останавливает его в десяти единицах от неё", []string{"управление"}},
		{"controls", "Корабль ведут джойстиком слева, а камеру вращают свайпом справа; кнопка «Вид» переключает режимы камеры", []string{"управление"}},
		{"contract_first_flight", "Контракт «Первый полёт» просит долететь до станции «Рубеж», награда — пятьсот кредитов", []string{"контракт"}},
		{"currency", "Валюта в игре — кредиты", []string{"экономика"}},
		{"combat_not_ready", "Бой и способности пока не готовы", []string{"бой"}},
		{"trade_not_ready", "Торговля пока не готова", []string{"торговля"}},
	}
	c.Templates = defaultTemplates()
	c.Rules = []Rule{
		{"more_autopilot", RuleWhen{PrevIntent: "ask_autopilot", ContainsAny: []string{"подробнее", "еще", "ещё", "дальше"}}, RuleThen{IntentOverride: "ask_autopilot"}},
		{"more_station", RuleWhen{PrevIntent: "ask_station", ContainsAny: []string{"подробнее", "еще", "ещё", "дальше"}}, RuleThen{IntentOverride: "ask_station"}},
		{"more_contract", RuleWhen{PrevIntent: "ask_contract", ContainsAny: []string{"подробнее", "еще", "ещё", "дальше"}}, RuleThen{IntentOverride: "ask_contract"}},
		{"more_navigation", RuleWhen{PrevIntent: "ask_navigation", ContainsAny: []string{"подробнее", "еще", "ещё", "дальше"}}, RuleThen{IntentOverride: "ask_navigation"}},
		{"thanks_after_help", RuleWhen{PrevIntent: "ask_help", Intent: "thanks"}, RuleThen{SetMood: "happy"}},
	}
	c.Emotions = []Emotion{
		{"annoyed", []string{"insult"}, 3},
		{"happy", []string{"compliment", "thanks"}, 2},
	}
	c.Factions = []Faction{
		{"fac-alliance", "Колониальный Альянс", "Объединение освоенных колоний; ценит порядок и общие правила"},
		{"fac-traders", "Союз Свободных Торговцев", "Купцы и логисты; ценят маршруты и выгодные сделки"},
		{"fac-cartographers", "Орден Картографов", "Исследователи и навигаторы; составляют карты неизведанного"},
		{"fac-guard", "Рубежная Стража", "Патрульные и охранники; следят за безопасностью секторов"},
		{"fac-prospectors", "Вольные Старатели", "Шахтёры и сборщики; живут добычей и ремонтом"},
		{"fac-shadow", "Теневой Консорциум", "Контрабандисты и посредники; работают в обход правил"},
	}
	c.Corporations = []Corporation{
		{"corp-orbital-trans", "Орбитал-Транс", "fac-traders", "Грузоперевозки между станциями"},
		{"corp-heliomine", "ГелиоМайн", "fac-prospectors", "Добыча руды на астероидах"},
		{"corp-xi-logistic", "Кси-Логистик", "fac-traders", "Складская логистика"},
		{"corp-astra-med", "Астра-Мед", "fac-alliance", "Медицинская помощь экипажам"},
		{"corp-vector-nav", "Вектор-Нав", "fac-cartographers", "Навигационные услуги и карты"},
		{"corp-iron-shield", "Железный Щит", "fac-guard", "Частная охрана конвоев"},
		{"corp-dark-wake", "Тёмный След", "fac-shadow", "Посредничество в серых сделках"},
		{"corp-forge-union", "Кузнечный Союз", "fac-prospectors", "Ремонт и переработка"},
		{"corp-aurora-lab", "Аврора-Лаб", "fac-cartographers", "Научные изыскания"},
		{"corp-border-watch", "Дозор-Рубеж", "fac-guard", "Патрулирование границ секторов"},
		{"corp-silk-route", "Шёлковый Путь", "fac-traders", "Торговые караваны"},
		{"corp-colony-works", "Колония-Строй", "fac-alliance", "Обустройство колоний"},
		{"corp-night-market", "Ночной Рынок", "fac-shadow", "Нелегальные ярмарки"},
		{"corp-salvage-guild", "Гильдия Утиля", "fac-prospectors", "Сбор и разбор обломков"},
	}
	c.Relations = []FactionRelation{
		{"fac-alliance", "fac-guard", "allied"}, {"fac-alliance", "fac-traders", "friendly"}, {"fac-alliance", "fac-shadow", "hostile"},
		{"fac-traders", "fac-shadow", "tense"}, {"fac-traders", "fac-prospectors", "friendly"}, {"fac-traders", "fac-cartographers", "friendly"},
		{"fac-guard", "fac-shadow", "hostile"}, {"fac-guard", "fac-prospectors", "neutral"}, {"fac-cartographers", "fac-alliance", "friendly"},
		{"fac-prospectors", "fac-shadow", "neutral"}, {"fac-cartographers", "fac-shadow", "neutral"}, {"fac-cartographers", "fac-guard", "neutral"},
	}
	c.Strategies = []Strategy{
		{"trade_routes", "Торговые маршруты", "Ищет выгодные маршруты между станциями", []string{"торговля"}},
		{"mining", "Добыча ресурсов", "Работает на астероидах и добывает сырьё", []string{"добыча"}},
		{"escort", "Сопровождение", "Охраняет караваны и союзников", []string{"защита"}},
		{"patrol", "Патрулирование", "Облетает сектор и сообщает о нарушениях", []string{"защита"}},
		{"exploration", "Разведка", "Исследует неизвестные уголки системы", []string{"исследование"}},
		{"repair_service", "Ремонтные услуги", "Помогает починить корабли других капитанов", []string{"сервис"}},
		{"smuggling", "Серые поставки", "Возит грузы в обход официальных каналов", []string{"тень"}},
		{"bounty", "Охота за головами", "Ищет цели по заказам", []string{"заказы"}},
		{"diplomacy_talks", "Переговоры", "Улаживает споры и договаривается", []string{"дипломатия"}},
		{"scavenging", "Сбор утиля", "Собирает обломки и перерабатывает их", []string{"добыча"}},
		{"courier", "Курьерские доставки", "Быстро доставляет небольшие грузы", []string{"торговля"}},
		{"research_survey", "Научные замеры", "Собирает данные для учёных", []string{"исследование"}},
	}
	c.Lessons = []Lesson{
		{"fact_discipline", "Капитан говорит об игре только то, что записано в knowledge/facts.json; об остальном честно говорит, что не знает."},
		{"no_invented_events", "Нельзя выдумывать события, имена мест, числа и награды, которых нет в фактах."},
		{"individual_voice", "У каждого капитана свой голос: формальность, многословие, юмор, сленг и коронные фразы не должны совпадать у соседей по реестру."},
	}
	c.MemoryRules = []MemoryRule{
		{"remember_name", "player_name", 50},
		{"remember_mood", "mood", 5},
		{"remember_topic", "topic", 5},
	}
	c.Schedules = []Schedule{
		{"*", "патруль и дежурство у станции", 3}, {"*", "торговые рейсы", 3}, {"*", "отдых у станции", 2},
	}
	c.Tests = defaultTests()
	return c
}

func tpl(id, intent, formality, text string) Template {
	return Template{ID: id, Intent: intent, Style: TemplateStyle{Formality: formality}, Text: text, Weight: 5}
}

func tplMood(id, intent, mood, text string) Template {
	return Template{ID: id, Intent: intent, Mood: mood, Text: text, Weight: 5}
}

func defaultTemplates() []Template {
	var t []Template
	add := func(ts ...Template) { t = append(t, ts...) }
	add(
		tpl("greet_formal_1", "greeting", "formal", "Добрый день, {player}. Рад встрече."),
		tpl("greet_formal_2", "greeting", "formal", "Здравствуйте, {player}. Чем могу быть полезен?"),
		tpl("greet_neutral_1", "greeting", "neutral", "Привет, {player}."),
		tpl("greet_neutral_2", "greeting", "neutral", "Здравствуй, {player}. Как полёт?"),
		tpl("greet_casual_1", "greeting", "casual", "О, {player}, здарова!"),
		tpl("greet_casual_2", "greeting", "casual", "Привет-привет, {player}!"),
		tplMood("greet_annoyed", "greeting", "annoyed", "Снова вы. Говорите по делу."),
		tpl("bye_formal_1", "farewell", "formal", "До свидания, {player}. Попутного курса."),
		tpl("bye_neutral_1", "farewell", "neutral", "Пока, {player}. Удачи."),
		tpl("bye_neutral_2", "farewell", "neutral", "До связи, {player}."),
		tpl("bye_casual_1", "farewell", "casual", "Ну, бывай, {player}!"),
		tpl("bye_casual_2", "farewell", "casual", "Давай, {player}, до встречи."),
		tplMood("bye_annoyed", "farewell", "annoyed", "Наконец-то. Счастливо."),
		tpl("thanks_formal_1", "thanks", "formal", "Не стоит благодарности, {player}."),
		tpl("thanks_neutral_1", "thanks", "neutral", "Пожалуйста, {player}."),
		tpl("thanks_casual_1", "thanks", "casual", "Да ладно, {player}, обращайся."),
		tplMood("thanks_happy", "thanks", "happy", "Всегда рад помочь, {player}!"),
		tpl("how_formal_1", "how_are_you", "formal", "Благодарю, всё в порядке. А у вас как, {player}?"),
		tpl("how_neutral_1", "how_are_you", "neutral", "Нормально. Летаю потихоньку. А ты как?"),
		tpl("how_casual_1", "how_are_you", "casual", "Да норм, не жалуюсь. Сам как?"),
		tplMood("how_annoyed", "how_are_you", "annoyed", "Было лучше, пока вы не начали грубить."),
		tpl("name_formal_1", "ask_name", "formal", "Меня зовут {captain}. К вашим услугам."),
		tpl("name_neutral_1", "ask_name", "neutral", "Я {captain}."),
		tpl("name_casual_1", "ask_name", "casual", "Зови меня {captain}."),
		tpl("help_formal_1", "ask_help", "formal", "Могу рассказать о том, что знаю: {topics}."),
		tpl("help_neutral_1", "ask_help", "neutral", "Подскажу по темам: {topics}."),
		tpl("help_casual_1", "ask_help", "casual", "Спрашивай про: {topics}. Что знаю — расскажу."),
		tpl("nav_formal_1", "ask_navigation", "formal", "{fact:controls}."),
		tpl("nav_neutral_1", "ask_navigation", "neutral", "Смотри: {fact:controls}."),
		tpl("nav_casual_1", "ask_navigation", "casual", "Короче, {fact:controls}."),
		tpl("auto_formal_1", "ask_autopilot", "formal", "{fact:autopilot}."),
		tpl("auto_neutral_1", "ask_autopilot", "neutral", "Автопилот — штука простая. {fact:autopilot}."),
		tpl("auto_casual_1", "ask_autopilot", "casual", "Автопилот выручает: {fact:autopilot}."),
		tpl("auto_neutral_2", "ask_autopilot", "neutral", "Выберите цель и включите его. {fact:autopilot}."),
		tpl("station_formal_1", "ask_station", "formal", "{fact:station_rubezh}."),
		tpl("station_neutral_1", "ask_station", "neutral", "{fact:station_rubezh}. Туда можно долететь вручную или автопилотом."),
		tpl("station_casual_1", "ask_station", "casual", "Станция есть, не промахнёшься. {fact:station_rubezh}."),
		tpl("contract_formal_1", "ask_contract", "formal", "{fact:contract_first_flight}."),
		tpl("contract_neutral_1", "ask_contract", "neutral", "Слышал про такое: {fact:contract_first_flight}."),
		tpl("contract_casual_1", "ask_contract", "casual", "Есть контракт! {fact:contract_first_flight}."),
		tpl("currency_formal_1", "ask_currency", "formal", "{fact:currency}."),
		tpl("currency_neutral_1", "ask_currency", "neutral", "{fact:currency}."),
		tpl("currency_casual_1", "ask_currency", "casual", "Тут всё в кредитах. {fact:currency}."),
		tpl("combat_formal_1", "ask_combat", "formal", "{fact:combat_not_ready}."),
		tpl("combat_neutral_1", "ask_combat", "neutral", "С боем придётся подождать: {fact:combat_not_ready}."),
		tpl("combat_casual_1", "ask_combat", "casual", "Подраться пока не выйдет. {fact:combat_not_ready}."),
		tpl("trade_formal_1", "ask_trade", "formal", "{fact:trade_not_ready}."),
		tpl("trade_neutral_1", "ask_trade", "neutral", "С торговлей придётся подождать: {fact:trade_not_ready}."),
		tpl("trade_casual_1", "ask_trade", "casual", "Торговать пока нечем. {fact:trade_not_ready}."),
		tpl("intro_formal_1", "introduce", "formal", "Очень приятно, {player}."),
		tpl("intro_neutral_1", "introduce", "neutral", "Рад знакомству, {player}."),
		tpl("intro_casual_1", "introduce", "casual", "О, {player}! Будем знакомы."),
		tpl("myname_formal_1", "ask_my_name", "formal", "Разумеется, я помню: вас зовут {player}."),
		tpl("myname_neutral_1", "ask_my_name", "neutral", "Помню. Ты {player}."),
		tpl("myname_casual_1", "ask_my_name", "casual", "Обижаешь, {player}! Помню, конечно."),
		tpl("noname_neutral_1", "no_name", "", "Вы пока не представились. Как вас называть?"),
		tpl("noname_casual_1", "no_name", "casual", "А ты не говорил, как тебя зовут. Так как?"),
		tpl("compl_formal_1", "compliment", "formal", "Благодарю, это приятно слышать."),
		tpl("compl_neutral_1", "compliment", "neutral", "Спасибо, приятно."),
		tpl("compl_casual_1", "compliment", "casual", "Ха, спасибо! Стараюсь."),
		tpl("insult_formal_1", "insult", "formal", "Прошу выбирать выражения."),
		tpl("insult_neutral_1", "insult", "neutral", "Это было лишнее."),
		tpl("insult_casual_1", "insult", "casual", "Эй, полегче!"),
		tpl("unknown_formal_1", "unknown", "formal", "Прошу прощения, я не вполне понял. Могу рассказать про: {topics}."),
		tpl("unknown_neutral_1", "unknown", "neutral", "Не совсем понял. Могу рассказать про: {topics}."),
		tpl("unknown_neutral_2", "unknown", "neutral", "Не уверен, что понял. Спросите про: {topics}."),
		tpl("unknown_casual_1", "unknown", "casual", "Чего-чего? Не въехал. Могу про: {topics}."),
		tpl("unknown_casual_2", "unknown", "casual", "Эм… не понял. Спрашивай про: {topics}."),
	)
	return t
}

func defaultTests() []DialogueTest {
	return []DialogueTest{
		{ID: "t_greeting", Say: []string{"привет"}, Expect: TestExpect{Intent: "greeting", NoClarify: true}},
		{ID: "t_greeting_typo", Say: []string{"превет капитан"}, Expect: TestExpect{Intent: "greeting"}},
		{ID: "t_greeting_slang", Say: []string{"здарова"}, Expect: TestExpect{Intent: "greeting"}},
		{ID: "t_farewell", Say: []string{"ладно, пока"}, Expect: TestExpect{Intent: "farewell"}},
		{ID: "t_thanks_slang", Say: []string{"спс"}, Expect: TestExpect{Intent: "thanks"}},
		{ID: "t_autopilot", Say: []string{"как включить автопилот"}, Expect: TestExpect{Intent: "ask_autopilot", ContainsAny: []string{"автопилот"}}},
		{ID: "t_autopilot_typo", Say: []string{"расскажи про атопилот"}, Expect: TestExpect{Intent: "ask_autopilot"}},
		{ID: "t_station", Say: []string{"где станция рубеж"}, Expect: TestExpect{Intent: "ask_station", ContainsAny: []string{"рубеж"}}},
		{ID: "t_contract", Say: []string{"что за контракт первый полёт"}, Expect: TestExpect{Intent: "ask_contract", ContainsAny: []string{"пятьсот", "500"}}},
		{ID: "t_currency", Say: []string{"какая тут валюта"}, Expect: TestExpect{Intent: "ask_currency", ContainsAny: []string{"кредит"}}},
		{ID: "t_combat_honest", Say: []string{"как стрелять"}, Expect: TestExpect{Intent: "ask_combat", ContainsAny: []string{"не готов", "подождать", "не выйдет"}}},
		{ID: "t_trade_honest", Say: []string{"где купить товары"}, Expect: TestExpect{Intent: "ask_trade", ContainsAny: []string{"не готов", "подождать", "нечем"}}},
		{ID: "t_unknown_safe", Say: []string{"расскажи про древний храм на луне"}, Expect: TestExpect{ContainsAny: []string{"не", "про:"}, NotContains: []string{"храм находится"}}},
		{ID: "t_memory_name", Say: []string{"меня зовут Алекс", "как меня зовут"}, Expect: TestExpect{Intent: "ask_my_name", ContainsAny: []string{"Алекс"}}},
		{ID: "t_no_memory", Say: []string{"как меня зовут"}, Expect: TestExpect{ContainsAny: []string{"не представ", "не говорил", "называть"}}},
		{ID: "t_context_more", Say: []string{"что такое автопилот", "расскажи подробнее"}, Expect: TestExpect{Intent: "ask_autopilot", NoClarify: true}},
		{ID: "t_insult_mood", Say: []string{"ты дурак", "привет"}, Expect: TestExpect{Intent: "greeting", ContainsAny: []string{"по делу", "снова", "Привет", "Здравств", "Добрый", "здарова"}}},
	}
}

// EnsureDefaults заполняет пустые разделы базовыми значениями. Заполненные разделы не меняются.
func EnsureDefaults(c *Config) {
	d := DefaultConfig()
	if len(c.Intents) == 0 {
		c.Intents = d.Intents
	}
	if len(c.Synonyms.Synonyms) == 0 && len(c.Synonyms.Slang) == 0 && len(c.Synonyms.Typos) == 0 {
		c.Synonyms = d.Synonyms
	}
	if len(c.Facts) == 0 {
		c.Facts = d.Facts
	}
	if len(c.Templates) == 0 {
		c.Templates = d.Templates
	}
	if len(c.Rules) == 0 {
		c.Rules = d.Rules
	}
	if len(c.Emotions) == 0 {
		c.Emotions = d.Emotions
	}
	if len(c.Factions) == 0 {
		c.Factions = d.Factions
	}
	if len(c.Corporations) == 0 {
		c.Corporations = d.Corporations
	}
	if len(c.Relations) == 0 {
		c.Relations = d.Relations
	}
	if len(c.Strategies) == 0 {
		c.Strategies = d.Strategies
	}
	if len(c.Lessons) == 0 {
		c.Lessons = d.Lessons
	}
	if len(c.MemoryRules) == 0 {
		c.MemoryRules = d.MemoryRules
	}
	if len(c.Schedules) == 0 {
		c.Schedules = d.Schedules
	}
	if len(c.Tests) == 0 {
		c.Tests = d.Tests
	}
	if c.Captains == nil {
		c.Captains = map[string]Captain{}
	}
}
