#!/usr/bin/env python3
"""Bangun landing/index.html dari landing-src/index.src.html.

Blok <script> yang diapit penanda <!-- build:obfuscate --> ...
<!-- /build:obfuscate --> di-obfuscate memakai javascript-obfuscator
(terpasang di ~/workspace/tools/obfuscator) supaya metode ECDH +
envelope tidak terbaca mentah oleh orang iseng (permintaan Rams
2026-11-24). Ini KOSMETIK, bukan lapisan keamanan — sumber yang
bisa dibaca manusia adalah index.src.html di folder ini (SENGAJA
di luar folder landing/ agar tidak ikut terbit ke web).

Alur rilis: ubah versi unduhan di index.src.html, jalankan skrip
ini, lalu push kedua berkas (landing-src/* + landing/index.html).
"""
import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
SRC = HERE / "index.src.html"
OUT = HERE.parent / "landing" / "index.html"
OBF = (pathlib.Path.home() / "workspace" / "tools" / "obfuscator"
       / "node_modules" / ".bin" / "javascript-obfuscator")

html = SRC.read_text(encoding="utf-8")
m = re.search(
    r"<!-- build:obfuscate -->\s*<script>\n(.*?)\n</script>\s*<!-- /build:obfuscate -->",
    html, re.S)
if not m:
    sys.exit("penanda build:obfuscate tidak ditemukan di index.src.html")
js = m.group(1)

tmp_in = HERE / ".obf-in.js"
tmp_out = HERE / ".obf-out.js"
tmp_in.write_text(js, encoding="utf-8")
cmd = [str(OBF), str(tmp_in), "--output", str(tmp_out),
       "--target", "browser",
       "--compact", "true",
       "--identifier-names-generator", "hexadecimal",
       "--rename-globals", "true",
       "--string-array", "true",
       "--string-array-encoding", "base64",
       "--string-array-threshold", "0.85",
       "--string-array-rotate", "true",
       "--string-array-shuffle", "true",
       "--numbers-to-expressions", "true",
       "--simplify", "true",
       "--control-flow-flattening", "false",
       "--dead-code-injection", "false",
       "--self-defending", "false",
       "--split-strings", "false"]
r = subprocess.run(cmd, capture_output=True, text=True)
if r.returncode != 0:
    sys.exit("obfuscator gagal: " + (r.stderr or r.stdout)[-2000:])
obf = tmp_out.read_text(encoding="utf-8").strip()
tmp_in.unlink()
tmp_out.unlink()

new_html = html[:m.start()] + "<script>\n" + obf + "\n</script>" + html[m.end():]
OUT.write_text(new_html, encoding="utf-8")
print(f"index.html terbangun: {len(new_html)} byte "
      f"(skrip {len(js)} -> {len(obf)} byte)")
