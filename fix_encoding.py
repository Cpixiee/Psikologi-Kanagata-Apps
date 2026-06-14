import os

def fix_file(filepath):
    with open(filepath, 'rb') as f:
        raw = f.read()

    # Read as utf-8 (the file is actually UTF-8 but was previously mis-read)
    content = raw.decode('utf-8', errors='replace')

    # These corrupted strings occur because utf-8 emoji bytes got mojibake'd
    # Brain emoji 0xF09FA780 -> corrupted  
    # Target emoji 0xF09F8EAF -> corrupted
    # etc.
    # We need to detect and replace the garbled sequences

    replacements = [
        # Brain emoji corrupted 
        ('\xf0\x9f\xa7\xa0', '<i data-lucide="brain" style="width:20px;height:20px;color:#7c3aed;"></i>'),
        # Target emoji
        ('\xf0\x9f\x8e\xaf', '<i data-lucide="target" style="width:20px;height:20px;color:#2563eb;"></i>'),
        # Books emoji
        ('\xf0\x9f\x93\x9a', '<i data-lucide="book-open" style="width:20px;height:20px;color:#059669;"></i>'),
        # People emoji
        ('\xf0\x9f\x91\xa5', '<i data-lucide="users" style="width:20px;height:20px;color:#db2777;"></i>'),
        # Clipboard emoji
        ('\xf0\x9f\x93\x8b', '<i data-lucide="clipboard-list" style="width:20px;height:20px;color:#0e7490;"></i>'),
    ]

    # Try raw bytes approach
    raw2 = raw
    for bad_bytes_str, good in replacements:
        bad_bytes = bad_bytes_str.encode('raw_unicode_escape')
        good_bytes = good.encode('utf-8')
        raw2 = raw2.replace(bad_bytes_str.encode('latin-1', errors='ignore'), good_bytes)

    content = raw2.decode('utf-8', errors='replace')

    # Fix en-dash (U+2013 -> &ndash;) - appears in file as the raw UTF-8 bytes but viewed as mojibake
    content = content.replace('\u2013', '--')
    content = content.replace('\u2014', '--')
    content = content.replace('\u2022', '*')

    # Fix remaining visible corruptions using string replacement
    # These are the actual mojibake strings visible in the file
    moji_fixes = [
        ('60\u00e2\u20ac\u201990 Menit', '60-90 Menit'),
        ('30\u00e2\u20ac\u201945 Menit', '30-45 Menit'),
        ('15\u00e2\u20ac\u201920 Menit', '15-20 Menit'),
        ('25\u00e2\u20ac\u201935 Menit', '25-35 Menit'),
        ('20\u00e2\u20ac\u201930 Menit', '20-30 Menit'),
        ('\u00e2\u20ac\xa2', ' - '),
        ('Akurat \u00e2\u20ac\xa2 Ilmiah \u00e2\u20ac\xa2 Terpercaya', 'Akurat - Ilmiah - Terpercaya'),
        # Stopwatch
        ('\u00e2\xb1', '&#9203;'),
    ]

    for bad, good in moji_fixes:
        content = content.replace(bad, good)

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)
    
    print("Fixed: " + filepath)

fix_file('views/hasil_tes.html')
print("All done!")
