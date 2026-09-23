#!/usr/bin/env python3
"""Round-trip test for the bytecode decompiler (macOS only).

Usage:

    go build -o /tmp/sv ./cmd/scriptview
    SCRIPTVIEW=/tmp/sv python3 tools/corpus/roundtrip.py /tmp/list.txt

For each compiled script in the list, decompile its bytecode with
`scriptview bytecode`, compile the result again with osacompile, and compare
the disassembly of the two. Identical bytecode means the decompiled source is
equivalent to the original. Recompile errors and bytecode diffs are written to
$ROUNDTRIP_DIR (default /tmp/scriptview-roundtrip).
"""
import collections, difflib, hashlib, os, subprocess, sys
from concurrent.futures import ThreadPoolExecutor
SV = os.environ.get('SCRIPTVIEW', 'scriptview')
OUT = os.environ.get('ROUNDTRIP_DIR', '/tmp/scriptview-roundtrip')
os.makedirs(OUT, exist_ok=True)
def dis(path):
    p = subprocess.run([SV, 'disasm', path], capture_output=True, timeout=60, stdin=subprocess.DEVNULL)
    return p.stdout.decode('utf-8', 'replace')
def check(f):
    try:
        if not open(f, 'rb').read(8).startswith(b'FasdUAS'): return 'skip'
    except Exception: return 'skip'
    b = subprocess.run([SV, 'bytecode', f], capture_output=True, timeout=60, stdin=subprocess.DEVNULL)
    if b.returncode: return 'decompile-error'
    h = hashlib.md5(f.encode()).hexdigest()[:10]
    src, dst = os.path.join(OUT, h + '.applescript'), os.path.join(OUT, h + '.scpt')
    open(src, 'wb').write(b.stdout)
    try:
        c = subprocess.run(['osacompile', '-o', dst, src], capture_output=True, timeout=60, stdin=subprocess.DEVNULL)
    except subprocess.TimeoutExpired: return 'compile-timeout'
    if c.returncode:
        open(os.path.join(OUT, h + '.err'), 'w').write(f + '\n' + c.stderr.decode('utf-8', 'replace'))
        return 'compile-error'
    x, y = dis(f), dis(dst)
    if x == y: return 'same'
    diff = difflib.unified_diff(x.splitlines(True), y.splitlines(True), 'original', 'recompiled', n=2)
    open(os.path.join(OUT, h + '.diff'), 'w').write(f + '\n' + ''.join(diff))
    return 'bytecode-differs'
files = [l.strip() for l in open(sys.argv[1]) if l.strip()]
with ThreadPoolExecutor(6) as pool:
    print(dict(collections.Counter(pool.map(check, files))))
