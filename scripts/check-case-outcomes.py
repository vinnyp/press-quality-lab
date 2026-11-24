"""Check held/not_held case evidence against first confined recordings.

Does not re-derive not_observed outcomes from host controls, placeholders,
QD4 or QA7. Signal results are checked against QO3–QO5 and host attribution.
Discovery counts are covered by the case-id cross-check, not this checker.
Reads only case files and behaviour/<form>.json; never runs a subject.
"""
import argparse
import base64
from decimal import Decimal
import hashlib
import json
from pathlib import Path
import sys


def load(path):
    return json.loads(path.read_text(), parse_float=Decimal)


def key(pre, command=None, home=None):
    command = pre if command is None else command
    return dict(words=command['words'], offline_args=command['offline_args'],
                env=sorted(command['env']), runbook=pre['runbook'], step=pre['step'],
                computation=None, home=home or ('runbook' if pre['runbook'] is not None else 'seed'))


def find(recording, invocation):
    matches = [entry for entry in recording['invocations'] if entry['key'] == invocation]
    if len(matches) != 1:
        raise ValueError('expected exactly one recorded invocation')
    return matches[0]['confined'][0]


def kept(stream):
    if stream.get('kept') is None:
        raise ValueError('predicate stream bytes not kept')
    return base64.b64decode(stream['kept'], validate=True)


def equal(left, right):
    if isinstance(left, bool) or isinstance(right, bool):
        return type(left) is type(right) and left == right
    if isinstance(left, (int, Decimal)) and isinstance(right, (int, Decimal)):
        return left == right
    if type(left) is not type(right):
        return False
    if isinstance(left, list):
        return len(left) == len(right) and all(equal(a, b) for a, b in zip(left, right))
    if isinstance(left, dict):
        return left.keys() == right.keys() and all(equal(left[k], right[k]) for k in left)
    return left == right


ABSENT = object()


def pointer(value, path):
    for token in path.split('/')[1:] if path else []:
        token = token.replace('~1', '/').replace('~0', '~')
        if isinstance(value, dict):
            value = value.get(token, ABSENT)
        elif isinstance(value, list) and token.isascii() and token.isdigit() and (token == '0' or not token.startswith('0')) and int(token) < len(value):
            value = value[int(token)]
        else:
            return ABSENT
    return value


def parsed(stream):
    raw = kept(stream)

    def pairs(items):
        result = {}
        for name, value in items:
            if name in result:
                raise ValueError('duplicate JSON key')
            result[name] = value
        return result

    def invalid(value):
        raise ValueError('non-JSON number')

    try:
        return json.loads(raw.decode('utf-8'), parse_float=Decimal,
                          object_pairs_hook=pairs, parse_constant=invalid)
    except (ValueError, UnicodeError):
        return ABSENT


def store_changed(run, subject):
    store = subject.get('store')
    store = store if isinstance(store, str) else None
    empty = hashlib.sha256(b'').hexdigest()
    for change in run['home_changes']['changes']:
        path = change['path']
        if path == '.pql-tmp' or path.startswith('.pql-tmp/'):
            continue
        if any(path == b or path.startswith(b + '/') or
               (b.startswith(path + '/') and change['kind'] == 'directory' and change['change'] == 'created')
               for b in subject.get('bookkeeping', [])):
            continue
        if store and (path == store + '-shm' or
                      path in (store + '-wal', store + '-journal') and
                      (change['change'] == 'removed' or change.get('sha256') == empty)):
            continue
        return True
    return False


def signal_result(run, invocation, host):
    end = run['end']
    if end.get('by') == 'budget':
        return ('not_held', None, 'over_budget') if host.get('known_fast', 'within') == 'within' else ('not_observed', 'harness_fault', None)
    if end.get('signal') == 'SIGKILL' and end.get('by') == 'confinement':
        attribution = next((a['by'] for a in host.get('kill_attribution', []) if a['invocation'] == invocation), None)
        if attribution == 'engine':
            return 'not_observed', 'harness_fault', None
        if attribution not in ('memory', 'code_signing'):
            return 'not_observed', 'network_unverifiable', None
    return 'not_held', None, 'crashed'


def derive(pre, subject, recording, host):
    probe = pre['probe']
    size = None
    if probe == 'planted_secret':
        runs = [find(recording, key(pre, command, 'planted')) for command in pre['parameters'].get('commands', [])]
        ok = not any(run['leak_scan']['leaks'] for run in runs)
        return ('held' if ok else 'not_held', None, None if ok else 'predicate'), size
    invocation = key(pre)
    run = find(recording, invocation)
    if run['end']['kind'] != 'exit':
        return signal_result(run, invocation, host), size
    commands = subject.get('commands')
    candidates = [c for c in commands if pre['words'][:len(c['words'])] == c['words']] if isinstance(commands, list) else []
    command = max(candidates, key=lambda c: len(c['words']), default={})
    accepted = command.get('exit_codes') or [0]
    check = None
    if probe in ('bare', 'exit_codes'):
        accepted = pre['parameters']['exit_codes'] or [0]
    elif probe is None:
        runbook = next(r for r in subject['runbooks'] if r['name'] == pre['runbook'])
        step = next(s for s in runbook['steps'] if s['name'] == pre['step'])
        check = step['checks'][pre['index']]
        expected = check['expected']
        if 'literal' in expected:
            expected = expected['literal']
        else:
            computation = dict(words=None, offline_args=[], env=[], runbook=pre['runbook'], step=pre['step'], computation=expected['computation']['argv'], home='scratch')
            expected = find(recording, computation)['printed_value']
        accepted = [expected] if check['predicate'] == 'exit_code' else check['exit_codes'] if check['exit_codes'] is not None else accepted
    if run['end']['code'] not in accepted:
        return ('not_held', None, 'wrong_exit'), size
    if probe in ('bare', 'exit_codes'):
        ok = True
    elif probe == 'byte_bound':
        size = run['stdout']['bytes'] + run['stderr']['bytes']
        ok = size <= pre['parameters']['bound']
    elif probe == 'store_unchanged':
        ok = not store_changed(run, subject)
    elif check is not None:
        predicate = check['predicate']
        if predicate == 'exit_code':
            ok = True
        elif predicate == 'stdout_bytes_at_most':
            size = run['stdout']['bytes']
            ok = size <= expected
        elif predicate == 'stdout_empty':
            ok = run['stdout']['bytes'] == 0
        elif predicate in ('stdout_equals', 'stdout_contains', 'stdout_not_contains', 'stderr_contains'):
            out = kept(run['stderr' if predicate == 'stderr_contains' else 'stdout'])
            wanted = expected.encode('utf-8')
            ok = out == wanted if predicate == 'stdout_equals' else wanted not in out if predicate == 'stdout_not_contains' else wanted in out
            if predicate == 'stdout_not_contains':
                size = run['stdout']['bytes']
        elif predicate in ('stdout_json_equals', 'json_path_equals', 'row_count'):
            value = parsed(run['stdout'])
            if predicate != 'stdout_json_equals':
                value = pointer(value, check['path'])
            if predicate == 'row_count':
                ok = isinstance(value, list) and len(value) == expected
                if 'computation' in check['expected'] and isinstance(value, list):
                    size = len(value)
            else:
                ok = equal(value, expected)
        else:
            raise ValueError('unsupported predicate: ' + predicate)
    else:
        raise ValueError('unsupported probe: ' + str(probe))
    return ('held' if ok else 'not_held', None, None if ok else 'predicate'), size


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('kit', type=Path)
    parser.add_argument('cases', type=Path)
    args = parser.parse_args()
    bad = checked = 0
    for directory in sorted(args.cases.iterdir()):
        case = load(directory / 'case.json')
        if case['kind'] != 'judge-checked':
            continue
        checked += 1
        subject = load(directory / case['input']['subject_file'])
        record = load(directory / 'input.json')
        preimages = load(directory / 'preimages.json')
        recording = load(args.kit / 'behaviour' / (case['input']['form'] + '.json'))
        for evidence in record['evidence']:
            pre = preimages[evidence['check_id']]
            if evidence['outcome'] == 'not_observed' or pre['probe'] == 'discovery':
                continue
            try:
                expected, size = derive(pre, subject, recording, case['input']['host'] or {})
                stated = tuple(evidence[k] for k in ('outcome', 'reason', 'failure_kind'))
                if stated != expected:
                    raise ValueError(f'stated {stated}; recording says {expected}')
                if evidence['target_size'] != size:
                    raise ValueError(f"target_size {evidence['target_size']}; recording says {size}")
            except (ValueError, KeyError, TypeError, StopIteration) as exc:
                print(f"{case['id']} check {evidence['check_id']}: {exc}")
                bad += 1
    print(f'{checked} judge-checked cases: {bad} outcome disagreements')
    return int(bad != 0)


if __name__ == '__main__':
    sys.exit(main())
