@echo off

:: 默认构建参数
setlocal enabledelayedexpansion

set BINARY_NAME=sshwarden
set SOURCE=cmd/sshwarden/main.go
set DIST_DIR=./dist
set VERSION=2.0.0
set BUILD_TIME=%date% %time%

:: 日志文件
set LOG_FILE=%DIST_DIR%/build.log

:: 创建输出目录
if not exist "%DIST_DIR%" (
    mkdir "%DIST_DIR%"
)

:: 获取构建目标平台
if "%1" == "" (
    echo.
    echo 用法: build.bat [platform]
    echo.
    echo 支持的平台:
    echo   linux        构建 Linux amd64 可执行文件
    echo   windows      构建 Windows amd64 可执行文件
    echo   darwin       构建 macOS amd64 可执行文件
    echo   all          构建所有平台
    echo   clean        清理输出目录
    echo.
    exit /b 1
)

:: 清理构建
if /i "%1" == "clean" (
    echo 正在清理输出目录 %DIST_DIR% ...
    if exist "%DIST_DIR%" (
        rmdir /s /q "%DIST_DIR%"
        mkdir "%DIST_DIR%"
        echo 清理完成。
    ) else (
        echo 输出目录不存在。
    )
    exit /b 0
)

:: 构建所有平台
if /i "%1" == "all" (
    call :build_platform linux amd64
    call :build_platform windows amd64
    call :build_platform darwin amd64
    exit /b 0
)

:: 构建单个平台
if /i "%1" == "linux" call :build_platform linux amd64 & exit /b 0
if /i "%1" == "windows" call :build_platform windows amd64 & exit /b 0
if /i "%1" == "darwin" call :build_platform darwin amd64 & exit /b 0

echo 错误：不支持的平台 [%1]
exit /b 1

:: 构建函数
:build_platform
set GOOS=%1
set GOARCH=%2
set CGO_ENABLED=0

:: 设置输出路径
if /i "%GOOS%" == "windows" (
    set OUTPUT=%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.exe
) else (
    set OUTPUT=%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%
)

echo [%date% %time%] 开始构建 %GOOS%/%GOARCH%... >> "%LOG_FILE%"

:: 构建命令
go build -ldflags "-s -w -X 'main.version=%VERSION%'" -o "%OUTPUT%" "%SOURCE%"

if %ERRORLEVEL% == 0 (
    echo [%date% %time%] 构建成功: %OUTPUT% >> "%LOG_FILE%"
    echo 构建成功: %OUTPUT%
) else (
    echo [%date% %time%] 构建失败: %GOOS%/%GOARCH% >> "%LOG_FILE%"
    echo 构建失败: %GOOS%/%GOARCH%
    exit /b 1
)

:: 可选：压缩文件（需安装 UPX）
:: echo [%date% %time%] 正在压缩文件... >> "%LOG_FILE%"
:: upx --best "%OUTPUT%" >> "%LOG_FILE%" 2>&1

:: 可选：打包为压缩文件
if /i "%GOOS%" == "linux" (
    tar -czf "%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.tar.gz" -C "%DIST_DIR%" "%BINARY_NAME%_%GOOS%_%GOARCH%"
) else if /i "%GOOS%" == "darwin" (
    tar -czf "%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.tar.gz" -C "%DIST_DIR%" "%BINARY_NAME%_%GOOS%_%GOARCH%"
) else if /i "%GOOS%" == "windows" (
    pwsh -Command "Compress-Archive -Path '%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.exe' -DestinationPath '%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.zip' -Force"
)
:: tar -czf "%DIST_DIR%/%BINARY_NAME%_%GOOS%_%GOARCH%.tar.gz" -C "%DIST_DIR%" "%BINARY_NAME%_%GOOS%_%GOARCH%.exe"
echo [%date% %time%] %GOOS%/%GOARCH% 构建完成 >> "%LOG_FILE%"
echo 构建完成。
echo.
exit /b 0
