@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: ==== 管理员权限检查 ====
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] 请以管理员身份运行此脚本！
    pause
    exit /b 1
)

:: ==== 修复 dism 路径 ====
set "sysPath=%SystemRoot%\System32"
if not exist "%sysPath%\dism.exe" (
    echo [警告] dism.exe 路径异常，尝试修复环境变量...
    set "PATH=%SystemRoot%\System32;%SystemRoot%;%SystemRoot%\System32\Wbem;%PATH%"
)

:: ==== WSL和虚拟化检测 ====
set "vmEnabled=0"
set "requiresRestart=0"

:: ==== 检测虚拟化支持 ====
echo [信息] 正在检测虚拟化支持...
sc query hvservice 2>&1 | findstr "RUNNING" >nul && set "vmEnabled=1"

if %vmEnabled% equ 1 (
    echo [成功] 虚拟化支持已启用
) else (
    echo [警告] 虚拟化未启用，尝试启用 VirtualMachinePlatform...
    %sysPath%\dism.exe /Online /Enable-Feature /FeatureName:VirtualMachinePlatform /All /NoRestart
    echo [提示] 如果启用成功，请重启系统以生效1
    set "requiresRestart=1"
)

:: ==== 检测并启用 WSL ====
echo [信息] 正在检测 WSL 状态...
wsl --status >nul 2>&1
if %errorlevel% neq 0 (
    echo [警告] WSL 未启用，正在尝试启用...
    %sysPath%\dism.exe /Online /Enable-Feature /FeatureName:Microsoft-Windows-Subsystem-Linux /All /NoRestart
    echo [操作] 设置默认 WSL 版本为 2...
    wsl --set-default-version 2 >nul
    set "requiresRestart=1"
) else (
    echo [成功] WSL 已启用
)


:: 统一重启提示
if %requiresRestart% equ 1 (
    echo.
    echo ========================================
    echo [重要] 系统需要重启才能完成组件安装！
    echo.
    :RESTART_PROMPT
    set /p choice="是否立即重启计算机? (Y/N): "
    if /I "%choice%"=="Y" (
        shutdown /r /t 0
    ) else if /I "%choice%"=="N" (
        echo.
        echo [警告] 重启操作被取消！
        echo 必须重启计算机后功能才能生效
        echo 请手动重启后再次运行此脚本
        pause
        exit /b 0
    ) else (
        echo 无效输入，请输入Y或N
        goto :RESTART_PROMPT
    )
)

:: ==== 检查 Docker 是否启动 ====
echo [信息] 检查 Docker 服务状态...
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo [错误] Docker 服务未启动或未安装
    echo 请先启动 Docker Desktop 或前往官网下载安装：
    echo     https://www.docker.com/products/docker-desktop/
    pause
    exit /b 1
) else (
    echo [成功] Docker 正在运行
)

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

if not exist ekuiper.tar (
    echo [错误] 缺少ekuiper.tar镜像文件
    pause
    exit /b 1
)

echo [信息] 检查 iotp 镜像是否已存在...
docker images --format "{{.Repository}}" | findstr luomi-iotp >nul
if !errorlevel! equ 0 (
    echo [信息] 镜像 luomi-iotp 已存在，跳过加载
) else (
    echo [信息] 正在加载镜像...
    docker load -i iotp.tar
    if !errorlevel! neq 0 (
        echo [错误] 镜像加载失败，可能原因：
        echo   1. 镜像文件损坏
        echo   2. 磁盘空间不足
        pause
        exit /b 1
    )
)

echo [信息] 检查 ekuiper 镜像是否已存在...
docker images --format "{{.Repository}}" | findstr lfedge/ekuiper >nul
if !errorlevel! equ 0 (
    echo [信息] 镜像 ekuiper 已存在，跳过加载
) else (
    echo [信息] 正在加载 ekuiper 镜像...
    docker load -i ekuiper.tar
    if !errorlevel! neq 0 (
        echo [错误] ekuiper 镜像加载失败，可能原因：
        echo   1. 镜像文件损坏
        echo   2. 磁盘空间不足
        pause
        exit /b 1
    )
)

:: ==== 容器启动 ====
echo [信息] 正在启动容器...
docker rm -f luomi-iotp 2>nul
mkdir mount\data mount\log 2>nul

docker run -d --name luomi-iotp ^
    --restart unless-stopped ^
    -p 8080:8080 -p 1883:1883 ^
    -v "%cd%\mount\data:/mosquitto/data" ^
    -v "%cd%\mount\log:/mosquitto/log" ^
    luomi-iotp:latest

if !errorlevel! neq 0 (
    echo [错误] 容器启动失败
    pause
    exit /b 1
)

docker rm -f ekuiper 2>nul

docker run -d --name ekuiper ^
    --restart unless-stopped ^
    -p 9081:9081 ^
    lfedge/ekuiper:latest

if !errorlevel! neq 0 (
    echo [警告] ekuiper容器启动失败
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
