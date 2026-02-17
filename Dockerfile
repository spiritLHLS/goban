# 多阶段构建 - 构建前端
FROM node:18-alpine AS frontend-builder

WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# 多阶段构建 - 构建后端
FROM golang:1.24-alpine AS backend-builder

# 安装构建依赖
RUN apk add --no-cache gcc g++ musl-dev sqlite-dev

WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
# 使用CGO编译以支持SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o goban .

# 最终运行镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata sqlite-libs

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
ENV USERNAME=admin
ENV PASSWORD=admin123

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/users/list || exit 1

# 启动应用
CMD ["./goban"]
