$files = @(
  'dashboard.html',
  'notifications.html',
  'profile.html',
  'profile_holland.html',
  'settings.html',
  'profile_rmib.html',
  'profile_papi.html',
  'profile_learning_style.html',
  'profile_kraepelin.html',
  'hasil_tes.html'
)

$old = '<a href="/profile/holland"><i data-lucide="briefcase" class="ds-icon"></i> Career Center</a>'
$new = '<a href="/profile"><i data-lucide="user" class="ds-icon"></i> Data Profile</a>'

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { continue }
  $bytes = [System.IO.File]::ReadAllBytes((Resolve-Path $p))
  if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
    $bytes = $bytes[3..($bytes.Length - 1)]
  }
  $text = [System.Text.Encoding]::UTF8.GetString($bytes)
  if ($text.Contains($old)) {
    $text = $text.Replace($old, $new)
    [System.IO.File]::WriteAllText((Resolve-Path $p), $text, $utf8NoBom)
    Write-Host "patched $f"
  } else {
    Write-Host "skip    $f"
  }
}
