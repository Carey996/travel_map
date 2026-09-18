<#
.SYNOPSIS
  停止本地开发依赖容器
.PARAMETER Purge
  连同容器、数据卷一起删除（会清空所有本地数据）
.EXAMPLE
  pwsh -File scripts/dev-down.ps1
  pwsh -File scripts/dev-down.ps1 -Purge
#>
param([switch]$Purge)

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

$Names   = @('travelblog-postgres', 'travelblog-redis', 'travelblog-minio')
$Volumes = @('travelblog-pgdata', 'travelblog-redisdata', 'travelblog-miniodata')
$Network = 'travelblog-net'

foreach ($n in $Names) {
  podman container exists $n 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { continue }
  if ($Purge) {
    Write-Host ('  [remove] {0}' -f $n)
    podman rm -f $n | Out-Null
  } else {
    Write-Host ('  [stop]   {0}' -f $n)
    podman stop $n | Out-Null
  }
}

if ($Purge) {
  foreach ($v in $Volumes) {
    podman volume exists $v 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
      Write-Host ('  [remove] volume {0}' -f $v)
      podman volume rm -f $v | Out-Null
    }
  }
  podman network exists $Network 2>&1 | Out-Null
  if ($LASTEXITCODE -eq 0) {
    Write-Host ('  [remove] network {0}' -f $Network)
    podman network rm -f $Network 2>&1 | Out-Null
  }
  Write-Host '已清理全部容器 / 卷 / 网络。' -ForegroundColor Yellow
} else {
  Write-Host '已停止容器（数据卷保留，下次 dev-up 会复用）。' -ForegroundColor Green
}
