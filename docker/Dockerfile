FROM golang:1.23-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 设置 Go 模块代理（国内加速）
ENV GOPROXY=https://goproxy.cn,direct

# 下载依赖（利用 layer 缓存）
COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download

# 复制源码并构建
COPY . .
RUN go build -o xyzw_analyzer ./cmd/xyzw

# ---- 运行镜像 ----
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/xyzw_analyzer .

# 持久化数据目录（脚本、备注）
VOLUME ["/app/data"]

# MITM 代理端口 / Web UI 端口
EXPOSE 12311 12582

ENV SERVER_MODE=1

CMD ["./xyzw_analyzer"]
