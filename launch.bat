@echo off
REM YKC 啟動器（Windows）：經 WSL 執行 launch.sh
REM 需求：已安裝 WSL（wsl --install），並在其中裝好預設發行版。

where wsl >nul 2>nul
if errorlevel 1 (
  echo [YKC] 未偵測到 WSL。請先在 PowerShell 執行：wsl --install
  pause
  exit /b 1
)

for /f "usebackq tokens=*" %%i in (`wsl wslpath -a "%CD%"`) do set "WSLDIR=%%i"
echo [YKC] 正在 WSL 中啟動（%WSLDIR%）...
wsl bash -c "cd '%WSLDIR%' && bash launch.sh"
pause
