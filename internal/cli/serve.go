package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clawinfra/agent-tools/internal/api"
	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newServeCmd() *cobra.Command {
	var (
		addr   string
		dbPath string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the agent-tools registry server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, _ := zap.NewProduction()
			defer log.Sync() //nolint:errcheck

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			reg := registry.New(db, log)
			handler := api.NewHandler(reg, log)

			srv := &http.Server{
				Addr:         addr,
				Handler:      handler,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 60 * time.Second,
				IdleTimeout:  120 * time.Second,
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			go func() {
				log.Info("registry server listening", zap.String("addr", addr))
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("server error", zap.Error(err))
					cancel()
				}
			}()

			<-ctx.Done()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			log.Info("shutting down")
			return srv.Shutdown(shutdownCtx)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8433", "listen address")
	cmd.Flags().StringVar(&dbPath, "db", "./data/agent-tools.db", "SQLite database path")

	return cmd
}
