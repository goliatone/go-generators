#!/usr/bin/env python3
"""Rerun the frozen Go producer and strict TS/runtime proof in a disposable tree."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / '.ctx/specs/rpc-contract-typescript/support/evidence'

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tsc', required=True, type=Path, help='installed TypeScript bin/tsc (no downloads)')
    parser.add_argument('--record', type=Path, help='write successful evidence JSON')
    args = parser.parse_args()
    tsc = args.tsc.resolve()
    source = json.loads((EVIDENCE / 'bootstrap-source.json').read_text())
    archive = EVIDENCE / 'bootstrap-source.tar.gz'
    if digest(archive) != source['archiveSHA256']:
        raise RuntimeError('producer archive hash mismatch')
    env = dict(os.environ, GOWORK='off', GOPROXY='off', GOSUMDB='off', GOFLAGS='-mod=readonly -buildvcs=false')
    commands = []
    def run(argv, cwd):
        commands.append({'argv': [str(x) for x in argv], 'cwd': str(cwd)})
        result = subprocess.run([str(x) for x in argv], cwd=cwd, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        if result.returncode:
            raise RuntimeError(result.stdout)
        print(result.stdout, end='')
        return result.stdout.strip()
    with tempfile.TemporaryDirectory(prefix='rpc-contract-proof-') as scratch:
        work = Path(scratch)
        producer, out, second = work/'producer', work/'out', work/'second'
        producer.mkdir(); out.mkdir(); second.mkdir()
        with tarfile.open(archive) as tar:
            for member in tar.getmembers():
                if not member.isfile() or Path(member.name).is_absolute() or '..' in Path(member.name).parts:
                    raise RuntimeError('unsafe source archive member')
                target = producer/member.name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(tar.extractfile(member).read())
        for name, expected in source['files'].items():
            if digest(producer/name) != expected:
                raise RuntimeError('producer source hash mismatch: '+name)
        for dest in [out, second]:
            run(['go', 'run', './examples/rpc/tsgen/export', '-out', dest], producer)
        for name in ['manifest.json', 'wire.json', 'checks.json']:
            if (out/name).read_bytes() != (second/name).read_bytes():
                raise RuntimeError('nonrepeatable producer: '+name)
            if (out/name).read_bytes() != (ROOT/'rpccontract/testdata'/name).read_bytes():
                raise RuntimeError('stale producer golden: '+name)
        checks = json.loads((out/'checks.json').read_text())
        expected_checks = {'reject_alias_bounds_bypass', 'reject_duplicate_merge_bounds_bypass', 'reject_ambiguous_canonical_alias', 'reject_ambiguous_canonical_duplicate', 'preserve_safe_alias_and_unknown_fields', 'preserve_safe_duplicate_merge', 'reject_unsafe_egress', 'reject_unquoted_json_number', 'reject_normalized_command_id_collision', 'preserve_dash_and_quoted_number'}
        if set(checks) != expected_checks or any(value is not True for value in checks.values()):
            raise RuntimeError('fresh producer regression checks are incomplete or failed')
        print('PASS: all 10 fresh producer review regression checks')
        run(['go', 'run', './cmd/rpc-contract-ts', '-manifest', out/'manifest.json', '-out', out/'contracts.ts'], ROOT)
        run(['go', 'run', './cmd/rpc-contract-ts', '-manifest', out/'manifest.json', '-out', out/'contracts.ts', '--check'], ROOT)
        if (out/'contracts.ts').read_bytes() != (ROOT/'rpccontract/testdata/contracts.ts').read_bytes():
            raise RuntimeError('stale emitter golden')
        wire = json.loads((out/'wire.json').read_text())
        bindings = {'wireEdgesInput': 'CommandInputs["example.wire_edges"]', 'wireEdgesRequest': 'MethodRequests["example.wire_edges"]', 'wireEdgesResponse': 'SuccessEnvelope<MethodOutputs["example.wire_edges"]>', 'input': 'CommandInputs["example.echo"]', 'inputMin': 'CommandInputs["example.echo"]', 'request': 'MethodRequests["example.echo"]', 'dispatchRequest': 'DispatchRequest<"example.echo">', 'dispatchInline': 'SuccessEnvelope<DispatchResponse<"example.echo">>', 'dispatchQueued': 'SuccessEnvelope<DispatchResponse<"example.echo">>', 'dispatchFailure': 'SuccessEnvelope<DispatchResponse<"example.echo">>', 'object': 'SuccessEnvelope<MethodOutputs["example.echo"]>', 'presentNull': 'SuccessEnvelope<MethodOutputs["example.nullable"]>'}
        for name, method in {'presentFalse':'boolean', 'presentZero':'number', 'presentEmpty':'string', 'presentSlice':'slice', 'presentMap':'map', 'omittedFalse':'boolean', 'omittedZero':'number', 'omittedEmpty':'string', 'omittedNil':'slice', 'omittedSlice':'slice', 'omittedMap':'map'}.items():
            bindings[name] = 'SuccessEnvelope<MethodOutputs["example.'+method+'"]>'
        # Compile fresh literal Go bytes, with concrete annotations and no casts.
        text = 'import type { CommandInputs, MethodRequests, MethodOutputs, DispatchRequest, DispatchResponse, SuccessEnvelope } from "./contracts";\n'
        for name, annotation in bindings.items():
            text += 'export const '+name+': '+annotation+' = '+json.dumps(wire[name], ensure_ascii=True)+';\n'
        (out/'wire.ts').write_text(text)
        shutil.copyfile(ROOT/'rpccontract/interop/strict.ts', out/'strict.ts')
        shutil.copyfile(ROOT/'rpccontract/interop/runtime.cjs', out/'runtime.cjs')
        version = run(['node', tsc, '--version'], out)
        run(['node', tsc, '--strict', '--exactOptionalPropertyTypes', '--noUncheckedIndexedAccess', '--target', 'ES2020', '--module', 'commonjs', '--outDir', out/'js', out/'contracts.ts', out/'wire.ts', out/'strict.ts'], out)
        shutil.copyfile(out/'runtime.cjs', out/'js/runtime.cjs')
        run(['node', out/'js/runtime.cjs', out/'wire.json'], out)
        record = {'status':'PASS', 'sourceArchiveSHA256': source['archiveSHA256'], 'artifacts': {name:digest(out/name) for name in ['manifest.json','wire.json','checks.json','contracts.ts','wire.ts']}, 'producerChecks':checks, 'typescript':version, 'go':run(['go','version'],producer), 'node':run(['node','--version'],out), 'environment':{k:env[k] for k in ['GOWORK','GOPROXY','GOSUMDB','GOFLAGS']}, 'commands':commands, 'generatorSourceSHA256':{str(p.relative_to(ROOT)):digest(p) for p in sorted([ROOT/'go.mod',ROOT/'go.sum',*ROOT.glob('rpccontract/*.go'),*ROOT.glob('cmd/rpc-contract-ts/*.go'),*ROOT.glob('rpccontract/interop/*')]) if p.is_file()}, 'scope':'fresh uncommitted Go source -> emitter -> strict TS and actual wire callbacks; no published or live HTTP claim'}
        if args.record:
            args.record.write_text(json.dumps(record, indent=2)+'\n')
        print('PASS: fresh Go source export -> TypeScript interoperability')

if __name__ == '__main__':
    main()
