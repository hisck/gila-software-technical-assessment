package platform

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"ecommerce/internal/contract"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler func(context.Context, pgx.Tx, string, contract.Event) error

const deliverySchema = `
CREATE TABLE IF NOT EXISTS inbox (
    id uuid PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS outbox (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id uuid NOT NULL UNIQUE,
    topic text NOT NULL,
    body jsonb NOT NULL
);
`

func Run(service, schema string, setup func(*pgxpool.Pool, *Cache) (http.Handler, Handler)) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", service))
	if err := run(service, schema, setup); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(service, schema string, setup func(*pgxpool.Pool, *Cache) (http.Handler, Handler)) error {
	databaseURL, brokerURL := os.Getenv("DATABASE_URL"), os.Getenv("AMQP_URL")
	if databaseURL == "" || brokerURL == "" {
		return errors.New("DATABASE_URL and AMQP_URL are required")
	}
	if _, ok := subscriptions[service]; !ok {
		return errors.New("unknown service: " + service)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	config.MaxConns = 8
	db, err := pgxpool.NewWithConfig(startup, config)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := initialize(startup, db, schema); err != nil {
		return err
	}
	cancel()

	cache := openCache(os.Getenv("REDIS_ADDR"))
	if cache != nil {
		defer cache.client.Close()
	}
	routes, handler := setup(db, cache)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		probe, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := db.Ping(probe); err != nil {
			Error(w, http.StatusServiceUnavailable, "Database unavailable")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": service})
	})
	if routes != nil {
		mux.Handle("/", routes)
	}

	workers, cancelWorkers := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() {
		reconnect(workers, brokerURL, "consumer", func(ch *brokerChannel) error {
			return consume(workers, db, service, ch, handler)
		})
	})
	if service != "search" {
		background.Go(func() {
			reconnect(workers, brokerURL, "publisher", func(ch *brokerChannel) error {
				return publishOutbox(workers, db, ch)
			})
		})
	}
	defer func() {
		cancelWorkers()
		background.Wait()
	}()

	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
			default:
				origin := r.Header.Get("Origin")
				if origin != "" && !strings.EqualFold(origin, "http://"+r.Host) {
					Error(w, http.StatusForbidden, "Cross-origin writes are unavailable")
					return
				}
			}
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	done := make(chan error, 1)
	go func() {
		slog.Info("http listening", "address", server.Addr)
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func initialize(ctx context.Context, db *pgxpool.Pool, schema string) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(721318014)"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, deliverySchema+schema)
		return err
	})
}
