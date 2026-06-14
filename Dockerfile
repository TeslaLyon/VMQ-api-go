# 阶段 1: 构建阶段
FROM golang:1.26.3-alpine AS builder

# 设置环境变量，启用 Go Modules，禁用 CGO 以确保静态编译，并指定 ARM64 架构
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm64

WORKDIR /build

# 缓存依赖 (利用 Docker 层缓存加速构建)
COPY go.mod go.sum ./
RUN go mod download

# 复制代码并构建 (注意构建路径指向了 cmd/server)
COPY . .
RUN go build -o /build/main ./cmd/server

# 阶段 2: 运行阶段
FROM alpine:latest

WORKDIR /app

# 设置时区
RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /build/main /app/main

# 暴露端口
EXPOSE 8081

# 启动命令
CMD ["./main"]