package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	Addr      string
	DBPath    string
	SelfCheck bool
}

func defaultAddress() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return "127.0.0.1:19081"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "127.0.0.1:19081"
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(n))
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("stagecaption", flag.ContinueOnError)
	cfg := config{}
	fs.StringVar(&cfg.Addr, "addr", defaultAddress(), "HTTP 监听地址")
	fs.StringVar(&cfg.DBPath, "db", "stagecaption.db", "SQLite 数据库路径")
	fs.BoolVar(&cfg.SelfCheck, "selfcheck", false, "通过真实 HTTP 完成有界自检")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("存在无法识别的参数")
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return cfg, fmt.Errorf("监听地址必须是 host:port：%w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return cfg, fmt.Errorf("监听地址必须使用回环主机，拒绝 %q", host)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return cfg, fmt.Errorf("监听端口不合法")
	}
	return cfg, nil
}
