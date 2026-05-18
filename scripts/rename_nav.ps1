$files = @(
  'dashboard.html',
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

# 1) Rename label "Assessment" -> "Tes Psikologi" pada link /test di sidebar
$oldAssess = '<a href="/test"><i data-lucide="clipboard-list" class="ds-icon"></i> Assessment</a>'
$newAssess = '<a href="/test"><i data-lucide="clipboard-list" class="ds-icon"></i> Tes Psikologi</a>'

# 2) Pindahkan link "Hasil Tes" ke /hasil-tes
$oldHasil = '<a href="/profile"><i data-lucide="bar-chart-3" class="ds-icon"></i> Hasil Tes</a>'
$newHasil = '<a href="/hasil-tes"><i data-lucide="bar-chart-3" class="ds-icon"></i> Hasil Tes</a>'

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { continue }
  $c = Get-Content $p -Raw
  $changed = $false
  if ($c.Contains($oldAssess)) { $c = $c.Replace($oldAssess, $newAssess); $changed = $true }
  if ($c.Contains($oldHasil))  { $c = $c.Replace($oldHasil,  $newHasil);  $changed = $true }
  if ($changed) {
    Set-Content -Path $p -Value $c -NoNewline -Encoding UTF8
    Write-Host "patched $f"
  } else {
    Write-Host "skip    $f"
  }
}
