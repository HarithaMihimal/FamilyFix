@echo off
setlocal

set "ROOT=%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo Go was not found. Install Go and make sure it is on PATH.
    exit /b 1
)

where npm >nul 2>nul
if errorlevel 1 (
    echo npm was not found. Install Node.js and make sure npm is on PATH.
    exit /b 1
)

if not exist "%ROOT%frontend\node_modules" (
    echo Frontend dependencies are missing. Run "npm install" in the frontend folder first.
    exit /b 1
)

start "FamilyFix Backend" /D "%ROOT%backend" cmd /k "go run ."
start "FamilyFix Frontend" /D "%ROOT%frontend" cmd /k "npm run dev"

echo FamilyFix is starting.
echo Frontend: http://192.168.8.173:5173
echo Backend:  http://localhost:3000
