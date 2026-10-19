#!/usr/bin/env python3
"""Check frozen source coverage against a supplied local repository; never fetch/write."""
import argparse,csv,json,subprocess,datetime,sys
from pathlib import Path
ap=argparse.ArgumentParser();ap.add_argument('--git-dir',required=True);a=ap.parse_args()
p=Path(__file__).resolve().parents[4]/'docs/superpowers/plans/remake-20261019'
s=json.loads((p/'source-snapshot.json').read_text());lo=datetime.datetime.fromisoformat(s['window_start']).timestamp();hi=datetime.datetime.fromisoformat(s['window_end']).timestamp()
G=['git','--git-dir='+a.git_dir]
def git(*args):return subprocess.check_output(G+list(args),text=True,stderr=subprocess.STDOUT)
missing=[]
for name,sha in s['heads'].items():
 try:git('cat-file','-e',sha+'^{commit}')
 except subprocess.CalledProcessError:missing.append({'head':name,'sha':sha})
if missing:print(json.dumps({'status':'MISSING_SOURCE_OBJECTS','missing':missing},indent=2));sys.exit(1)
actual=set()
for line in git('log',*s['heads'].values(),'--format=%H%x09%ct').splitlines():
 sha,ts=line.split('\t')
 if lo<=int(ts)<=hi:actual.add(sha)
expected={r['sha'] for r in csv.DictReader((p/'commit-ledger.csv').open(encoding='utf-8-sig'))}
if actual!=expected:print(json.dumps({'status':'COVERAGE_MISMATCH','missing':sorted(expected-actual),'extra':sorted(actual-expected)},indent=2));sys.exit(1)
print(json.dumps({'status':'PASS','frozen_heads':len(s['heads']),'window_commits':len(expected),'scope':'Git source coverage only'},indent=2))
