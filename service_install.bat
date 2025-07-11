@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: ==== Docker镜像优先安装 ====
echo [信息] 正在优先安装Docker镜像...
cd /d %~dp0

:: 检查Docker服务状态
docker info >nul 2>&1
if !errorlevel! neq 0 (
    echo [错误] Docker服务未运行，请先启动Docker
    pause
    exit /b 1
)

:: 检查镜像文件
if not exist iotp.tar (
    echo [错误] 缺少iotp.tar镜像文件
    pause
    exit /b 1
)

:: 检查是否已存在镜像
docker images --format "{{.Repository}}:{{.Tag}}" | findstr luomi-iotp:2.0.20 >nul
if !errorlevel! equ 0 (
    echo [信息] 镜像 luomi-iotp:2.0.20 已存在，跳过加载
    goto :RUN_CONTAINER
)

:: 加载镜像
echo [信息] 正在加载镜像...
docker load -i iotp.tar
if !errorlevel! neq 0 (
    echo [错误] 镜像加载失败，可能原因：
    echo   1. 镜像文件损坏
    echo   2. 磁盘空间不足
    pause
    exit /b 1
)

:: ==== 容器启动 ====
:RUN_CONTAINER
echo [信息] 正在启动容器...
docker rm -f luomi-iotp 2>nul
mkdir mount\data mount\log 2>nul

docker run -d --name luomi-iotp ^
    --restart unless-stopped ^
    -p 8080:8080 -p 1883:1883 ^
    -v "%cd%\mount\data:/mosquitto/data" ^
    -v "%cd%\mount\log:/mosquitto/log" ^
    luomi-iotp:2.0.20

if !errorlevel! neq 0 (
    echo [错误] 容器启动失败
    pause
    exit /b 1
)

:: ==== 服务安装 ====
echo [信息] 安装系统服务...

:: 1. 安装主服务
if exist iotserver.exe (
    iotserver.exe -service install
    if !errorlevel! neq 0 (
        echo [警告] iotserver服务安装失败
    )
)

:: 2. 安装webScada服务（新增部分）
if exist "scada\webScada.exe" (
    pushd scada
    webScada.exe -service install
    if !errorlevel! neq 0 (
        echo [警告] webScada服务安装失败
    )
    popd
) else (
    echo [警告] 找不到scada\webScada.exe
)

:: ==== 结果验证 ====
echo.
echo [结果] 安装状态：
docker ps -f name=luomi-iotp --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
sc query iotserver >nul 2>&1 && echo iotserver服务：已安装 || echo iotserver服务：未安装
if exist "scada\webScada.exe" (
    sc query "LM Scada" >nul 2>&1 && echo webScada服务：已安装 || echo webScada服务：未安装
)

pause
