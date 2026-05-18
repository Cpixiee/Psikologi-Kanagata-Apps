$files = @(
  'dashboard.html',
  'notifications.html',
  'profile.html',
  'profile_holland.html',
  'profile_main.html',
  'profile_edit.html',
  'settings.html',
  'profile_rmib.html',
  'profile_papi.html',
  'profile_learning_style.html',
  'profile_kraepelin.html',
  'hasil_tes.html'
)

$old = '<a href="/settings"><i data-lucide="settings" class="ds-icon"></i> Pengaturan</a>'

$new = @'
<details class="ds-submenu">
              <summary><i data-lucide="settings" class="ds-icon"></i> Pengaturan <i data-lucide="chevron-down" class="ds-caret"></i></summary>
              <div class="ds-subnav">
                <a href="/profile/edit"><i data-lucide="user-pen" class="ds-icon"></i> Edit Profil</a>
                <a href="/settings#password"><i data-lucide="lock" class="ds-icon"></i> Ganti Password</a>
                <a href="/settings#notif"><i data-lucide="bell-ring" class="ds-icon"></i> Atur Notifikasi</a>
              </div>
            </details>
'@

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
