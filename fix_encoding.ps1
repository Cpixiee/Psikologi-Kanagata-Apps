$file = "c:\laragon\www\Psikologi_Apps\views\hasil_tes.html"

# Read as bytes to detect actual encoding
$bytes = [System.IO.File]::ReadAllBytes($file)
$content = [System.Text.Encoding]::UTF8.GetString($bytes)

# Replace corrupted duration strings with plain text (no special chars)
$content = $content.Replace("60`u{E2}`u{80}`u{93}90 Menit", "60-90 Menit")
$content = $content.Replace("30`u{E2}`u{80}`u{93}45 Menit", "30-45 Menit")
$content = $content.Replace("15`u{E2}`u{80}`u{93}20 Menit", "15-20 Menit")
$content = $content.Replace("25`u{E2}`u{80}`u{93}35 Menit", "25-35 Menit")
$content = $content.Replace("20`u{E2}`u{80}`u{93}30 Menit", "20-30 Menit")

# Replace bullet
$content = $content.Replace("`u{E2}`u{80}`u{A2}", " - ")

# Replace brain emoji block
$badBrain = [System.Text.Encoding]::UTF8.GetString([byte[]](0xC3, 0xB0, 0xC5, 0xB8, 0xC2, 0xA7, 0xC2, 0xA0))
$content = $content.Replace($badBrain, '<i data-lucide="brain" style="width:20px;height:20px;color:#7c3aed;"></i>')

[System.IO.File]::WriteAllText($file, $content, [System.Text.Encoding]::UTF8)
Write-Host "Done!"
