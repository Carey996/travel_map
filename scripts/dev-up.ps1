<#
.SYNOPSIS
  启动本地开发依赖容器（Podman）：Postgres / Redis / MinIO
.DESCRIPTION
  幂等操作：已存在并运行的容器会被跳过，已存在但停止的会被启动，
  不存在的会被创建。不使用 compose provider，只依赖 podman 本体。
.EXAMPLE
  pwsh -File scripts/dev-up.ps1
#>
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# 解析 podman 可执行文件。
# 不依赖 PATH：某些终端（IDE 内置终端、计划任务、CI）不会带上 Podman 的安装目录。
# 解析成功后用同名函数遮蔽外部命令，脚本其余部分照旧直接写 `podman ...`。
# ---------------------------------------------------------------------------
function Resolve-Podman {
  $cmd = Get-Command podman -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  $candidates = @(
    (Join-Path $env:LOCALAPPDATA 'Programs\Podman\podman.exe'),
    (Join-Path $env:ProgramFiles 'RedHat\Podman\podman.exe'),
    (Join-Path ${env:ProgramFiles(x86)} 'RedHat\Podman\podman.exe'),
    'C:\Program Files\Podman\podman.exe'
  )
  foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path $candidate)) { return $candidate }
  }
  throw '找不到 podman，请先安装 Podman 并确保它在 PATH 中'
}

$PodmanExe = Resolve-Podman
function podman { & $PodmanExe @args }
Write-Host ('  使用 podman: ' + $PodmanExe) -ForegroundColor DarkGray

$Network       = 'travelblog-net'
$PgName        = 'travelblog-postgres'
$RedisName     = 'travelblog-redis'
$MinioName     = 'travelblog-minio'

$PgUser        = 'travelblog'
$PgPassword    = 'travelblog'
$PgDb          = 'travelblog'
$MinioUser     = 'minioadmin'
$MinioPassword = 'minioadmin'

$PgImage       = 'docker.io/library/postgres:16-alpine'
$RedisImage    = 'docker.io/library/redis:7-alpine'
$MinioImage    = 'quay.io/minio/minio:latest'

function Ensure-Running {
  param(
    [Parameter(Mandatory)][string]   $Name,
    [Parameter(Mandatory)][string[]] $RunArgs
  )
  podman container exists $Name 2>&1 | Out-Null
  if ($LASTEXITCODE -eq 0) {
    $running = podman container inspect $Name --format '{{.State.Running}}' 2>&1
    if ($running -eq 'true') {
      Write-Host ('  [skip]  {0} 已在运行' -f $Name) -ForegroundColor DarkGray
      return
    }
    Write-Host ('  [start] {0}' -f $Name) -ForegroundColor Yellow
    podman start $Name | Out-Null
  } else {
    Write-Host ('  [create]{0}' -f $Name) -ForegroundColor Green
    & podman run -d --restart=unless-stopped --name $Name @RunArgs | Out-Null
  }
  if ($LASTEXITCODE -ne 0) { throw "容器 $Name 启动失败" }
}

function Wait-Service {
  param(
    [string]$Label,
    [string]$Container,
    [string[]]$ProbeArgs,
    [int]$TimeoutSec = 60
  )
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  while ((Get-Date) -lt $deadline) {
    & podman exec $Container @ProbeArgs *> $null
    if ($LASTEXITCODE -eq 0) {
      Write-Host ('  [ready] {0}' -f $Label) -ForegroundColor Green
      return $true
    }
    Start-Sleep -Milliseconds 700
  }
  Write-Host ('  [TIMEOUT] {0} 未在 {1}s 内就绪' -f $Label, $TimeoutSec) -ForegroundColor Red
  return $false
}

Write-Host '==> 准备 Podman 网络与数据卷' -ForegroundColor Cyan
podman network exists $Network 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
  podman network create $Network | Out-Null
  Write-Host ('  [create] network {0}' -f $Network)
}
foreach ($vol in @('travelblog-pgdata', 'travelblog-redisdata', 'travelblog-miniodata')) {
  podman volume exists $vol 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) {
    podman volume create $vol | Out-Null
    Write-Host ('  [create] volume {0}' -f $vol)
  }
}

Write-Host '==> 启动容器' -ForegroundColor Cyan
Ensure-Running -Name $PgName -RunArgs @(
  '--network', $Network,
  '-p', '5432:5432',
  '-e', ('POSTGRES_USER={0}' -f $PgUser),
  '-e', ('POSTGRES_PASSWORD={0}' -f $PgPassword),
  '-e', ('POSTGRES_DB={0}' -f $PgDb),
  '-v', 'travelblog-pgdata:/var/lib/postgresql/data',
  $PgImage
)

Ensure-Running -Name $RedisName -RunArgs @(
  '--network', $Network,
  '-p', '6379:6379',
  '-v', 'travelblog-redisdata:/data',
  $RedisImage,
  'redis-server', '--appendonly', 'yes'
)

Ensure-Running -Name $MinioName -RunArgs @(
  '--network', $Network,
  '-p', '9000:9000',
  '-p', '9001:9001',
  '-e', ('MINIO_ROOT_USER={0}' -f $MinioUser),
  '-e', ('MINIO_ROOT_PASSWORD={0}' -f $MinioPassword),
  '-v', 'travelblog-miniodata:/data',
  $MinioImage,
  'server', '/data', '--console-address', ':9001'
)

Write-Host '==> 等待服务就绪' -ForegroundColor Cyan
$ok = $true
$ok = (Wait-Service -Label 'Postgres' -Container $PgName -ProbeArgs @('pg_isready', '-U', $PgUser, '-d', $PgDb)) -and $ok
$ok = (Wait-Service -Label 'Redis' -Container $RedisName -ProbeArgs @('redis-cli', 'ping')) -and $ok

$minioReady = $false
$deadline = (Get-Date).AddSeconds(60)
while ((Get-Date) -lt $deadline) {
  try {
    $resp = Invoke-WebRequest -Uri 'http://localhost:9000/minio/health/live' -UseBasicParsing -TimeoutSec 3
    if ($resp.StatusCode -eq 200) { $minioReady = $true; break }
  } catch {
    Start-Sleep -Milliseconds 700
  }
}
if ($minioReady) {
  Write-Host '  [ready] MinIO' -ForegroundColor Green
} else {
  Write-Host '  [TIMEOUT] MinIO 未就绪' -ForegroundColor Red
  $ok = $false
}

Write-Host ''
if ($ok) {
  Write-Host '全部依赖已就绪：' -ForegroundColor Green
} else {
  Write-Host '部分依赖未就绪，请检查上面的日志：' -ForegroundColor Yellow
}
Write-Host ('  Postgres    postgresql://{0}:{1}@localhost:5432/{2}' -f $PgUser, $PgPassword, $PgDb)
Write-Host '  Redis       redis://localhost:6379/0'
Write-Host '  MinIO API   http://localhost:9000   (bucket: travelblog，后端启动时自动创建)'
Write-Host ('  MinIO 控制台 http://localhost:9001  ({0} / {1})' -f $MinioUser, $MinioPassword)
