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

$cards = @'
  <div class="ds-side-card">
    <div class="label">Kelas</div>
    <div class="value" id="sbKelasValue">-</div>
  </div>
  <div class="ds-side-card">
    <div class="label">Jurusan</div>
    <div class="value" id="sbJurusanValue" style="font-size:14px;">-</div>
  </div>
  <div class="ds-footer">
'@

$anchor = '<div class="ds-footer">'

# Cache-bust ulang
$oldCss = '/static/modern/ds_sidebar.css?v=20260515c'
$newCss = '/static/modern/ds_sidebar.css?v=20260515d'
$oldJs  = '/static/js/ds_sidebar.js?v=20260515c'
$newJs  = '/static/js/ds_sidebar.js?v=20260515d'

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { continue }
  $c = Get-Content $p -Raw
  $changed = $false
  if ($c -notmatch 'id="sbKelasValue"') {
    if ($c.Contains($anchor)) {
      # ganti SATU kemunculan pertama
      $idx = $c.IndexOf($anchor)
      $c = $c.Substring(0, $idx) + $cards + $c.Substring($idx + $anchor.Length)
      $changed = $true
    } else {
      Write-Host "NO-ANCHOR $f"
    }
  } else {
    Write-Host "has-cards $f"
  }
  if ($c.Contains($oldCss)) { $c = $c.Replace($oldCss, $newCss); $changed = $true }
  if ($c.Contains($oldJs))  { $c = $c.Replace($oldJs,  $newJs);  $changed = $true }
  if ($changed) {
    Set-Content -Path $p -Value $c -NoNewline -Encoding UTF8
    Write-Host "patched   $f"
  }
}
