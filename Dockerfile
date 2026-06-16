# 多阶段构建 - 构建前端
FROM node:24-alpine AS frontend-builder

WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 多阶段构建 - 构建后端
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
# 使用纯Go SQLite驱动，关闭CGO便于跨平台和多架构构建
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o goban .

# 最终运行镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# 从构建阶段复制后端可执行文件
COPY --from=backend-builder /app/goban .

# 从构建阶段复制前端构建产物
COPY --from=frontend-builder /app/web/dist ./web/dist

# 设置时区
ENV TZ=Asia/Shanghai

# 暴露端口
EXPOSE 8080

# 创建数据目录
RUN mkdir -p /app/data

# 设置环境变量
ENV PORT=8080
ENV GOBAN_USERNAME=admin
ENV DB_PATH=/app/data/goban.db

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./goban"]
