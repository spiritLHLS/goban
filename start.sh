#!/bin/bash

echo "========================================"
echo "Goban - B站评论监控系统"
echo "========================================"
echo ""
echo "启动后端服务..."
cd server
./goban &
BACKEND_PID=$!
echo "后端PID: $BACKEND_PID"
cd ..
echo ""
echo "后端服务已启动在 http://localhost:8080"
echo ""

# 等待后端启动
sleep 2

echo "启动前端服务..."
cd web
npm run dev &
FRONTEND_PID=$!
echo "前端PID: $FRONTEND_PID"
echo ""
echo "前端地址: http://localhost:3000"
echo "默认账号: admin / admin123"
echo ""
echo "按 Ctrl+C 停止服务"

# 捕获退出信号
trap "kill $BACKEND_PID $FRONTEND_PID; exit" INT TERM

wait
