// Command server — авторитетный игровой сервер «Звёздные рубежи».
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/account"
	"github.com/maillunex-lang/CosmocRPG/server/internal/admin"
	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/chat"
	"github.com/maillunex-lang/CosmocRPG/server/internal/config"
	"github.com/maillunex-lang/CosmocRPG/server/internal/corp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/design"
	"github.com/maillunex-lang/CosmocRPG/server/internal/httpapi"
	"github.com/maillunex-lang/CosmocRPG/server/internal/pilot"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipcatalog"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shipxp"
	"github.com/maillunex-lang/CosmocRPG/server/internal/shop"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldcfg"
	"github.com/maillunex-lang/CosmocRPG/server/internal/worldgen"
	"github.com/maillunex-lang/CosmocRPG/server/internal/ws"
)

// version задаётся при сборке: -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		// для HEALTHCHECK в Docker: в минимальном образе нет curl
		if err := healthcheck(os.Getenv("COSMOC_HTTP_ADDR")); err != nil {
			fmt.Fprintln(os.Stderr, "нездоров:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "admin-create" {
		if err := adminCreate(); err != nil {
			fmt.Fprintln(os.Stderr, "ошибка:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "ships-sync" {
		if err := shipsSync(); err != nil {
			fmt.Fprintln(os.Stderr, "ошибка:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("сервер остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

// healthcheckURL строит адрес проверки по COSMOC_HTTP_ADDR (":8080", "0.0.0.0:8080", "127.0.0.1:9000").
func healthcheckURL(addr string) string {
	port := "8080"
	if i := strings.LastIndex(addr, ":"); i >= 0 && i+1 < len(addr) {
		port = addr[i+1:]
	}
	return "http://127.0.0.1:" + port + "/health"
}

func healthcheck(addr string) error {
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(healthcheckURL(addr))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("статус %d", resp.StatusCode)
	}
	return nil
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	_ = l.UnmarshalText([]byte(s))
	return l
}

// shipsSync добавляет в активный каталог кораблей то, чего в нём нет, из встроенных данных: новые корабли и недостающие способности.
// Значения существующих кораблей (правки из админки) не перезаписываются. Повторный запуск ничего не меняет.
func shipsSync() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	svc := shipcatalog.NewService(shipcatalog.NewPGStore(pool), nil)
	if err := svc.Init(ctx); err != nil {
		return err
	}
	changed, err := syncShips(ctx, svc, func(m string) { fmt.Println(m) })
	if err != nil {
		return err
	}
	fmt.Println("изменений:", changed)
	return nil
}

// syncShips добавляет недостающее из встроенного каталога (см. shipsSync) и возвращает число изменённых кораблей.
func syncShips(ctx context.Context, svc *shipcatalog.Service, say func(string)) (int, error) {
	active, _, err := svc.Active(ctx)
	if err != nil {
		return 0, err
	}
	have := map[string]world.ShipSpec{}
	for _, sp := range active.Ships {
		have[sp.ID] = sp
	}
	changed := 0
	for _, b := range world.DefaultDoc().Ships {
		cur, ok := have[b.ID]
		if !ok {
			if _, err := svc.UpdateShip(ctx, "система", b); err != nil {
				return changed, fmt.Errorf("корабль %s: %w", b.ID, err)
			}
			say("добавлен корабль " + b.ID)
			changed++
			continue
		}
		added := false
		for k, ab := range b.Abilities {
			if _, has := cur.Abilities[k]; !has {
				cur.Abilities[k] = ab
				added = true
				say("корабль " + b.ID + ": добавлена способность " + k)
			}
		}
		if added {
			if _, err := svc.UpdateShip(ctx, "система", cur); err != nil {
				return changed, fmt.Errorf("корабль %s: %w", b.ID, err)
			}
			changed++
		}
	}
	return changed, nil
}

// adminCreate создаёт администратора: server admin-create
// Имя берётся из COSMOC_ADMIN_USERNAME, пароль из COSMOC_ADMIN_PASSWORD или первой строки stdin.
// Пароль не принимается аргументом командной строки (виден в списке процессов и истории).
func adminCreate() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	username := os.Getenv("COSMOC_ADMIN_USERNAME")
	if username == "" {
		return errors.New("COSMOC_ADMIN_USERNAME не задана")
	}
	password := os.Getenv("COSMOC_ADMIN_PASSWORD")
	if password == "" {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return errors.New("пароль не задан: используйте COSMOC_ADMIN_PASSWORD или stdin")
		}
		password = strings.TrimRight(line, "\r\n")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	store := admin.NewStore(pool)
	u, err := store.CreateUser(ctx, username, password)
	if err != nil {
		return err
	}
	_ = store.Audit(ctx, u.ID, u.Username, "admin_created", "cli", nil)
	fmt.Printf("администратор %q создан\n", u.Username)
	return nil
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	pool, err := db.Connect(connCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := db.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	log.Info("миграции выполнены", "applied", applied)

	boots, err := db.RecordBoot(ctx, pool, version)
	if err != nil {
		return err
	}
	log.Info("запуск зафиксирован", "boots_total", boots, "version", version)

	// Э-1 (Решение №30): три системы — Аврора (главная, бывшая «Кси»), Меридиан, Красный Шторм.
	// Каждая система — свой изолированный мир; Router направляет игроков и выполняет гиперпереходы.
	gameWorld := world.New()
	playerStore := world.NewPGStore(pool)
	var universe *world.Router
	var quests *quest.Service
	if cfg.FeatureQuests {
		quests = quest.NewService(log, quest.NewPGStore(pool), gameWorld.ObjectPos)
		if err := quests.Init(ctx); err != nil {
			return err
		}
		log.Info("контракты включены (COSMOC_FEATURE_QUESTS)")
	}
	// конфигурация мира («живые локации»): версии в базе; активная применяется к миру на лету, если функция включена флагом
	worldCfg := worldcfg.NewService(worldcfg.NewPGStore(pool), func(wc worldgen.Config) {
		if !cfg.FeatureLiving || len(wc.Systems) == 0 || universe == nil {
			return
		}
		for _, sp := range wc.Systems {
			if w := universe.WorldOf(sp.ID); w != nil {
				if err := w.ApplyLayout(worldgen.Generate(sp, wc), wc); err != nil {
					log.Error("конфигурация мира не применена", "system", sp.ID, "err", err)
				}
			}
		}
		log.Info("живые локации применены", "systems", len(wc.Systems))
	})
	if err := worldCfg.Init(ctx); err != nil {
		return err
	}
	// вселенная: мир на каждую систему из активной конфигурации; главный — первая (Аврора)
	{
		active := worldgen.DefaultConfig()
		if raw, _, err := worldCfg.Active(ctx); err == nil && len(raw) > 0 {
			var wc worldgen.Config
			// Э-1: конфигурации эпохи одной системы («Кси») больше не применяем: берём встроенные три системы
			if json.Unmarshal(raw, &wc) == nil && len(wc.Systems) >= 3 && wc.Systems[0].ID != "xi" {
				active = wc
			} else if err == nil && len(raw) > 0 {
				log.Info("опубликованная конфигурация эпохи одной системы не применена, используются встроенные три системы")
			}
		}
		universe = world.NewRouter(gameWorld)
		for _, sp := range active.Systems {
			w := gameWorld
			if sp.ID != active.Systems[0].ID {
				w = world.New()
			}
			lay := worldgen.Generate(sp, active)
			if err := w.ApplyLayout(lay, active); err != nil {
				return fmt.Errorf("раскладка системы %s: %w", sp.ID, err)
			}
			universe.Add(w)
			gates := make([]world.Gate, 0, len(sp.Jumps))
			for _, j := range sp.Jumps {
				gates = append(gates, world.Gate{To: j.To, Pos: world.Vec{X: j.Pos[0], Y: j.Pos[1]}})
			}
			universe.SetGates(sp.ID, gates)
			log.Info("система готова", "id", sp.ID, "переходы", len(gates))
		}
	}
	// каталог кораблей: версии в базе, активный каталог применяется к миру на лету
	ships := shipcatalog.NewService(shipcatalog.NewPGStore(pool), gameWorld.SetCatalog)
	if err := ships.Init(ctx); err != nil {
		return err
	}
	// First RPG integration is metadata for the current Strizh only. Active
	// catalog values remain authoritative; no draft models are published.
	passports, err := shipcatalog.DefaultPassportCatalog()
	if err != nil {
		return fmt.Errorf("паспорта RPG: %w", err)
	}
	chatSvc := chat.NewService(chat.NewPGStore(pool))
	if err := chatSvc.Load(ctx); err != nil {
		return err
	}
	go func() { // история чата хранится ограниченный срок (настройка retention_days)
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				if n, err := chatSvc.Purge(pctx); err != nil {
					log.Error("чат: очистка истории", "err", err)
				} else if n > 0 {
					log.Info("чат: удалены старые сообщения", "count", n)
				}
				cancel()
			}
		}
	}()
	var designSvc *design.Service
	{
		var signer *design.Signer
		if cfg.AssetSignKeyFile != "" {
			sg, err := design.LoadSigner(cfg.AssetSignKeyFile)
			if err != nil {
				log.Error("ключ подписи ресурсов не загружен: публикация ресурсов выключена", "err", err)
			} else {
				signer = sg
			}
		}
		ds, err := design.NewService(design.NewPGStore(pool), signer)
		if err != nil {
			return fmt.Errorf("библиотека дизайна: %w", err)
		}
		designSvc = ds
		log.Info("библиотека дизайна готова", "публикация", signer != nil)
	}
	// корпорации минимум (R-64): взнос списывается через quest-сервис
	var corpSvc *corp.Service
	if quests != nil {
		corpSvc = corp.NewService(corp.NewPGStore(pool), quests.Spend)
	}

	wsHandler := ws.NewHandler(log, ws.Options{
		MaxMessage: cfg.WSMaxMessage, RatePerSec: cfg.WSRatePerSec, Version: version,
		World: universe, Store: playerStore, ShipPassports: &passports, Quests: quests, Corp: corpSvc, Accounts: account.NewService(playerStore), RequireCharacter: true, Shop: shop.NewService(shop.NewPGStore(pool)), Chat: chatSvc, Pilot: pilot.NewService(pilot.NewPGStore(pool)), Ships: shipxp.NewService(shipxp.NewPGStore(pool)),
	})
	var agent *aiagent.Manager
	if cfg.FeatureAI {
		studioSvc := aistudio.NewService(aistudio.NewPGStore(pool))
		agent = aiagent.NewManager(gameWorld, aiagent.StudioSource{Svc: studioSvc}, aiagent.NewPGStore(pool), log) // ИИ-1 живёт в Авроре (Э-1)
		// ИИ-2: ответы капитанов в общем и личном чате из обучения AI Studio
		dlg := aiagent.NewDialogue(studioSvc, chatSvc, agent, log)
		dlg.Push = wsHandler.PushChat // реплики ИИ должны доходить до живых клиентов, а не только попадать в историю
		wsHandler.SetOnChat(func(m chat.Message) { dlg.Handle(context.Background(), m) })
		go dlg.RunAmbient(ctx) // ИИ пишут в общий чат сами, пока в мире есть живые игроки
		// личные сообщения ИИ-капитанам по имени из мира
		chatSvc.Directory = func(ctx context.Context, q string) (chat.Person, bool) {
			c, ok := agent.Lookup(q)
			if !ok {
				return chat.Person{}, false
			}
			return chat.Person{ID: c.ID, Name: c.Name, AI: true}, true
		}
		go agent.Run(ctx, 2*time.Second)
		log.Info("ИИ-игроки включены (COSMOC_FEATURE_AI), реплики в чате (ИИ-2)")
	}
	worldDone := make(chan struct{})
	go func() {
		wsHandler.Run(ctx) // симуляция мира; при остановке сохраняет позиции всех кораблей
		close(worldDone)
	}()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           gameMux(pool, version, wsHandler, designSvc),
		ReadHeaderTimeout: 5 * time.Second,
	}

	servers := []*http.Server{srv}
	errCh := make(chan error, 2)
	serve := func(name string, s *http.Server) {
		log.Info("HTTP слушает", "server", name, "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("%s: %w", name, err)
		}
	}
	go serve("game", srv)

	// Админ-API на отдельном адресе: наружу не публикуется, доступен через Caddy с HTTPS.
	if cfg.AdminAddr != "" {
		store := admin.NewStore(pool)
		adminSrv := &http.Server{
			Addr: cfg.AdminAddr,
			Handler: admin.NewHandler(store, log, admin.Options{
				SessionTTL: cfg.AdminSessionTTL, CookieSecure: cfg.AdminCookieSecure, TrustProxy: cfg.AdminTrustProxy,
				Version: version, StartedAt: time.Now(), WSSessions: wsHandler.Sessions,
				AI: aistudio.NewService(aistudio.NewPGStore(pool)), Agent: agent, Ships: ships, World: worldCfg, LivingEnabled: cfg.FeatureLiving, Chat: chatSvc, Design: designSvc,
			}).Routes(),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       60 * time.Second, // загрузка ZIP до 16 МБ
			WriteTimeout:      60 * time.Second,
		}
		servers = append(servers, adminSrv)
		go serve("admin", adminSrv)
		go cleanupSessions(ctx, log, store)
	} else {
		log.Info("админ-API выключен (COSMOC_ADMIN_ADDR=off)")
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал остановки")
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutCancel()
	for _, s := range servers {
		if err := s.Shutdown(shutCtx); err != nil {
			return err
		}
	}
	select {
	case <-worldDone:
	case <-shutCtx.Done():
	}
	log.Info("сервер остановлен корректно")
	return nil
}

// cleanupSessions периодически удаляет просроченные сессии администраторов.
func cleanupSessions(ctx context.Context, log *slog.Logger, store *admin.Store) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := store.CleanupSessions(ctx); err != nil {
				log.Error("очистка сессий администраторов", "err", err)
			} else if n > 0 {
				log.Info("удалены просроченные сессии администраторов", "count", n)
			}
		}
	}
}

// gameMux — публичные маршруты игрового порта: здоровье, WebSocket и раздача подписанных ресурсов дизайна (/assets/).
func gameMux(db httpapi.Pinger, version string, ws http.Handler, d *design.Service) *http.ServeMux {
	mux := httpapi.NewMux(db, version, ws)
	if d != nil {
		mux.Handle("/assets/", d.PublicHandler())
	}
	return mux
}
