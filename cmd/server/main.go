package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
	webui "stagecaption/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("StageCaption 启动失败：%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	dbPath := cfg.DBPath
	if cfg.SelfCheck {
		temporary, err := os.CreateTemp("", "stagecaption-selfcheck-*.db")
		if err != nil {
			return err
		}
		dbPath = temporary.Name()
		temporary.Close()
		defer os.Remove(dbPath)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	app := service.New(st, quality.New())
	handler := webui.New(app)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s：%w", cfg.Addr, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			serveErrors <- err
		}
		close(serveErrors)
	}()
	if cfg.SelfCheck {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		checkErr := runSelfCheck(ctx, "http://"+listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		fmt.Println("StageCaption selfcheck 通过：完整 HTTP 流程及播出包摘要验证成功")
		return nil
	}
	log.Printf("StageCaption 工作台已启动：http://%s/workbench", listener.Addr().String())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-signals:
		log.Printf("收到 %s，准备关闭", sig)
	case err := <-serveErrors:
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
