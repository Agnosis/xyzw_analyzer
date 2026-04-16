//go:build windows

package main

import "github.com/husanpao/game-mitm/gosysproxy"

func setGlobalProxy(addr, bypass string) error {
	return gosysproxy.SetGlobalProxy(addr, bypass)
}

func proxyOff() {
	gosysproxy.Off()
}
