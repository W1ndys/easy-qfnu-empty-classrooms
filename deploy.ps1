<#
.SYNOPSIS
    Go 项目自动化部署脚本 (Tar + SSH)

.DESCRIPTION
    1. 自动检测本地 SSH 密钥
    2. 检查并自动创建远程目标目录
    3. 使用 tar + ssh 管道流式上传文件（排除 .git, .env 等敏感文件）
    4. 远程执行重启命令

.EXAMPLE
    .\deploy.ps1 -Server "192.168.1.100" -RemotePath "/app/qfnu-cas"
#>

param(
    # 服务器地址
    [string]$Server = "my.server",

    # SSH 端口
    [int]$Port = 22,

    # 登录用户
    [string]$User = "root",

    # 远程部署路径 (默认: /root/easy-qfnu-empty-classrooms)
    [string]$RemotePath = "/root/easy-qfnu-empty-classrooms",

    # 部署完成后执行的命令
    [string]$RestartCmd = "supervisorctl restart easy-qfnu-empty-classrooms:easy-qfnu-empty-classrooms_00",

    # 本地项目路径
    [string]$LocalPath = ".",

    # SSH 私钥路径 (留空则自动检测)
    [string]$IdentityFile = ""
)

$ErrorActionPreference = "Stop"

# 1. 自动检测 SSH 密钥
if ([string]::IsNullOrEmpty($IdentityFile)) {
    $sshDir = "$env:USERPROFILE\.ssh"
    $possibleKeys = @("id_rsa", "id_ed25519")

    foreach ($keyName in $possibleKeys) {
        $path = Join-Path $sshDir $keyName
        if (Test-Path $path) {
            $IdentityFile = $path
            Write-Host "[-] 自动检测到 SSH 密钥: $IdentityFile" -ForegroundColor Cyan
            break
        }
    }
}

if (-not (Test-Path $IdentityFile)) {
    Write-Error "未找到 SSH 密钥，请通过 -IdentityFile 参数指定。"
}

# 构建基础 SSH 命令前缀
$sshCmdPrefix = @("ssh", "-i", "$IdentityFile", "-p", "$Port", "-o", "StrictHostKeyChecking=no", "$User@$Server")

# 2. 交叉编译 (Windows -> Linux)
Write-Host "[-] 正在编译 Linux (amd64) 二进制文件..." -ForegroundColor Cyan
$ProjectName = "easy-qfnu-empty-classrooms"
$TargetOS = "linux"
$TargetArch = "amd64"
$BinaryName = "${ProjectName}-${TargetOS}-${TargetArch}"

# 保存旧的环境变量
$OriginalGOOS = $env:GOOS
$OriginalGOARCH = $env:GOARCH

try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = $TargetOS
    $env:GOARCH = $TargetArch

    go build -ldflags "-s -w" -o $BinaryName .

    if ($LASTEXITCODE -ne 0) {
        Write-Error "编译失败，请检查 Go 环境或代码错误。"
    }
    Write-Host "[-] 编译成功: $BinaryName" -ForegroundColor Green
}
finally {
    # 恢复环境变量
    $env:GOOS = $OriginalGOOS
    $env:GOARCH = $OriginalGOARCH
}

# 3. 检查并修复远程路径 (mkdir -p)
Write-Host "[-] 正在检查/创建远程目录: $RemotePath" -ForegroundColor Cyan
$mkdirCmd = $sshCmdPrefix + "mkdir -p $RemotePath"
& $mkdirCmd[0] $mkdirCmd[1..($mkdirCmd.Length-1)]
if ($LASTEXITCODE -ne 0) {
    Write-Error "无法创建远程目录，请检查连接或权限。"
}

# 4. 使用 Tar + SSH 上传二进制文件 (通过 Git Bash)
Write-Host "[-] 正在上传二进制文件..." -ForegroundColor Cyan

# 查找 Git Bash (优先使用 Git for Windows，避免 WSL)
$gitBashPaths = @(
    "$env:ProgramFiles\Git\bin\bash.exe",
    "${env:ProgramFiles(x86)}\Git\bin\bash.exe",
    "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe",
    "$env:ProgramFiles\Git\usr\bin\bash.exe"
)

$bashExe = $null
foreach ($path in $gitBashPaths) {
    if (Test-Path $path) {
        $bashExe = $path
        break
    }
}

# 如果预设路径找不到，尝试从 PATH 中查找 Git 目录下的 bash
if (-not $bashExe) {
    $gitCmd = Get-Command git -ErrorAction SilentlyContinue
    if ($gitCmd) {
        # git.exe 通常在 Git\cmd 目录，bash 在 Git\bin 目录
        $gitDir = Split-Path (Split-Path $gitCmd.Source -Parent) -Parent
        $gitBash = Join-Path $gitDir "bin\bash.exe"
        if (Test-Path $gitBash) {
            $bashExe = $gitBash
        }
    }
}

if (-not $bashExe) {
    Write-Error "未找到 Git Bash，请确保已安装 Git for Windows。"
}

Write-Host "[-] 使用 Git Bash: $bashExe" -ForegroundColor DarkGray

# 将 Windows 路径转换为 Unix 风格路径 (用于 Git Bash)
$unixIdentityFile = $IdentityFile -replace '\\', '/' -replace '^([A-Za-z]):', '/$1'

# 构造 bash 命令：
# 1. 本地 tar 打包二进制文件
# 2. SSH 传输
# 3. 远程 tar 解压
# 4. 远程 chmod +x 赋予执行权限
$bashCmd = "tar -c $BinaryName | ssh -i '$unixIdentityFile' -p $Port -o StrictHostKeyChecking=no $User@$Server 'tar -x -C $RemotePath && chmod +x $RemotePath/$BinaryName'"

Write-Host "Executing: Upload..." -ForegroundColor DarkGray
& $bashExe -c $bashCmd

if ($LASTEXITCODE -eq 0) {
    Write-Host "[+] 文件上传成功!" -ForegroundColor Green

    # 删除本地构建产物
    if (Test-Path $BinaryName) {
        Remove-Item $BinaryName -Force
        Write-Host "[-] 已清理本地构建产物: $BinaryName" -ForegroundColor DarkGray
    }
} else {
    Write-Error "文件上传失败。"
}

# 5. 执行远程重启命令
if (-not [string]::IsNullOrEmpty($RestartCmd)) {
    Write-Host "[-] 正在执行远程命令: $RestartCmd" -ForegroundColor Cyan
    $remoteExec = $sshCmdPrefix + $RestartCmd
    & $remoteExec[0] $remoteExec[1..($remoteExec.Length-1)]

    if ($LASTEXITCODE -eq 0) {
        Write-Host "[+] 远程命令执行成功!" -ForegroundColor Green
    } else {
        Write-Warning "远程命令执行返回了非零状态码。"
    }
}

Write-Host "`n部署流程结束。" -ForegroundColor Cyan
