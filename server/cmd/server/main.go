// Command server — авторитетный игровой сервер «Звёздные рубежи».
package main

import (
	"bufio"
	"context"
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
	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
	"github.com/maillunex-lang/CosmocRPG/server/internal/config"
	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/httpapi"
	"github.com/maillunex-lang/CosmocRPG/server/internal/quest"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
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

	gameWorld := world.New()
	playerStore := world.NewPGStore(pool)
	var quests *quest.Service
	if cfg.FeatureQuests {
		quests = quest.NewService(log, quest.NewPGStore(pool), gameWorld.ObjectPos)
		if err := quests.Init(ctx); err != nil {
			return err
		}
		log.Info("контракты включены (COSMOC_FEATURE_QUESTS)")
	}
	wsHandler := ws.NewHandler(log, ws.Options{
		MaxMessage: cfg.WSMaxMessage, RatePerSec: cfg.WSRatePerSec, Version: version,
		World: gameWorld, Store: playerStore, Quests: quests, Accounts: account.NewService(playerStore), RequireCharacter: true,
	})
	worldDone := make(chan struct{})
	go func() {
		wsHandler.Run(ctx) // симуляция мира; при остановке сохраняет позиции всех кораблей
		close(worldDone)
	}()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewMux(pool, version, wsHandler),
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
				AI: aistudio.NewService(aistudio.NewPGStore(pool)),
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
