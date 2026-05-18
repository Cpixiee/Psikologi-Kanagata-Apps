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

$newAside = @'
<aside id="layout-menu" class="layout-menu menu-vertical menu bg-menu-theme ds-sidebar">
  <div class="ds-brand">
    <img class="ds-brand-logo" src="/static/icons/icon_psikologi_kanagata.png" alt="PsychoWellness" />
    <div class="ds-brand-text">
      <div class="name">PsychoWellness</div>
      <div class="sub">AI Analytics System</div>
    </div>
  </div>
  <nav class="ds-nav">
    <a href="/dashboard"><i data-lucide="layout-dashboard" class="ds-icon"></i> Dashboard</a>
    <a href="/test"><i data-lucide="clipboard-list" class="ds-icon"></i> Assessment</a>
    <a href="/profile"><i data-lucide="bar-chart-3" class="ds-icon"></i> Hasil Tes</a>
    <a href="/profile/holland"><i data-lucide="briefcase" class="ds-icon"></i> Career Center</a>
    <a href="/notifications"><i data-lucide="bell" class="ds-icon"></i> Notifikasi</a>
    <a href="/settings"><i data-lucide="settings" class="ds-icon"></i> Pengaturan</a>
    <a href="#" id="ds-logout-btn"><i data-lucide="log-out" class="ds-icon"></i> Logout</a>
  </nav>
  <div class="ds-footer">&copy; 2026 Psychee Wellness<br />All rights reserved.</div>
</aside>
'@

$asideRegex = '(?s)<aside\s+id="layout-menu"[^>]*>.*?</aside>'

$oldOverlay = '<link rel="stylesheet" href="/static/modern/sneat_sidebar_dashboard.css?v=20260515a" />'
$newCss     = '<link rel="stylesheet" href="/static/modern/ds_sidebar.css?v=20260515a" />'
$lucideTag  = '<script src="https://unpkg.com/lucide@latest/dist/umd/lucide.min.js"></script>'
$sidebarJs  = '<script src="/static/js/ds_sidebar.js?v=20260515a"></script>'

foreach ($f in $files) {
  $p = Join-Path 'views' $f
  if (-not (Test-Path $p)) { Write-Host "MISSING $f"; continue }
  $c = Get-Content $p -Raw

  $matches = [regex]::Matches($c, $asideRegex)
  if ($matches.Count -eq 0) { Write-Host "NO-ASIDE $f"; continue }
  $c = [regex]::Replace($c, $asideRegex, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $newAside }, 1)

  # Swap CSS link: overlay -> ds_sidebar.css (jika overlay belum di-inject sebelumnya, sisipkan sesudah sidebar-modern.css)
  if ($c.Contains($oldOverlay)) {
    $c = $c.Replace($oldOverlay, $newCss)
  } elseif (-not $c.Contains('ds_sidebar.css')) {
    $anchor = '<link rel="stylesheet" href="/static/sneat/assets/css/sidebar-modern.css" />'
    if ($c.Contains($anchor)) {
      $c = $c.Replace($anchor, $anchor + "`r`n    " + $newCss)
    }
  }

  # Sisipkan Lucide + ds_sidebar.js sebelum </body> jika belum ada
  if (-not $c.Contains('lucide@latest')) {
    $c = $c.Replace('</body>', "    $lucideTag`r`n    $sidebarJs`r`n  </body>")
  } elseif (-not $c.Contains('ds_sidebar.js')) {
    $c = $c.Replace('</body>', "    $sidebarJs`r`n  </body>")
  }

  Set-Content -Path $p -Value $c -NoNewline -Encoding UTF8
  Write-Host "replaced $f"
}
