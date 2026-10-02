@echo off
rem Build gorubik natively on Windows.
rem
rem   build.cmd              debug build   -> bin\gorubik.exe (console attached)
rem   build.cmd release      release build -> bin\gorubik.exe (stripped, no console)
rem   build.cmd test         gofmt, vet, unit tests, cube-state verification
rem
rem GLFW is reached through cgo, so this needs a C compiler on PATH. The usual
rem install is:  winget install BrechtSanders.WinLibs.POSIX.UCRT
rem (check with: gcc --version). If you would rather not install a toolchain on
rem the Windows side at all, cross-compile from WSL with build.sh windows instead
rem - the project directory is shared, so the .exe lands in the same bin folder.

setlocal
rem pushd rather than cd: from a UNC path (\\wsl.localhost\...) cmd cannot
rem make a UNC the current directory, but pushd maps it to a drive letter.
pushd "%~dp0" || exit /b 1

if "%1"=="test" goto test

set LDFLAGS=
if /I "%1"=="release" set LDFLAGS=-s -w -H=windowsgui

gcc --version >nul 2>&1
if errorlevel 1 (
  echo error: gcc not found on PATH. See the comment at the top of this file. 1>&2
  popd
  exit /b 1
)

echo building bin\gorubik.exe
go build -ldflags "%LDFLAGS%" -o bin\gorubik.exe .\cmd\gorubik
if errorlevel 1 (
  popd
  exit /b 1
)
dir bin\gorubik.exe
echo run: bin\gorubik.exe
goto done

:test
gofmt -l .
if errorlevel 1 exit /b 1
go vet ./... || exit /b 1
go test ./... || exit /b 1
go run .\cmd\gorubik -verify

:done
popd
endlocal
