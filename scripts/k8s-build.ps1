<#
.SYNOPSIS
  构建前后端容器镜像（供 Kubernetes 或 podman play kube 使用）
.DESCRIPTION
  后端走"本机交叉编译 + 极简运行时镜像"的路径：国内网络下拉 golang 基础镜像经常超时，
  而 Go 交叉编译只需要一条 GOOS=linux。产物与多阶段构建完全等价。
  想要标准的从源码构建，用 deploy/containerfiles/Containerfile.api 即可。
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/k8s-build.ps1
#>
param(
  [string]$Tag = 'dev',
  [switch]$SkipWeb
)

$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$ApiDir = Join-Path $Root 'api'

function Resolve-Go {
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $candidates = @()
  if ($env:ProgramFiles) { $candidates += (Join-Path $env:ProgramFiles 'Go\bin\go.exe') }
  if ($env:LOCALAPPDATA) { $candidates += (Join-Path $env:LOCALAPPDATA 'Programs\go\bin\go.exe') }
  $candidates += 'C:\Go\bin\go.exe'
  foreach ($c in $candidates) { if ($c -and (Test-Path $c)) { return $c } }
  throw '找不到 go，请先安装 Go 1.24+'
}

function Resolve-Podman {
  $cmd = Get-Command podman -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $candidates = @()
  if ($env:LOCALAPPDATA) { $candidates += (Join-Path $env:LOCALAPPDATA 'Programs\Podman\podman.exe') }
  if ($env:ProgramFiles) { $candidates += (Join-Path $env:ProgramFiles 'RedHat\Podman\podman.exe') }
  foreach ($c in $candidates) { if ($c -and (Test-Path $c)) { return $c } }
  throw '找不到 podman'
}

$Go = Resolve-Go
$Podman = Resolve-Podman
Write-Host ('  使用 go:     ' + $Go) -ForegroundColor DarkGray
Write-Host ('  使用 podman: ' + $Podman) -ForegroundColor DarkGray

# ---- 1) 交叉编译后端 ----
Write-Host '==> 交叉编译后端（linux/amd64）' -ForegroundColor Cyan
$outDir = Join-Path $ApiDir 'bin'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$outFile = Join-Path $outDir 'server-linux'

$env:CGO_ENABLED = '0'   # 静态二进制，才能跑在 alpine 上
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
# -C 指定模块目录：脚本工作目录是仓库根，而 go.mod 在 api/ 下
& $Go build -C $ApiDir -trimpath -ldflags '-s -w' -o $outFile ./cmd/server
if ($LASTEXITCODE -ne 0) { throw '交叉编译失败' }
Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
Write-Host ('  [ok] ' + $outFile + '  ' + [math]::Round((Get-Item $outFile).Length / 1MB, 1) + ' MB') -ForegroundColor Green

# ---- 2) 后端镜像 ----
Write-Host '==> 构建后端镜像' -ForegroundColor Cyan
& $Podman build -f (Join-Path $Root 'deploy\containerfiles\Containerfile.api.local') -t ('travel-blog-api:' + $Tag) $Root
if ($LASTEXITCODE -ne 0) { throw '后端镜像构建失败' }

# ---- 3) 前端镜像 ----
if (-not $SkipWeb) {
  Write-Host '==> 构建前端镜像（npm ci + vite build，耗时较长）' -ForegroundColor Cyan
  & $Podman build -f (Join-Path $Root 'deploy\containerfiles\Containerfile.web') -t ('travel-blog-web:' + $Tag) $Root
  if ($LASTEXITCODE -ne 0) { throw '前端镜像构建失败' }
}

Write-Host ''
Write-Host '镜像已就绪：' -ForegroundColor Green
& $Podman images --format '  {{.Repository}}:{{.Tag}}  {{.Size}}' | Select-String 'travel-blog'
