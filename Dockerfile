# 阶段1：构建前端
# 前端产物与目标架构无关，固定在构建机原生平台运行，避免 arm64 在 QEMU 下执行 pnpm 时崩溃挂起
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

# 安装pnpm并构建前端
RUN npm install -g pnpm@10
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY api/ /app/api/
RUN cp -R src/api/generated /tmp/generated-sdk \
    && pnpm run openapi:generate \
    && diff -ru /tmp/generated-sdk src/api/generated
RUN pnpm run build


# 阶段2：构建后端
FROM golang:1.26.10-alpine AS backend-builder

# 安装CGO编译依赖
RUN apk add --no-cache gcc g++ musl-dev libwebp-dev

# 设置工作目录
WORKDIR /app

# 复制Go依赖文件并下载
COPY go.mod go.sum ./
RUN go mod download

# 复制后端源代码
COPY backend/ ./backend/
COPY api/ ./api/
COPY main.go ./

# 复制前端构建结果到后端可访问的路径
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# 编译后端应用（启用CGO支持webp）
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main ./main.go


# 阶段3：最终运行环境
FROM alpine:3.24

# 安装运行时依赖
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    libwebp \
    su-exec

# 设置工作目录
WORKDIR /app

# 从后端构建阶段复制二进制文件
COPY --from=backend-builder /app/main ./

# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# 根据 PUID/PGID 修正持久化目录权限并降权启动应用
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# 暴露端口
EXPOSE 8080

# 运行应用
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["./main"]
