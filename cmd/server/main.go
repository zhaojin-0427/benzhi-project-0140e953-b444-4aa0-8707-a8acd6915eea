package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"specimen-custody-gate/internal/application"
	"specimen-custody-gate/internal/httpapi"
	"specimen-custody-gate/internal/persistence"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "服务失败:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	if cfg.selfcheck {
		temp, err := os.MkdirTemp("", "specimen-custody-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		cfg.dataDir = temp
	}
	store, err := persistence.Open(cfg.dataDir)
	if err != nil {
		return fmt.Errorf("恢复存储: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	api := httpapi.New(application.NewService(store), logger)
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.address, err)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logger.Info("样本监管服务已启动", "address", listener.Addr().String(), "selfcheck", cfg.selfcheck)
	if cfg.selfcheck {
		return runBoundedSelfcheck(server, listener.Addr().String(), serveErrors)
	}
	return waitForShutdown(server, serveErrors, logger)
}

func runBoundedSelfcheck(server *http.Server, address string, serveErrors <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	checkErr := httpapi.RunSelfcheck(ctx, address)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	serveErr := <-serveErrors
	if checkErr != nil {
		return checkErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	fmt.Println("selfcheck: 离场预检、完整流程、整改修订、幂等重放、冻结保护和凭证深度校验均通过")
	return nil
}

func waitForShutdown(server *http.Server, serveErrors <-chan error, logger *slog.Logger) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case sig := <-signals:
		logger.Info("收到关闭信号", "signal", sig.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	err := <-serveErrors
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
