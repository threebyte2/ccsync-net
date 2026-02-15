$OutputEncoding = [System.Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Host "🚀 Starting Build for Windows..."

# 1. Setup Environment
Write-Host "🔍 Detecting Go environment..."
try {
    # 优先尝试从环境配置获取 (避免 go env 输出包含警告导致路径无效)
    $env:GOPATH = [System.Environment]::GetEnvironmentVariable('GOPATH', 'User')
    if (-not $env:GOPATH) {
        $env:GOPATH = [System.Environment]::GetEnvironmentVariable('GOPATH', 'Machine')
    }
    if (-not $env:GOPATH) {
        $rawGoPath = go env GOPATH
        # 简单清洗: 取第一行非警告内容 (防止 'go: warning' 混入)
        $env:GOPATH = ($rawGoPath -split '\r?\n' | Where-Object { $_ -and -not $_.StartsWith('go:') } | Select-Object -First 1).Trim()
    }
}
catch {
    Write-Warning "Failed to detect GOPATH: $_"
}

if (-not $env:GOPATH) {
    if (Test-Path "$HOME\go") {
        $env:GOPATH = "$HOME\go"
    }
    else {
        Write-Warning "⚠️ GOPATH warning: Could not detect valid GOPATH."
    }
}

Write-Host "✅ Using GOPATH: $env:GOPATH"

# Append to PATH safely

# Append to PATH safely
$pathsToAdd = @("$env:GOPATH\bin", "$HOME\go\bin")
foreach ($p in $pathsToAdd) {
    if ($env:PATH -notlike "*$p*") {
        $env:PATH = "$env:PATH;$p"
    }
}

# 2. Check for Wails
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Error "❌ Error: 'wails' command not found even after searching GOPATH\bin."
    Write-Host "Please ensure Wails is installed (go install github.com/wailsapp/wails/v2/cmd/wails@latest)"

    exit 1
}

# 3. Compile project
Write-Host "📂 Compiling application..."
wails build

if ($LASTEXITCODE -ne 0) {
    Write-Error "❌ Build failed."

    exit $LASTEXITCODE
}


Write-Host "📂 Building Background Service..."
go build -ldflags "-H windowsgui" -o build\bin\ccsync-service.exe .\cmd\ccsync-service

if ($LASTEXITCODE -ne 0) {
    Write-Error "❌ Service Build failed."
    exit $LASTEXITCODE
}

Write-Host ""
Write-Host "✅ Build complete!"
Write-Host "The executables can be found in: build\bin\"
Write-Host "  - ccsync-net.exe (UI)"
Write-Host "  - ccsync-service.exe (Background Service)"

Write-Host ""
#
