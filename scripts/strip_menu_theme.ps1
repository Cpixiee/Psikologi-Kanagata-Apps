$files = @(
  'notifications.html',
  'profile.html',
  'profile_holland.html',
  'settings.html',
  'profile_rmib.html',
  'profile_main.html',
  'profile_papi.html',
  'profile_learning_style.html',
  'profile_kraepelin.html'
)

# Buang class `bg-menu-theme` HANYA dari aside #layout-menu yang sudah di-skin (.ds-sidebar)
$old = '<aside id="layout-menu" class="layout-menu menu-vertical menu bg-menu-theme ds-sidebar">'
$new = '<aside id="layout-menu" class="layout-menu menu-vertical menu ds-sidebar">'

# Sekaligus naikkan versi CSS untuk cache-bust
$oldCss = '/static/modern/ds_sidebar.css?v=20260515a'
$newCss = '/static/modern/ds_sidebar.css?v=20260515c'
$oldJs  = '/static/js/ds_sidebar.js?v=20260515a'
$newJs  = '/static/js/ds_sidebar.js?v=20260515c'

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { continue }
  $c = Get-Content $p -Raw
  $changed = $false
  if ($c.Contains($old)) { $c = $c.Replace($old, $new); $changed = $true }
  if ($c.Contains($oldCss)) { $c = $c.Replace($oldCss, $newCss); $changed = $true }
  if ($c.Contains($oldJs))  { $c = $c.Replace($oldJs,  $newJs);  $changed = $true }
  if ($changed) {
    Set-Content -Path $p -Value $c -NoNewline -Encoding UTF8
    Write-Host "patched $f"
  } else {
    Write-Host "skip    $f"
  }
}
