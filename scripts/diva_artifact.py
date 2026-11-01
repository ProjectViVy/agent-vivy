"""Assemble an explicit Windows core artifact for the independent desktop build."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
RIG = '810d7dfd783c59f309892bbfeb1d73dbf65d51b6'

def digest(path):
    with path.open('rb') as file: return hashlib.file_digest(file, 'sha256').hexdigest()

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--binary',type=Path,required=True)
    p.add_argument('--rig-path',type=Path,required=True)
    p.add_argument('--output',type=Path,required=True)
    args=p.parse_args()
    actual=subprocess.check_output(['git','-C',str(args.rig_path),'rev-parse','HEAD'],text=True).strip()
    if actual != RIG: p.error('core requires the recorded Rig checkpoint fork')
    if subprocess.check_output(['git','status','--porcelain','--untracked-files=no'],cwd=ROOT,text=True).strip():
        p.error('commit backend changes before exporting a matching artifact')
    binary=args.binary.resolve(strict=True)
    output=args.output.resolve(); output.mkdir(parents=True,exist_ok=True)
    files={
        'diva-core-x86_64-pc-windows-msvc.exe':binary,
        'protocol.ts':ROOT/'plugins/vivy-diva-front/contract/protocol.ts',
        'examples.json':ROOT/'plugins/vivy-diva-front/contract/examples.json',
        'version.json':ROOT/'plugins/vivy-diva-front/contract/version.json',
        'recipe.yaml':ROOT/'recipes/diva-agent.yaml',
        'Cargo.lock':ROOT/'.generated/diva-agent-rig-dev/Cargo.lock',
    }
    for name,path in files.items(): shutil.copyfile(path,output/name)
    licenses=output/'licenses'; licenses.mkdir(exist_ok=True)
    for root,prefix in [(ROOT,'vivy'),(args.rig_path,'rig')]:
        for path in root.glob('LICENSE*'):
            if path.is_file(): shutil.copyfile(path,licenses/(prefix+'-'+path.name))
    metadata={
        'schema':1,'protocol_version':json.loads((output/'version.json').read_text()),
        'target':'x86_64-pc-windows-msvc',
        'backend_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),
        'rig_commit':RIG,'files':{name:digest(output/name) for name in files},
    }
    (output/'metadata.json').write_text(json.dumps(metadata,indent=2)+'\n',encoding='utf-8')
    print(output)

if __name__=='__main__':main()
