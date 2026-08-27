package main

import "testing"

func TestAddressConfiguration(t *testing.T) {
	t.Setenv("PORT", "19123")
	cfg, err := parseConfig(nil)
	if err != nil || cfg.Addr != "127.0.0.1:19123" {
		t.Fatalf("PORT 配置失败：%+v %v", cfg, err)
	}
	if _, err = parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("应拒绝非回环地址")
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19999"})
	if err != nil || cfg.Addr != "127.0.0.1:19999" {
		t.Fatal("-addr 配置失败")
	}
}
