// SNI Bridge: 透明代理桥接程序
// 通过读取 TLS ClientHello 中的 SNI 字段获取真实域名，
// 然后以 HTTP CONNECT 方式转发给 game-mitm 代理（携带真实域名）。
// 部署方式: iptables REDIRECT port 443 → 本程序 → game-mitm
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	listenAddr    = "0.0.0.0:12346"
	mitmProxyAddr = "127.0.0.1:12311"
)

// 游戏域名列表 - 这些域名总是走 MITM 而不是透明隧道
var gameDomains = []string{
	"xxz-xyzw.hortorgames.com",
	"hortorgames.com",
}

func isGameDomain(host string) bool {
	for _, domain := range gameDomains {
		if strings.Contains(host, domain) {
			return true
		}
	}
	return false
}

func main() {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[sni-bridge] listening on %s → mitm proxy %s\n", listenAddr, mitmProxyAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	origDst, err := getOriginalDst(conn)
	if err != nil {
		fmt.Printf("[sni-bridge] getOriginalDst error: %v\n", err)
		return
	}

	// 读取并缓存 TLS ClientHello，提取 SNI
	var buf bytes.Buffer
	sni := extractSNI(conn, &buf)

	// 优先用 SNI 作为 host（保留端口）
	host := origDst
	if sni != "" {
		_, port, err := net.SplitHostPort(origDst)
		if err == nil {
			host = net.JoinHostPort(sni, port)
		}
		fmt.Printf("[sni-bridge] SNI=%s host=%s origDst=%s\n", sni, host, origDst)
	} else {
		fmt.Printf("[sni-bridge] SNI empty, using origDst=%s\n", origDst)
	}

	// 连接到 game-mitm，发送 HTTP CONNECT
	proxy, err := net.DialTimeout("tcp", mitmProxyAddr, 5*time.Second)
	if err != nil {
		fmt.Printf("[sni-bridge] dial proxy error: %v\n", err)
		return
	}
	defer proxy.Close()

	fmt.Fprintf(proxy, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", host, host)

	// 读取代理响应头（等待 200 Connection Established）
	br := bufio.NewReader(proxy)
	line, err := br.ReadString('\n')
	if err != nil {
		fmt.Printf("[sni-bridge] read proxy response error: %v\n", err)
		return
	}
	if !strings.Contains(line, "200") {
		fmt.Printf("[sni-bridge] proxy returned: %s\n", line)
		return
	}
	// 读掉剩余的响应头
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// 双向转发（把已读取的 ClientHello 先发过去）
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(proxy, io.MultiReader(&buf, conn))
		done <- struct{}{}
	}()
	go func() {
		io.Copy(conn, br)
		done <- struct{}{}
	}()
	<-done
}

// gameDomain is the game domain that needs special handling
const gameDomain = "xxz-xyzw.hortorgames.com"

// extractSNI 读取一条 TLS 握手记录，解析 ClientHello 中的 SNI 扩展。
func extractSNI(conn net.Conn, buf *bytes.Buffer) string {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		buf.Write(hdr)
		return ""
	}
	buf.Write(hdr)

	if hdr[0] != 0x16 { // 不是 TLS Handshake
		return ""
	}

	recLen := int(binary.BigEndian.Uint16(hdr[3:5]))
	body := make([]byte, recLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		buf.Write(body)
		return ""
	}
	buf.Write(body)

	// ClientHello: type(1)=0x01 + length(3) + version(2) + random(32)
	if len(body) < 1 || body[0] != 0x01 {
		return ""
	}
	pos := 1 + 3 + 2 + 32

	// Session ID
	if pos >= len(body) {
		return ""
	}
	pos += 1 + int(body[pos])

	// Cipher Suites
	if pos+2 > len(body) {
		return ""
	}
	pos += 2 + int(binary.BigEndian.Uint16(body[pos:]))

	// Compression Methods
	if pos+1 > len(body) {
		return ""
	}
	pos += 1 + int(body[pos])

	// Extensions
	if pos+2 > len(body) {
		return ""
	}
	extTotal := int(binary.BigEndian.Uint16(body[pos:]))
	pos += 2
	end := pos + extTotal
	if end > len(body) {
		end = len(body)
	}

	for pos+4 <= end {
		extType := binary.BigEndian.Uint16(body[pos:])
		extLen := int(binary.BigEndian.Uint16(body[pos+2:]))
		pos += 4
		if extType == 0x0000 { // SNI extension
			if pos+2 <= len(body) {
				listLen := int(binary.BigEndian.Uint16(body[pos:]))
				p := pos + 2
				listEnd := p + listLen
				for p+3 <= listEnd && p+3 <= len(body) {
					nameType := body[p]
					nameLen := int(binary.BigEndian.Uint16(body[p+1:]))
					p += 3
					if nameType == 0 && p+nameLen <= len(body) {
						return string(body[p : p+nameLen])
					}
					p += nameLen
				}
			}
		}
		pos += extLen
	}
	return ""
}

// getOriginalDst 通过 SO_ORIGINAL_DST 获取 iptables REDIRECT 前的原始目标地址。
func getOriginalDst(conn net.Conn) (string, error) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not TCP")
	}
	f, err := tc.File()
	if err != nil {
		return "", err
	}
	defer f.Close()

	const SO_ORIGINAL_DST = 80
	var addr [16]byte
	addrLen := uint32(16)
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		f.Fd(),
		syscall.IPPROTO_IP,
		SO_ORIGINAL_DST,
		uintptr(unsafe.Pointer(&addr[0])),
		uintptr(unsafe.Pointer(&addrLen)),
		0,
	)
	if errno != 0 {
		return "", errno
	}
	port := binary.BigEndian.Uint16(addr[2:4])
	ip := net.IP(addr[4:8])
	return fmt.Sprintf("%s:%d", ip, port), nil
}
