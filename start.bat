@echo off
echo ========================================
echo Goban - B站评论监控系统
echo ========================================
echo.
echo 启动后端服务...
start cmd /k "cd /d %~dp0\server && title Goban Backend && goban.exe"
echo.
echo 后端服务已启动在 http://localhost:8080
echo.
echo 如需启动前端，请在web目录执行:
echo   cd web
echo   npm install
echo   npm run dev
echo.
echo 前端地址: http://localhost:3000
echo 默认账号: admin / admin123
echo.
pause
