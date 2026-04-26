Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

$buildDir = "build"
if (-not (Test-Path $buildDir)) {
    New-Item -ItemType Directory -Name $buildDir
}

Write-Host "Building for Windows amd64..."
go build -o $buildDir/max2tg-windows-amd64.exe main.go

Write-Host "Building for Linux amd64..."
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o $buildDir/max2tg-linux-amd64 main.go

Write-Host "Building for Linux arm64..."
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o $buildDir/max2tg-linux-arm64 main.go

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Write-Host "Done!"