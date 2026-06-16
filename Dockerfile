# 使用官方推荐的最新稳定版轻量级 Alpine 镜像
FROM alpine:3.20

# 1. 严谨性增强：一次性安装时区数据与 HTTPS 根证书（防止请求外部 HTTPS 接口失败）
# 同时清理 apk 缓存，确保镜像体积绝对最小化
RUN apk add --no-cache tzdata ca-certificates && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 2. 安全性增强：创建独立的非 root 运行用户，并限制其权限
# 赋予其 10001 UID，无家目录，禁止 shell 登录
RUN adduser -D -g "" -u 10001 appuser

WORKDIR /app

# 3. 从 GitHub Actions 传输过来的上下文中，复制预编译好的 ARM64 二进制文件
# 并在复制的同时，直接将所有权移交给非 root 用户
COPY --chown=appuser:appuser vmq-app /app/main
COPY --chown=appuser:appuser config.yaml /app/config.yaml

# 4. 切换到非 root 用户身份运行后续指令
USER appuser

# 暴露服务端口
EXPOSE 8081

# 5. 启动程序（确保二进制文件具有可执行权限）
CMD ["./main"]