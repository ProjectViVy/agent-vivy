#!/usr/bin/env python3
"""Read-only validation of the delivered planning package; no product checks."""
from pathlib import Path
import csv,json,re,sys
root=Path(__file__).resolve().parents[4]
p=root/'docs/superpowers/plans/remake-20261019'
s=json.loads((p/'source-snapshot.json').read_text())
rows=list(csv.DictReader((p/'commit-ledger.csv').open(encoding='utf-8-sig')))
paths=json.loads((p/'commit-paths.json').read_text())
nodes=json.loads((p/'dag.json').read_text())
errors=[]
def check(ok,msg):
 if not ok:errors.append(msg)
check(len(rows)==s['counts']['total'],'ledger count')
check(len({r['sha'] for r in rows})==len(rows),'unique ledger SHA')
check({r['sha'] for r in rows}==set(paths),'one path record per SHA')
check(sum(r['in_main']=='True' for r in rows)==s['counts']['main'],'main count')
check(sum(r['kind']=='merge' for r in rows)==s['counts']['merge'],'merge count')
from collections import Counter
check(dict(Counter(r['primary_owner'] for r in rows))==s['candidate_owner_counts'],'owner counts')
for r in rows:
 check(len(paths[r['sha']])==int(r['path_count']),'path count '+r['sha'])
 check(bool(r['primary_owner']) and bool(r['review_state']) and bool(r['disposition']),'routing fields '+r['sha'])
 check(re.fullmatch('[0-9a-f]{40}',r['sha']) is not None,'full SHA')
check(not json.loads((p/'source-path-check.json').read_text())['missing'],'verified donor paths')
for n,d in nodes.items():
 check(len(d['depends'])==len(set(d['depends'])),'duplicate dependency '+n)
 for pred in d['depends']:check(pred in nodes and pred!=n,'invalid dependency '+n)
done=set();remaining=set(nodes);waves=[]
while remaining:
 ready=sorted(n for n in remaining if set(nodes[n]['depends'])<=done)
 if not ready:errors.append('DAG cycle or missing endpoint');break
 waves.append(ready);done.update(ready);remaining.difference_update(ready)
# Detect redundant edges in this small dependency graph.
def ancestors(n):
 out=set();todo=list(nodes[n]['depends'])
 while todo:
  x=todo.pop()
  if x in out or x not in nodes:continue
  out.add(x);todo+=nodes[x]['depends']
 return out
for n,d in nodes.items():
 for x in d['depends']:
  for y in d['depends']:
   if x!=y:check(x not in ancestors(y),'redundant edge '+n+' -> '+x)
md=list(p.rglob('*.md'))+[root/'docs/superpowers/specs/2026-10-19-remake-design.md'];linkcount=0
for f in md:
 text=f.read_text(encoding='utf-8')
 for m in re.finditer(r'(?<!!)\[[^\]]+\]\(([^)]+)\)',text):
  target=m.group(1).split('#',1)[0]
  if not target or re.match(r'^[A-Za-z][\w+.-]*:',target):continue
  linkcount+=1;check((f.parent/target).exists(),'missing link '+str(f.relative_to(root))+' -> '+target)
 for i,line in enumerate(text.splitlines(),1):check(line==line.rstrip(),'trailing whitespace '+str(f.relative_to(root))+':'+str(i))
check(len(list((p/'lanes').glob('*.md')))==15,'15 lane cards')
if errors:
 print('\n'.join('FAIL '+e for e in errors));sys.exit(1)
print(json.dumps({'status':'PASS','commits':len(rows),'main':s['counts']['main'],'branch_only':s['counts']['branch_only'],'merge':s['counts']['merge'],'nodes':len(nodes),'waves':waves,'lane_cards':15,'local_links':linkcount,'scope':'planning package only'},ensure_ascii=False,indent=2))
