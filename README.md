# Goban - B站评论监控与自动举报系统

一个用于监控B站UP主视频评论区，自动匹配关键字并举报违规评论的系统。

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 16+

或使用Docker（推荐）

### 方式一：Docker部署

#### 使用Docker命令

```bash
# 拉取镜像
docker pull spiritlhl/goban:latest

# 运行容器
docker run -d \
  --name goban \
  -p 8080:8080 \
  -e USERNAME=admin \
  -e PASSWORD=admin123 \
  -e TZ=Asia/Shanghai \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  spiritlhl/goban:latest
```

#### 使用Docker Compose

1. 创建 `docker-compose.yml` 文件：

```yaml
version: '3.8'

services:
  goban:
    image: spiritlhl/goban:latest
    container_name: goban
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - USERNAME=admin
      - PASSWORD=admin123
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/app/data
```

2. 启动服务：

```bash
docker-compose up -d
```

3. 访问 `http://localhost:8080`（注意：Docker部署前后端在同一端口）


#### 环境变量说明

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| PORT | 服务端口 | 8080 |
| USERNAME | 登录用户名 | admin |
| PASSWORD | 登录密码 | admin123 |
| TZ | 时区 | Asia/Shanghai |

#### 从源码构建Docker镜像

```bash
# 克隆仓库
git clone https://github.com/spiritLHLS/goban.git
cd goban

# 构建Docker镜像
docker build -t goban:local .

# 运行容器
docker run -d \
  --name goban \
  -p 8080:8080 \
  -e USERNAME=admin \
  -e PASSWORD=admin123 \
  -v $(pwd)/data:/app/data \
  goban:local
```

### 方式二：手动部署

#### 后端部署

1. 安装依赖

```bash
cd server
go mod download
```

2. 配置环境变量（可选）

```bash
export PORT=8080              # 服务端口，默认8080
export USERNAME=admin         # 登录用户名，默认admin
export PASSWORD=admin123      # 登录密码，默认admin123
```

3. 运行后端

```bash
go run main.go
```

### 前端部署

1. 安装依赖

```bash
cd web
npm install
```

2. 开发模式

```bash
npm run dev
```

访问 http://localhost:3000

3. 生产构建

```bash
npm run build
```

## 使用说明

### 1. 登录系统

默认账号 `admin` / `admin123` 部署时请确保修改为自定义的用户名密码避免被爆破

### 2. 添加B站账号

- **扫码登录**：生成二维码，使用B站APP扫描
- **Cookie登录**：从浏览器复制Cookie直接登录

### 3. 创建监控任务

1. 选择要使用的B站账号
2. 输入要监控的UP主UID
3. 配置监控参数：
   - **视频数量**：监控该UP主最新的多少条视频（1-20条）
   - **评论数量**：每个视频检查多少条最新评论（10-200条）
   - **关键字**：多个关键字用逗号分隔
   - **检查间隔**：多久执行一次监控（秒）
4. **（推荐）配置高级选项**：
   - **代理地址**：规避单IP举报限制
     - HTTP代理：`http://127.0.0.1:7890`
     - SOCKS5代理：`socks5://127.0.0.1:1080`
   - **举报间隔**：每次举报的等待时间（默认6秒）
   - **最大重试**：API失败时的重试次数（默认3次）
   - **重试间隔**：重试的基础间隔，使用指数退避（默认2秒）
5. 创建任务后自动开始监控

### 4. 查看日志和举报记录

- **监控日志**：查看监控任务执行情况
- **举报记录**：查看所有举报详情

## API文档

### 认证

```
Authorization: Basic base64(username:password)
```

### 主要接口

- `GET /api/users/list` - 获取B站用户列表
- `GET /api/users/login` - 生成登录二维码
- `POST /api/users/loginByCookie` - Cookie登录
- `GET /api/tasks/list` - 获取监控任务
- `POST /api/tasks/create` - 创建任务
- `GET /api/logs/monitor` - 获取监控日志
- `GET /api/logs/report` - 获取举报记录

## 注意事项

1. **举报频率限制**：
   - B站单IP限制：1分钟10条举报
   - 系统默认举报间隔6秒（确保不超限）
   - 强烈建议配置代理以提高举报效率

2. **API重试机制**：
   - 所有API调用失败会自动重试
   - 使用指数退避策略（2秒→4秒→8秒...）
   - 默认最大重试3次，可在任务中自定义

3. **监控配置建议**：
   - 监控间隔建议≥300秒，避免触发B站风控
   - 视频数量：1-20条（建议5-10条）
   - 评论数量：10-200条（建议50-100条）

4. Cookie通常30天有效，过期需重新登录

5. 系统默认使用"传谣类"举报理由（reason=11）

6. 建议使用小号进行监控和举报操作

7. 代理格式：
   - HTTP: `http://host:port`
   - SOCKS5: `socks5://host:port`
   - 带认证: `http://user:pass@host:port`

8. **Docker部署**：
   - Docker部署前后端在同一端口（8080）
   - 数据库文件保存在容器内 `/app/data` 目录，建议挂载卷持久化
   - 支持多架构：amd64和arm64
   - 容器自带健康检查，确保服务可用性

## Docker镜像

### 官方镜像

```bash
docker pull spiritlhl/goban:latest
```

### 支持的标签

- `latest` - 最新稳定版本
- `v1.x.x` - 特定版本号
- `main` - 主分支最新构建

### 多架构支持

镜像支持以下架构：
- `linux/amd64` - x86_64架构
- `linux/arm64` - ARM64架构（适用于Raspberry Pi 4、Apple Silicon等）

## 致谢

参考项目：
- [bilibili-API-collect](https://github.com/AkagiYui/bilibili-API-collect)
- [gobup](https://github.com/spiritLHLS/gobup)

## 免责声明

本项目仅供学习交流使用。使用本工具产生的任何后果由使用者自行承担。
