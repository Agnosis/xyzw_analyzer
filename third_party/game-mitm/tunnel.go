package gamemitm

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// gameServerPublicHost 是游戏 WebSocket 公网地址，用于替换内网 IP
const gameServerPublicHost = "xxz-xyzw.hortorgames.com:443"

// isPrivateIPHost 检查 host（含端口）是否为私网 IP
func isPrivateIPHost(host string) bool {
	h, _, _ := net.SplitHostPort(host)
	if h == "" {
		h = host
	}
	return strings.HasPrefix(h, "10.") ||
		strings.HasPrefix(h, "192.168.") ||
		strings.HasPrefix(h, "172.")
}

// handleTunneling handles HTTPS tunnel requests
func (p *ProxyServer) handleTunneling(w http.ResponseWriter, r *http.Request) {
	// 修复主机名格式
	host := r.Host
	if host == "" {
		p.logger.Error("Invalid Host header in the request")
		http.Error(w, "Invalid Host header", http.StatusBadRequest)
		return
	}
	// 清理多余的斜杠
	for len(host) > 0 && host[0] == '/' {
		host = host[1:]
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.logger.Error("Hijacking not supported for this connection")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	// Get client connection
	clientConn, brw, err := hijacker.Hijack()
	if err != nil {
		p.logger.Error("Failed to hijack connection: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established to client
	if _, err := brw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		p.logger.Error("Failed to send 200 response to client: %v", err)
		return
	}
	if err := brw.Flush(); err != nil {
		p.logger.Error("Failed to flush 200 response to client: %v", err)
		return
	}

	// 私网 IP 重定向：证书用原始 host（浏览器期望的），TCP 连接用公网地址
	connectHost := host
	if isPrivateIPHost(host) {
		p.logger.Info("Redirecting private IP %s to %s", host, gameServerPublicHost)
		connectHost = gameServerPublicHost
	}

	// 检查该域名是否有注册的 handler，没有则透明隧道直接转发
	if !p.hasHandler(connectHost) {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		destConn, err := dialer.Dial("tcp", connectHost)
		if err != nil {
			p.logger.Error("Failed to connect to target server %s: %v", connectHost, err)
			return
		}
		defer destConn.Close()
		done := make(chan struct{}, 2)
		go func() { io.Copy(destConn, clientConn); done <- struct{}{} }()
		go func() { io.Copy(clientConn, destConn); done <- struct{}{} }()
		<-done
		return
	}

	// 证书用原始 host（浏览器期望的 hostname，避免 TLS 握手失败）
	cert, err := p.certManager.GetCertificateForDomain(host)
	if err != nil {
		p.logger.Error("Failed to generate certificate for %s: %v", host, err)
		return
	}

	// Create TLS config for client connection
	config := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}

	// Create TLS connection with client
	tlsConn := tls.Server(clientConn, config)
	if err := tlsConn.Handshake(); err != nil {
		p.logger.Error("TLS handshake with client failed for %s: %v", host, err)
		return
	}
	defer tlsConn.Close()

	// Create dialer with timeout
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	// Connect to destination server（使用重定向后的公网地址）
	destConn, err := dialer.Dial("tcp", connectHost)
	if err != nil {
		p.logger.Error("Failed to connect to target server %s: %v", host, err)
		return
	}
	defer destConn.Close()
	serverName := connectHost
	if h, _, err := net.SplitHostPort(serverName); err == nil {
		serverName = h
	}

	// Establish TLS connection to target server
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	}

	destTLSConn := tls.Client(destConn, tlsConfig)
	if err := destTLSConn.Handshake(); err != nil {
		p.logger.Error("TLS handshake with target server %s failed: %v", connectHost, err)
		return
	}
	defer destTLSConn.Close()

	// Process HTTPS requests
	p.proxyHTTPS(tlsConn, destTLSConn, host)
}
