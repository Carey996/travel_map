<#
.SYNOPSIS
  用 Kubernetes 清单拉起整套服务
.DESCRIPTION
  有两种模式：
    有 kubectl 时  -> kustomize 应用到集群（生产路径）
    没有 kubectl 时 -> podman play kube 在本地跑起同一个 Pod（验证路径）
  为了让本地验证真正可行，这个脚本只做到"起得来、能访问"，不模拟 Ingress。
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts/k8s-up.ps1
  powershell -ExecutionPolicy Bypass -File scripts/k8s-up.ps1 -Down
#>
param(
  [switch]$Down,
  [switch]$Local,
  [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

function Resolve-Podman {
  $cmd = Get-Command podman -ErrorAction SilentlyContinue
  if ($cmd -and $cmd.Source) { return $cmd.Source }
  if ($env:LOCALAPPDATA) {
    $p = Join-Path $env:LOCALAPPDATA 'Programs\Podman\podman.exe'
    if (Test-Path $p) { return $p }
  }
  throw '找不到 podman'
}
$Podman = Resolve-Podman

$Kubectl = (Get-Command kubectl -ErrorAction SilentlyContinue).Source

if (-not $Local -and $Kubectl) {
  if ($Down) {
    Write-Host '==> 删除 K8s 资源' -ForegroundColor Cyan
    & $Kubectl delete -k (Join-Path $Root 'deploy\k8s\base') --ignore-not-found
    return
  }
  Write-Host ('==> 使用 kubectl 部署到集群 (' + $Kubectl + ')') -ForegroundColor Cyan
  Write-Host '  1/2 应用迁移 Job' -ForegroundColor DarkGray
  & $Kubectl apply -f (Join-Path $Root 'deploy\k8s\base\migrate-job.yaml')
  & $Kubectl -n travel-blog wait --for=condition=complete job/travel-blog-migrate --timeout=180s
  Write-Host '  2/2 应用其余资源' -ForegroundColor DarkGray
  & $Kubectl apply -k (Join-Path $Root 'deploy\k8s\base')
  Write-Host ''
  Write-Host '查看状态：kubectl -n travel-blog get pods,svc,ingress' -ForegroundColor Green
  return
}

# ---- 本地模式：podman play kube ----
$podFile = Join-Path $Root 'deploy\k8s\local\pod.yaml'

if ($Down) {
  Write-Host '==> 停止本地 Pod' -ForegroundColor Cyan
  & $Podman play kube --down $podFile
  return
}

if (-not $SkipBuild) {
  Write-Host '==> 先构建镜像' -ForegroundColor Cyan
  & (Join-Path $Root 'scripts\k8s-build.ps1')
}

Write-Host '==> 用 podman play kube 拉起本地 Pod' -ForegroundColor Cyan
& $Podman play kube --replace $podFile
if ($LASTEXITCODE -ne 0) { throw 'podman play kube 失败' }

Write-Host ''
Write-Host '等待容器就绪…' -ForegroundColor Cyan
$deadline = (Get-Date).AddSeconds(120)
while ((Get-Date) -lt $deadline) {
  $status = & $Podman pod ps --filter name=travel-blog-local --format '{{.Status}}'
  if ($status -match 'Running') { Start-Sleep -Seconds 8; break }
  Start-Sleep -Seconds 2
}

Write-Host ''
Write-Host '--------------------------------------------------------' -ForegroundColor DarkGray
Write-Host '  打开浏览器访问：http://127.0.0.1:8080' -ForegroundColor Green
Write-Host '  MinIO 控制台：  http://127.0.0.1:9001  (minioadmin / minioadmin)' -ForegroundColor DarkGray
Write-Host '  查看容器：      podman pod ps && podman ps --pod' -ForegroundColor DarkGray
Write-Host '  查看日志：      podman logs travel-blog-local-api' -ForegroundColor DarkGray
Write-Host '  停止：          scripts\k8s-up.ps1 -Down' -ForegroundColor DarkGray
Write-Host '--------------------------------------------------------' -ForegroundColor DarkGray
