package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	address   string
	dataDir   string
	selfcheck bool
}

func parseConfig() (config, error) {
	var cfg config
	flag.StringVar(&cfg.address, "addr", "127.0.0.1:19081", "HTTP 监听地址")
	flag.StringVar(&cfg.dataDir, "data", ".data/specimen-custody-gate", "事件日志和投影目录")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "运行真实 HTTP 自检后退出")
	flag.Parse()
	addressExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addressExplicit = true
		}
	})
	if !addressExplicit {
		if portText := strings.TrimSpace(os.Getenv("PORT")); portText != "" {
			port, err := strconv.Atoi(portText)
			if err != nil || port < 1 || port > 65535 {
				return cfg, errors.New("PORT 必须是 1 到 65535 的端口号")
			}
			cfg.address = fmt.Sprintf("127.0.0.1:%d", port)
		}
	}
	host, portText, err := net.SplitHostPort(cfg.address)
	if err != nil {
		return cfg, fmt.Errorf("无效 -addr: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return cfg, errors.New("-addr 端口必须在 1 到 65535 之间")
	}
	if host != "127.0.0.1" {
		return cfg, errors.New("-addr 必须绑定 127.0.0.1，禁止对外网卡开放")
	}
	if cfg.dataDir == "" {
		return cfg, errors.New("-data 不能为空")
	}
	return cfg, nil
}
