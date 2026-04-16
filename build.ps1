Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

$buildDir = "build"
if (-not (Test-Path $buildDir)) {
    New-Item -ItemType Directory -Name $buildDir
}

Write-Host "Building for Windows..."
go build -o $buildDir/main.exe main.go

Write-Host "Building for Linux..."
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o $buildDir/main main.go

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Write-Host "Done!"