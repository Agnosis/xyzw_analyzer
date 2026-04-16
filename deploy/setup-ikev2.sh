#!/bin/bash
# IKEv2/IPSec PSK VPN 一键部署
# Huawei 手机原生支持，无需安装任何 App
# 在本机执行: bash deploy/setup-ikev2.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
. "$SCRIPT_DIR/deploy.conf"

VPN_SERVER_IP="$REMOTE_HOST"
VPN_SUBNET="10.9.0.0/24"
VPN_POOL_START="10.9.0.10"
VPN_POOL_END="10.9.0.99"
VPN_LOCAL_IP="10.9.0.1"
SNI_BRIDGE_PORT="12346"

# 生成随机 PSK
VPN_PSK=$(openssl rand -base64 24 | tr -d '/+=' | head -c 32)

echo "=== 1. 安装 strongSwan ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
set -e
DEBIAN_FRONTEND=noninteractive apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y strongswan strongswan-pki libcharon-extra-plugins
ENDSSH

echo "=== 2. 配置 IKEv2 ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" \
    VPN_SERVER_IP="$VPN_SERVER_IP" \
    VPN_SUBNET="$VPN_SUBNET" \
    VPN_POOL_START="$VPN_POOL_START" \
    VPN_POOL_END="$VPN_POOL_END" \
    VPN_LOCAL_IP="$VPN_LOCAL_IP" \
    VPN_PSK="$VPN_PSK" \
    SNI_BRIDGE_PORT="$SNI_BRIDGE_PORT" \
    'bash -s' << 'ENDSSH'
set -e

IFACE=$(ip route | grep default | awk '{print $5}' | head -1)
echo "Main interface: $IFACE"

# ipsec.conf
cat > /etc/ipsec.conf << EOF
config setup
    charondebug="ike 1, knl 1, cfg 0"
    uniqueids=no

conn ikev2-vpn
    auto=add
    compress=no
    type=tunnel
    keyexchange=ikev2
    fragmentation=yes
    forceencaps=yes
    dpdaction=clear
    dpddelay=300s
    rekey=no
    left=%any
    leftid=$VPN_SERVER_IP
    leftauth=psk
    leftsubnet=0.0.0.0/0
    right=%any
    rightauth=psk
    rightsourceip=$VPN_POOL_START-$VPN_POOL_END
    rightdns=223.5.5.5
EOF

# ipsec.secrets
cat > /etc/ipsec.secrets << EOF
$VPN_SERVER_IP %any : PSK "$VPN_PSK"
EOF
chmod 600 /etc/ipsec.secrets

# 开启 IP 转发
echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-ikev2.conf
sysctl -p /etc/sysctl.d/99-ikev2.conf

# iptables 规则
# 互联网访问（MASQUERADE）
iptables -t nat -C POSTROUTING -s 10.9.0.0/24 -o "$IFACE" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s 10.9.0.0/24 -o "$IFACE" -j MASQUERADE

iptables -C FORWARD -s 10.9.0.0/24 -j ACCEPT 2>/dev/null || \
    iptables -A FORWARD -s 10.9.0.0/24 -j ACCEPT

iptables -C FORWARD -d 10.9.0.0/24 -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || \
    iptables -A FORWARD -d 10.9.0.0/24 -m state --state ESTABLISHED,RELATED -j ACCEPT

# MITM: 把 VPN 客户端的 443 流量重定向到 SNI 桥接程序
iptables -t nat -C PREROUTING -s 10.9.0.0/24 -p tcp --dport 443 -j REDIRECT --to-ports "$SNI_BRIDGE_PORT" 2>/dev/null || \
    iptables -t nat -A PREROUTING -s 10.9.0.0/24 -p tcp --dport 443 -j REDIRECT --to-ports "$SNI_BRIDGE_PORT"

echo "iptables rules applied"
ENDSSH

echo "=== 3. 启动 strongSwan ==="
ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" 'bash -s' << 'ENDSSH'
set -e
systemctl enable --now strongswan-starter 2>/dev/null || systemctl enable --now strongswan
systemctl restart strongswan-starter 2>/dev/null || systemctl restart strongswan
sleep 2
ipsec status
ENDSSH

echo ""
echo "============================================="
echo "  IKEv2 VPN 配置成功！"
echo "============================================="
echo ""
echo "  手机端配置（华为：设置 → 更多连接 → VPN → 添加）："
echo ""
echo "  名称:         xyzw-vpn"
echo "  类型:         IKEv2/IPSec PSK"
echo "  服务器地址:   $VPN_SERVER_IP"
echo "  IPSec 预共享密钥: $VPN_PSK"
echo ""
echo "  阿里云安全组需放行:"
echo "    UDP 500   (IKEv2)"
echo "    UDP 4500  (IKEv2 NAT穿透)"
echo "============================================="
