# ===== 阶段 1: 构建 =====
FROM golang:1.22-alpine AS builder

# 国内服务器访问不了 proxy.golang.org，改用国内镜像
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go env -w GOSUMDB=sum.golang.google.cn

WORKDIR /src

# 先复制依赖文件，最大化缓存命中
COPY go.mod go.sum ./
RUN go mod download

# 版本号通过构建参数传入（workflow 中来自 git tag）
ARG APP_VERSION=dev

# 复制源码并编译
COPY . .
# 静态链接（不需要 glibc），适合 scratch / distroless 镜像
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /out/clipsync-admin .

# ===== 阶段 2: 运行 =====
# 用 distroless 基础镜像（含 ca-certificates、tzdata、/etc/passwd）；
# 没有 shell / 没有包管理器，攻击面最小。
FROM gcr.io/distroless/base-debian12:nonroot

# 时区数据：让日志里 time.Local 走 UTC（distroless 默认 UTC）
ENV TZ=UTC

# 用非 root 用户（distroless nonroot tag 自带 uid 65532）
USER nonroot:nonroot

WORKDIR /app

# 拷贝编译产物和示例配置
COPY --from=builder /out/clipsync-admin /app/clipsync-admin
COPY --from=builder /src/config.example.yaml /app/config.example.yaml

# 上传文件目录（volume 挂载到这里持久化）
VOLUME ["/data/uploads"]

# 管理后端端口（与 config.example.yaml 默认值保持一致）
EXPOSE 28002

# 启动时从 /data/config/config.yaml 加载；找不到就用代码默认值
ENV CLIPSYNC_ADMIN_CONFIG=/data/config/config.yaml

ENTRYPOINT ["/app/clipsync-admin"]
CMD ["-c", "/data/config/config.yaml"]
