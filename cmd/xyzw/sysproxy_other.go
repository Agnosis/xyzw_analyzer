//go:build !windows

package main

func setGlobalProxy(addr, bypass string) error {
	return nil
}

func proxyOff() {}
