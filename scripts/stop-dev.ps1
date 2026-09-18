<#
.SYNOPSIS
  停止本地开发的前后端进程（不动 Podman 依赖容器）
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/stop-dev.ps1
#>
$ErrorActionPreference = 'SilentlyContinue'

foreach ($port in 8000, 5173) {
  $conns = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
  foreach ($c in $conns) {
    Write-Host ('  [stop] port ' + $port + ' pid ' + $c.OwningProcess)
    Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
  }
}
Write-Host '已停止 API 与前端进程（Podman 容器仍在运行）。' -ForegroundColor Green
Write-Host '若要一并停止依赖容器：scripts\dev-down.ps1' -ForegroundColor DarkGray
