<#
.SYNOPSIS
  一键启动本地开发环境（后端 API + 前端 Vite）
.DESCRIPTION
  依赖 Podman 容器（Postgres / Redis / MinIO）已在运行，见 scripts/dev-up.ps1。
  两个服务都以独立进程启动，日志写到 .logs/ 下，关闭窗口不会带停它们。
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
#>
param(
  [switch]$SkipDeps,
  [switch]$NoBrowser
)

$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$ApiDir = Join-Path $Root 'api'
$WebDir = Join-Path $Root 'web'
$Logs = Join-Path $Root '.logs'
New-Item -ItemType Directory -Force -Path $Logs | Out-Null

# ---------- 前置检查 ----------
$Python = Join-Path $ApiDir '.venv\Scripts\python.exe'
if (-not (Test-Path $Python)) {
  throw "找不到 Python 虚拟环境。请先执行：cd api; uv sync"
}

function Resolve-Node {
  # 先查 PATH，再回退到常见安装位置——不同终端环境下 PATH 未必完整
  $cmd = Get-Command node -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $candidates = @()
  if ($env:ProgramFiles) { $candidates += (Join-Path $env:ProgramFiles 'nodejs\node.exe') }
  if (${env:ProgramFiles(x86)}) { $candidates += (Join-Path ${env:ProgramFiles(x86)} 'nodejs\node.exe') }
  if ($env:LOCALAPPDATA) { $candidates += (Join-Path $env:LOCALAPPDATA 'Programs\nodejs\node.exe') }
  foreach ($c in $candidates) { if ($c -and (Test-Path $c)) { return $c } }
  return $null
}

$Node = Resolve-Node
if (-not $Node) { throw '找不到 node，请先安装 Node.js 20+ 并确保它在 PATH 中' }
Write-Host ('  使用 node: ' + $Node) -ForegroundColor DarkGray

$ViteBin = Join-Path $WebDir 'node_modules\vite\bin\vite.js'
if (-not (Test-Path $ViteBin)) {
  throw "找不到 vite。请先执行：cd web; npm install"
}

# ---------- 依赖容器 ----------
if (-not $SkipDeps) {
  Write-Host '==> 确保依赖容器已启动' -ForegroundColor Cyan
  & (Join-Path $Root 'scripts\dev-up.ps1')
  Write-Host ''
}

# ---------- 停掉占用端口的旧进程 ----------
foreach ($port in 8000, 5173) {
  Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.LocalPort -eq $port } |
    ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
}
Start-Sleep -Milliseconds 500

# ---------- 启动 ----------
Write-Host '==> 启动后端 API (http://127.0.0.1:8000)' -ForegroundColor Cyan
$api = Start-Process -FilePath $Python `
  -ArgumentList @('-m', 'uvicorn', 'app.main:app', '--host', '127.0.0.1', '--port', '8000', '--reload') `
  -WorkingDirectory $ApiDir `
  -RedirectStandardOutput (Join-Path $Logs 'api.out.log') `
  -RedirectStandardError (Join-Path $Logs 'api.err.log') `
  -PassThru -WindowStyle Hidden

Write-Host '==> 启动前端 (http://127.0.0.1:5173)' -ForegroundColor Cyan
$web = Start-Process -FilePath $Node `
  -ArgumentList @($ViteBin, '--host', '127.0.0.1', '--port', '5173') `
  -WorkingDirectory $WebDir `
  -RedirectStandardOutput (Join-Path $Logs 'vite.log') `
  -RedirectStandardError (Join-Path $Logs 'vite.err.log') `
  -PassThru -WindowStyle Hidden

# ---------- 等待就绪 ----------
function Wait-Http {
  param([string]$Url, [string]$Label, [int]$TimeoutSec = 60)
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  while ((Get-Date) -lt $deadline) {
    try {
      $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3
      if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500) {
        Write-Host ('  [ready] ' + $Label) -ForegroundColor Green
        return $true
      }
    } catch { Start-Sleep -Milliseconds 800 }
  }
  Write-Host ('  [TIMEOUT] ' + $Label) -ForegroundColor Red
  return $false
}

Write-Host ''
Write-Host '==> 等待服务就绪' -ForegroundColor Cyan
$apiOk = Wait-Http -Url 'http://127.0.0.1:8000/healthz' -Label 'API'
$webOk = Wait-Http -Url 'http://127.0.0.1:5173/' -Label 'Web'

if (-not $apiOk) {
  Write-Host '后端启动失败，最后 20 行日志：' -ForegroundColor Yellow
  Get-Content (Join-Path $Logs 'api.err.log') -Tail 20 -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host '--------------------------------------------------------' -ForegroundColor DarkGray
Write-Host '  打开浏览器访问：http://127.0.0.1:5173' -ForegroundColor Green
Write-Host '  API 文档：      http://127.0.0.1:8000/docs' -ForegroundColor DarkGray
Write-Host '  MinIO 控制台：  http://127.0.0.1:9001  (minioadmin / minioadmin)' -ForegroundColor DarkGray
Write-Host ('  进程号：        api=' + $api.Id + '  web=' + $web.Id) -ForegroundColor DarkGray
Write-Host '  日志目录：      .logs\' -ForegroundColor DarkGray
Write-Host '  停止服务：      scripts\stop-dev.ps1' -ForegroundColor DarkGray
Write-Host '--------------------------------------------------------' -ForegroundColor DarkGray

if (-not $NoBrowser -and $webOk) {
  Start-Process 'http://127.0.0.1:5173'
}
