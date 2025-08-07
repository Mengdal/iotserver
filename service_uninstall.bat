@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: ==== 服务卸载 ====
echo [信息] 正在卸载服务...
cd /d %~dp0

:: 卸载主服务
sc query iotserver >nul 2>&1
if !errorlevel! equ 0 (
    iotserver.exe -service uninstall
    if !errorlevel! neq 0 (
        echo [警告] iotserver服务卸载失败
    )
)

:: 卸载SCADA服务
if exist "scada\webScada.exe" (
    pushd scada
    sc query "LM Scada" >nul 2>&1
    if !errorlevel! equ 0 (
        webScada.exe -service uninstall
        if !errorlevel! neq 0 (
            echo [警告] webScada服务卸载失败
        )
    )
    popd
)

:: ==== 容器和镜像清理 ====
echo [信息] 正在清理Docker资源...
docker rm -f luomi-iotp 2>nul
docker rmi luomi-iotp:latest 2>nul
docker rm -f ekuiper 2>nul
docker rmi lfedge/ekuiper:latest 2>nul
:: ==== 验证 ====
echo.
echo [结果] 卸载完成状态:
sc query iotserver >nul 2>&1 || echo     iotserver: 已移除
if exist "scada\webScada.exe" (
    sc query "LM Scada" >nul 2>&1 || echo     webScada: 已移除
)
docker images --format "{{.Repository}}:{{.Tag}}" | findstr luomi-iotp:latest >nul || echo     luomi-iotp镜像: 已移除
docker images --format "{{.Repository}}:{{.Tag}}" | findstr lfedge/ekuiper:latest >nul || echo     luomi-iotp镜像: 已移除
pause
