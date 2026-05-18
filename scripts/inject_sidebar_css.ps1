$files = @(
  'notifications.html',
  'profile.html',
  'profile_holland.html',
  'settings.html',
  'profile_rmib.html',
  'profile_main.html',
  'profile_papi.html',
  'profile_learning_style.html',
  'profile_kraepelin.html',
  'admin_psychotest.html',
  'admin_psychotest_add_batch.html'
)

$needle = '<link rel="stylesheet" href="/static/sneat/assets/css/sidebar-modern.css" />'
$inject = $needle + "`r`n    " + '<link rel="stylesheet" href="/static/modern/sneat_sidebar_dashboard.css?v=20260515a" />'

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { Write-Host "MISSING $f"; continue }
  $c = Get-Content $p -Raw
  if ($c -match 'sneat_sidebar_dashboard\.css') { Write-Host "already $f"; continue }
  if ($c -notmatch [regex]::Escape($needle)) { Write-Host "NO-MATCH $f"; continue }
  $c2 = $c.Replace($needle, $inject)
  Set-Content -Path $p -Value $c2 -NoNewline -Encoding UTF8
  Write-Host "patched $f"
}
