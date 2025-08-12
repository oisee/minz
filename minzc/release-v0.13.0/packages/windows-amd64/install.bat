@echo off
echo Installing MinZ v0.13.0 'Module Revolution'...
echo.

REM Check for admin rights
net session >nul 2>&1
if %errorLevel% == 0 (
    echo Installing to C:\Program Files\MinZ...
    mkdir "C:\Program Files\MinZ" 2>nul
    copy bin\mz.exe "C:\Program Files\MinZ\" >nul
    echo MinZ installed to C:\Program Files\MinZ
    echo Please add C:\Program Files\MinZ to your PATH
) else (
    echo Installing to %USERPROFILE%\MinZ...
    mkdir "%USERPROFILE%\MinZ" 2>nul
    copy bin\mz.exe "%USERPROFILE%\MinZ\" >nul
    echo MinZ installed to %USERPROFILE%\MinZ
    echo Please add %USERPROFILE%\MinZ to your PATH
)

echo.
echo Installation complete!
echo.
echo Try these commands:
echo   mz --version              # Check version
echo   mz examples\fibonacci.minz -o fib.a80  # Compile example
echo.
echo Happy coding with MinZ!
pause
