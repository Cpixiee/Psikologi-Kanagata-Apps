$ErrorActionPreference = 'Stop'

# Patroli semua HTML view (selain dashboard juga, supaya seragam).
$files = Get-ChildItem -Path views -Filter *.html -File

# Pasangan mojibake (key: byte sequence yang nyasar, value: glyph asli).
# Catatan: '$([char]0x...)' menjamin perbandingan tepat-byte, bukan literal Unicode
# yang bisa tertukar oleh editor.
$pairs = @(
  @{ bad = ([char]0x00E2 + [char]0x0153 + [char]0x2022); good = '&times;' }    # âœ• -> ×
  @{ bad = ([char]0x00E2 + [char]0x008C + [char]0x02DC); good = '&#x2318;' }   # âŒ˜ -> ⌘
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x201D); good = '&mdash;' }    # â€" (em-dash mojibake)
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x201C); good = '&ndash;' }    # â€" (en-dash mojibake)
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x2122); good = "&rsquo;" }    # â€™
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x02DC); good = "&lsquo;" }    # â€˜
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x0153); good = '&ldquo;' }    # â€œ
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x009D); good = '&rdquo;' }    # â€<C2><9D>
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x2026); good = '&hellip;' }   # â€¦
  @{ bad = ([char]0x00E2 + [char]0x20AC + [char]0x00A2); good = '&bull;' }     # â€¢
  @{ bad = ([char]0x00C2 + [char]0x00A0);                good = '&nbsp;' }     # NBSP mojibake
  @{ bad = ([char]0x00C2 + [char]0x00A9);                good = '&copy;' }     # Â©
)

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

foreach ($f in $files) {
  $bytes = [System.IO.File]::ReadAllBytes($f.FullName)
  # Buang BOM bila ada
  if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) {
    $bytes = $bytes[3..($bytes.Length - 1)]
  }
  $text = [System.Text.Encoding]::UTF8.GetString($bytes)

  $changed = $false
  foreach ($p in $pairs) {
    if ($text.Contains($p.bad)) {
      $text = $text.Replace($p.bad, $p.good)
      $changed = $true
    }
  }

  if ($changed) {
    [System.IO.File]::WriteAllText($f.FullName, $text, $utf8NoBom)
    Write-Host "fixed $($f.Name)"
  }
}
