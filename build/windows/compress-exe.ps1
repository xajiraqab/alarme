param(
    [Parameter(Mandatory = $true)][string]$BinaryPath,
    [Parameter(Mandatory = $true)][string]$UpxPath
)

$ErrorActionPreference = "Stop"
$packedPath = "$BinaryPath.packed.exe"

Copy-Item -LiteralPath $BinaryPath -Destination $packedPath -Force
& $UpxPath --best --lzma $packedPath
if ($LASTEXITCODE -ne 0) {
    throw "UPX failed to compress $BinaryPath"
}

Move-Item -LiteralPath $packedPath -Destination $BinaryPath -Force
