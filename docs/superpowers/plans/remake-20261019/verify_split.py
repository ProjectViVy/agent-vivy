#!/usr/bin/env python3
"""Verify lossless donor separation, not runtime/product correctness."""
from pathlib import Path
import argparse,json,gzip,subprocess,os,tempfile,csv
ap=argparse.ArgumentParser();ap.add_argument('--repo',required=True);ap.add_argument('--receipt',required=True);a=ap.parse_args()
p=Path(__file__).resolve().parent;G=['git','-C',a.repo]
def git(*args,raw=False,env=None,data=None):return subprocess.check_output(G+list(args),text=not raw,env=env,input=data)
def load(name):
 f=p/name
 return json.loads(gzip.decompress(f.read_bytes())) if name.endswith('.gz') else json.loads(f.read_text())
def tree(ref):
 out={}
 for row in git('ls-tree','-rz','--full-tree',ref,raw=True).split(b'\0'):
  if not row:continue
  m,path=row.split(b'\t',1);mode,typ,sha=m.decode().split();out[path.decode()]={'mode':mode,'type':typ,'sha':sha}
 return out
s=load('source-snapshot.json');records=load('file-ownership.json.gz');parallel=load('parallel-file-ownership.json');hunks=load('hunk-map.json.gz');receipt=json.loads(Path(a.receipt).read_text());errors=[]
def ck(ok,msg):
 if not ok:errors.append(msg)
base=tree(s['original_baseline']);main=tree(s['main'])
changed={x for x in set(base)|set(main) if base.get(x)!=main.get(x)}
ck(changed=={r['path'] for r in records},'changed-path coverage')
ck(len(records)==len({r['path'] for r in records}),'unique final path ownership')
for r in records:
 ck(r['base_entry']==base.get(r['path']),'base entry '+r['path']);ck(r['donor_entry']==main.get(r['path']),'donor entry '+r['path'])
# Reconstruct the exact donor tree from all partition records in a disposable index.
with tempfile.TemporaryDirectory(prefix='remake-verify-') as d:
 env={**os.environ,'GIT_INDEX_FILE':str(Path(d)/'index')};git('read-tree',s['original_baseline'],env=env)
 for r in records:
  e=r['donor_entry']
  if e is None:git('update-index','--force-remove','--',r['path'],env=env)
  else:git('update-index','--add','--cacheinfo',e['mode']+','+e['sha']+','+r['path'],env=env)
 reconstructed=git('write-tree',env=env).strip()
ck(reconstructed==git('rev-parse',s['main']+'^{tree}').strip(),'union reconstructs exact donor tree')
# Each source branch changes only its assigned source files, plus plan/log docs.
allowed_docs=('docs/superpowers/plans/remake-20261019/','docs/superpowers/specs/2026-10-19-remake-design.md','docs/logs/2026-10-20-remake-source-split/')
assigned_count=0
for lane,info in receipt['lanes'].items():
 ref=info.get('sha') or info.get('local_commit') or info['tree'];actual=tree(ref)
 own=[r for r in (parallel if lane=='tool-parallelism' else records) if r['owner']==lane]
 paths={r['path'] for r in own};assigned_count+=len(paths)
 for r in own:ck(actual.get(r['path'])==r['donor_entry'],'lane exact donor '+lane+':'+r['path'])
 for path in set(base)|set(actual):
  if path in paths or path.startswith(allowed_docs):continue
  ck(actual.get(path)==base.get(path),'unassigned source changed '+lane+':'+path)
 ck(git('rev-parse',ref+'^{tree}').strip()==info['tree'],'lane tree receipt '+lane)
# Parallel manifest must describe its actual source base, even though branch starts on A.
for r in parallel:
 bt=tree(r['donor_base']);ck(r['base_entry']==bt.get(r['path']),'parallel before-base '+r['path'])
# Every non-null before/after hunk locator resolves to a blob in its stated tree.
locators=sorted({v for r in hunks for v in [r.get('before_locator'),r.get('after_locator')] if v})
result=git('cat-file','--batch-check',data='\n'.join(locators)+'\n').splitlines()
ck(len(result)==len(locators),'hunk locator result count')
gitlinks=0
for loc,res in zip(locators,result):
 if not res.endswith(' missing') and ' blob ' in res:continue
 revision,path=loc.split(':',1)
 entry=git('ls-tree',revision,'--',path).strip()
 valid_gitlink=entry.startswith('160000 commit ') and entry.endswith('\t'+path)
 if valid_gitlink:gitlinks+=1
 ck(valid_gitlink,'unresolvable hunk '+loc)
archive=receipt['archive']
for ref,sha in s['heads'].items():
 c=subprocess.run(G+['merge-base','--is-ancestor',sha,archive],capture_output=True);ck(c.returncode==0,'history not archived '+ref)
output={'status':'FAIL' if errors else 'PASS','changed_paths':len(records),'dedicated_source_deltas':assigned_count,'lanes':len(receipt['lanes']),'hunks':len(hunks),'hunk_locators':len(locators),'hunk_gitlink_pointers':gitlinks,'frozen_heads':len(s['heads']),'reconstructed_tree':reconstructed,'scope':'source separation only; no product acceptance','errors':errors[:15],'error_count':len(errors)}
print(json.dumps(output,indent=2));raise SystemExit(bool(errors))
