#!/bin/bash
# VPN + 透明 MITM 一键部署脚本
# 在 ECS 服务器上运行: bash setup-vpn.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/deploy.conf"

SERVER_IP="$REMOTE_HOST"
VPN_SERVER_IP="10.8.0.1"
VPN_CLIENT_IP="10.8.0.2"
WG_PORT="51820"
SNI_BRIDGE_PORT="12346"
MITM_PORT="12311"

echo "=== 1. 安装依赖 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
set -e
apt-get update -qq
apt-get install -y wireguard golang-go iptables

# Go 版本检查
go version
ENDSSH

echo "=== 2. 上传 SNI 桥接程序源码 ==="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "mkdir -p /opt/sni-bridge"
scp -P "$REMOTE_PORT" \
    "$PROJECT_DIR/cmd/sni-bridge/main.go" \
    "$REMOTE_USER@$REMOTE_HOST:/opt/sni-bridge/main.go"

echo "=== 3. 编译 SNI 桥接程序 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
set -e
cd /opt/sni-bridge
if [ ! -f go.mod ]; then
    go mod init sni-bridge
fi
go build -o sni-bridge .
echo "SNI bridge compiled OK"
ENDSSH

echo "=== 4. 配置 WireGuard ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" \
    SERVER_IP="$SERVER_IP" \
    VPN_SERVER_IP="$VPN_SERVER_IP" \
    VPN_CLIENT_IP="$VPN_CLIENT_IP" \
    WG_PORT="$WG_PORT" \
    SNI_BRIDGE_PORT="$SNI_BRIDGE_PORT" \
    MITM_PORT="$MITM_PORT" \
    'bash -s' << 'ENDSSH'
set -e

mkdir -p /etc/wireguard
cd /etc/wireguard

# 生成密钥（幂等）
[ -f server.key ] || wg genkey | tee server.key | wg pubkey > server.pub
[ -f client.key ] || wg genkey | tee client.key | wg pubkey > client.pub

SERVER_PRIV=$(cat server.key)
SERVER_PUB=$(cat server.pub)
CLIENT_PUB=$(cat client.pub)

# 获取主网卡名
IFACE=$(ip route | grep default | awk '{print $5}' | head -1)
echo "Main interface: $IFACE"

cat > /etc/wireguard/wg0.conf << EOF
[Interface]
PrivateKey = $SERVER_PRIV
Address = $VPN_SERVER_IP/24
ListenPort = $WG_PORT

PostUp  = iptables -t nat -N XYZW_MITM 2>/dev/null || true; \
          iptables -t nat -F XYZW_MITM; \
          iptables -t nat -A XYZW_MITM -d 10.8.0.0/24 -j RETURN; \
          iptables -t nat -A XYZW_MITM -d 127.0.0.0/8  -j RETURN; \
          iptables -t nat -A XYZW_MITM -p tcp --dport 443 -j REDIRECT --to-ports $SNI_BRIDGE_PORT; \
          iptables -t nat -A PREROUTING -i wg0 -j XYZW_MITM; \
          iptables -A FORWARD -i wg0 -j ACCEPT; \
          iptables -t nat -A POSTROUTING -o $IFACE -j MASQUERADE

PostDown = iptables -t nat -D PREROUTING -i wg0 -j XYZW_MITM 2>/dev/null || true; \
           iptables -t nat -F XYZW_MITM 2>/dev/null || true; \
           iptables -t nat -X XYZW_MITM 2>/dev/null || true; \
           iptables -D FORWARD -i wg0 -j ACCEPT 2>/dev/null || true; \
           iptables -t nat -D POSTROUTING -o $IFACE -j MASQUERADE 2>/dev/null || true

[Peer]
PublicKey = $CLIENT_PUB
AllowedIPs = $VPN_CLIENT_IP/32
EOF

echo "WireGuard config written"
ENDSSH

echo "=== 5. 启用 IP 转发 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-wireguard.conf
sysctl -p /etc/sysctl.d/99-wireguard.conf
ENDSSH

echo "=== 6. 创建 systemd 服务 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" \
    SNI_BRIDGE_PORT="$SNI_BRIDGE_PORT" \
    'bash -s' << 'ENDSSH'

cat > /etc/systemd/system/sni-bridge.service << EOF
[Unit]
Description=SNI Bridge for MITM Proxy
After=network.target docker.service
Requires=docker.service

[Service]
ExecStart=/opt/sni-bridge/sni-bridge
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now sni-bridge
systemctl status sni-bridge --no-pager
ENDSSH

echo "=== 7. 启动 WireGuard ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
systemctl enable --now wg-quick@wg0
wg show
ENDSSH

echo "=== 8. 输出手机端配置 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" \
    SERVER_IP="$SERVER_IP" \
    VPN_CLIENT_IP="$VPN_CLIENT_IP" \
    WG_PORT="$WG_PORT" \
    'bash -s' << 'ENDSSH'
CLIENT_PRIV=$(cat /etc/wireguard/client.key)
SERVER_PUB=$(cat /etc/wireguard/server.pub)

echo ""
echo "========================================="
echo "  手机 WireGuard 配置（保存为 xyzw.conf）"
echo "========================================="
cat << EOF
[Interface]
PrivateKey = $CLIENT_PRIV
Address = $VPN_CLIENT_IP/24
DNS = 223.5.5.5

[Peer]
PublicKey = $SERVER_PUB
Endpoint = $SERVER_IP:$WG_PORT
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
EOF
echo "========================================="
echo ""
echo "提示：将上述配置复制为二维码后，手机端 WireGuard App 扫码导入"
ENDSSH

echo ""
echo "=== 部署完成 ==="
echo "阿里云安全组需要放行: UDP $WG_PORT"
