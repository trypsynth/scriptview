#!/usr/bin/env python3
"""Compare scriptview's decompilation against macOS osadecompile.

Usage (macOS only):

    go build -o /tmp/sv ./cmd/scriptview
    find /Library/Scripts /System/Library -name '*.scpt' > /tmp/list.txt
    SCRIPTVIEW=/tmp/sv python3 tools/corpus/compare.py /tmp/list.txt [--passlist out.txt]

Each file is decompiled by both tools; outputs are compared after dropping
blank lines and trailing whitespace. Differences are written as unified
diffs to $CORPUS_DIR/diffs (default /tmp/scriptview-corpus), and
osadecompile's output is cached in $CORPUS_DIR/osacache so reruns are fast.
Files that are plain text rather than compiled scripts are counted
separately: Script Editor reformats those by compiling them first.
"""
import collections, difflib, hashlib, os, re, shutil, subprocess, sys
from concurrent.futures import ThreadPoolExecutor
base=os.environ.get('CORPUS_DIR', '/tmp/scriptview-corpus')
out=os.path.join(base,'diffs'); cache=os.path.join(base,'osacache')
os.makedirs(cache, exist_ok=True)
shutil.rmtree(out, ignore_errors=True); os.makedirs(out)
def norm(s):
    if os.environ.get('LENIENT'): s=re.sub(r'\s*¬\n\s*', ' ', s)
    s=re.sub(r'([{\[]) +', r'\1', s)
    return [l.rstrip() for l in s.split('\n') if l.strip()]
def expected(f):
    st=os.stat(f)
    key=hashlib.md5(f'{f}|{st.st_size}|{st.st_mtime}'.encode()).hexdigest()
    p=os.path.join(cache,key)
    if os.path.exists(p): return open(p,encoding='utf-8',errors='replace').read()
    try: e=subprocess.run(['osadecompile',f],capture_output=True,timeout=60,stdin=subprocess.DEVNULL).stdout.decode('utf-8','replace')
    except Exception: e=''
    open(p,'w',encoding='utf-8').write(e)
    return e
def check(f):
    try: head=open(f,'rb').read(16)
    except Exception: return 'skip',f,None
    if not head.startswith(b'FasdUAS') and not head.startswith(b'JsOsaDAS'): return 'plaintext',f,None
    exp=expected(f)
    if not exp: return 'skip',f,None
    p=subprocess.run([os.environ.get('SCRIPTVIEW','scriptview'),'decompile',f],capture_output=True,timeout=60,stdin=subprocess.DEVNULL)
    if p.returncode: return 'crash',f,p.stderr.decode()[:200]
    got=p.stdout.decode('utf-8','replace')
    a,b=norm(got),norm(exp)
    if a==b: return 'pass',f,None
    return 'fail',f,'\n'.join(difflib.unified_diff(b,a,'osadecompile','scriptview',lineterm='',n=0))
files=[l.strip() for l in open(sys.argv[1]) if l.strip()]
res=collections.Counter(); passes=[]
with ThreadPoolExecutor(8) as ex:
    for kind,f,info in ex.map(check, files):
        res[kind]+=1
        if kind=='pass': passes.append(f)
        elif kind=='fail':
            name=re.sub(r'[^A-Za-z0-9]','_',os.path.basename(f)[:-5])+'_'+hashlib.md5(f.encode()).hexdigest()[:4]
            open(f'{out}/{name}.diff','w').write(f+'\n'+info)
        elif kind=='crash': print('CRASH',f,info)
if '--passlist' in sys.argv: open(sys.argv[sys.argv.index('--passlist')+1],'w').write('\n'.join(sorted(passes))+'\n')
print(dict(res))
