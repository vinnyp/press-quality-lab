"""Derive outcomes, reasons, failure kinds, target sizes and preconditions.

Uses subject declarations, scripted host controls and first confined recordings,
including discovery, placeholders, runbook dependencies and probe controls.
Does not run subjects or derive verdicts, required sets, ids or tiers. Missing
predicate bytes or incomplete discovery text fail closed and are counted by
reason as underivable; no evidence is silently skipped.
"""
import argparse
import base64
from decimal import Decimal
import hashlib
import importlib.util
import json
from pathlib import Path
import sys


_spec = importlib.util.spec_from_file_location("case_ids", Path(__file__).with_name("check-case-ids.py"))
case_ids = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(case_ids)


def unobserved(reason):
    return ('not_observed', reason, None), None


def placeholder(value):
    return isinstance(value, dict) and set(value) == {'placeholder'}


def has_placeholder(value):
    if placeholder(value):
        return True
    if isinstance(value, dict):
        return any(has_placeholder(v) for v in value.values())
    if isinstance(value, list):
        return any(has_placeholder(v) for v in value)
    return False


def command_for(subject, words):
    return max((c for c in subject['commands'] if words[:len(c['words'])] == c['words']),
               key=lambda c: len(c['words']), default={})


def invocation_result(run, invocation, host):
    if not run['markers']['confinement'] or run['end'].get('by') == 'before_marker':
        return ('not_observed', 'network_unverifiable', None)
    if run['markers']['could_not_start']:
        return ('not_observed', 'could_not_start', None)
    if run['end']['kind'] != 'exit':
        return signal_result(run, invocation, host)
    return None


def discovery(subject, recording, host):
    status = {}
    keys = case_ids.discovery_keys(recording, status=status)
    runs = [find(recording, k) for k in keys]
    found = [k['words'][:-1] for k in keys[1:]]
    declared = [c['words'] for c in subject['commands'] if c['words']]
    bounded = status.get("bounded", False)
    for k, run in zip(keys, runs):
        result = invocation_result(run, k, host)
        if result is not None and result[2] != 'crashed':
            return result, False, False
    unreadable = status.get("unreadable", False) or any(run['end'] != {'kind': 'exit', 'code': 0} or
                     not (run['stdout']['bytes'] or run['stderr']['bytes']) for run in runs)
    root = runs[0]['stdout'] if runs[0]['stdout']['bytes'] else runs[0]['stderr']
    root_text = kept(root).decode('utf-8', errors='replace')
    headings = any(is_heading(line) for line in root_text.splitlines())
    unreadable |= bool(declared) and not headings
    if bounded or unreadable:
        return ('not_observed', 'precondition_unmet', None), False, False
    missing = any(c not in found for c in declared)
    if missing:
        return ('not_observed', 'precondition_unmet', None), True, False
    extra = any(c not in declared for c in found)
    if placeholder(subject['declared_count']):
        return ('not_observed', 'placeholder_unfilled', None), False, extra
    ok = len(found) == subject['declared_count']
    return ('held' if ok else 'not_held', None, None if ok else 'predicate'), False, extra


def is_heading(line):
    return not line.startswith(' ') and (line in ('COMMANDS:', 'Commands:', 'SUBCOMMANDS:') or line.endswith(' Commands:'))


def computation(pre, spec, subject, recording):
    for capture in spec['captures']:
        matches = [e for e in recording['invocations'] if e['key']['runbook'] == pre['runbook'] and e['key']['step'] == capture and e['key']['computation'] is None]
        if len(matches) != 1:
            raise ValueError('cannot derive computation capture')
        if matches[0]['confined'][0]['stdout']['cut']:
            return 'harness_fault', None
    invocation = dict(words=None, offline_args=[], env=[], runbook=pre['runbook'], step=pre['step'], computation=spec['argv'], home='scratch')
    matches = [e for e in recording['invocations'] if e['key'] == invocation]
    if not matches:
        return 'computation_failed', None
    run = find(recording, invocation)
    if (not run['markers']['confinement'] or run['markers']['could_not_start'] or
            run['end'] != {'kind': 'exit', 'code': 0} or run['printed_value'] is None or
            run['stdout']['bytes'] > 1048576):
        return 'computation_failed', None
    return None, run['printed_value']['value']


def derive_all(pre, subject, recording, host, previous, discovery_state, seed_path):
    probe = pre['probe']
    if host.get('confinement', {}).get('applied') is False or host.get('self_test') == 'failed':
        return unobserved('network_unverifiable')
    dependent = placeholder(subject['binary'])
    if probe != 'discovery':
        dependent |= placeholder(subject['seed']) or placeholder(subject['store'])
    if probe == 'discovery':
        dependent |= placeholder(subject['declared_count'])
    if probe == 'planted_secret':
        dependent |= placeholder(subject['secret'])
        words = subject['secret'].get('commands', []) if isinstance(subject['secret'], dict) else []
    else:
        words = [pre['words']] if pre['words'] is not None else []
    for word in words:
        c = command_for(subject, word)
        if probe != 'discovery':
            dependent |= placeholder(c.get('class')) or placeholder(c.get('writes'))
    if dependent:
        return unobserved('placeholder_unfilled')
    if probe is not None and discovery_state[1]:
        return unobserved('precondition_unmet')
    if any(previous[earlier][0][0] != 'held' for earlier in pre['earlier']):
        return unobserved('earlier_step_failed')
    if host.get('private_copy') == 'changed':
        return unobserved('harness_fault')
    if probe == 'discovery':
        return discovery_state[0], None
    if probe == 'planted_secret':
        if subject['secret'] is None:
            return unobserved('input_missing')
        keys = [key(pre, c, 'planted') for c in pre['parameters']['commands']]
        runs = [find(recording, k) for k in keys]
        if any(r['leak_scan']['leaks'] for r in runs):
            return ('not_held', None, 'predicate'), None
        reads = []
        for k, run in zip(keys, runs):
            planted = run['planted']
            success = run['end']['kind'] == 'exit' and run['end']['code'] in (command_for(subject, k['words']).get('exit_codes') or [0])
            candidate = success and planted is not None and planted['atime_moved'] and planted['same_file']
            control = host.get('read_control')
            killed = control == 'killed' if control is not None else planted is not None and planted['control_end'] == {'kind': 'signal', 'signal': 'SIGKILL', 'by': 'confinement'}
            if candidate and not killed:
                return unobserved('harness_fault')
            reads.append(candidate and killed and host.get('read_calibration', 'moved') == 'moved')
        for k, run in zip(keys, runs):
            result = invocation_result(run, k, host)
            if result is not None and result[2] != 'crashed':
                return result, None
        if any(r['leak_scan']['bound_reached'] for r in runs):
            return unobserved('harness_fault')
        return (('held', None, None), None) if any(reads) else unobserved('input_missing')
    if pre['words'] is None:
        return unobserved('empty_target')
    if probe is None:
        expected = pre['expected']
        for source in ([expected] if 'computation' in expected else []) + ([expected['precondition']] if expected['precondition'] else []):
            reason, value = computation(pre, source['computation'], subject, recording)
            if reason:
                return unobserved(reason)
            if source is expected and pre['predicate'] in ('exit_code', 'stdout_bytes_at_most', 'row_count'):
                if isinstance(value, bool) or not isinstance(value, (int, Decimal)) or value != int(value):
                    return unobserved('computation_failed')
    invocation = key(pre)
    run = find(recording, invocation)
    result = invocation_result(run, invocation, host)
    if result:
        return result, None
    derived = derive(pre, subject, recording, host)
    if derived[0][2] == 'wrong_exit':
        return derived
    if probe == 'store_unchanged':
        if subject['store'] is None or not seed_has_data(seed_path, subject['bookkeeping']):
            return unobserved('empty_target')
        detected = host.get('store_control') == 'detected' if 'store_control' in host else run['home_changes']['control_detected']
        if not detected:
            return unobserved('harness_fault')
    return derived



def seed_has_data(seed_path, bookkeeping):
    for path in seed_path.rglob('*'):
        name = path.relative_to(seed_path).as_posix()
        if any(name == b or name.startswith(b + '/') for b in bookkeeping + ['.pql-tmp']):
            continue
        if path.is_file() and path.stat().st_size:
            return True
    return False


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
        if change['kind'] in ('link', 'special'):
            return True
        if path == '.pql-tmp' or path.startswith('.pql-tmp/'):
            continue
        if any(path == b or path.startswith(b + '/') or
               (b.startswith(path + '/') and change['kind'] == 'directory' and change['change'] == 'created')
               for b in subject.get('bookkeeping', [])):
            continue
        if change['kind'] == 'file' and store and (path == store + '-shm' or
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
            computed_run = find(recording, computation)
            printed = computed_run['printed_value']
            if printed is None:
                return ('not_observed', 'computation_failed', None), size
            expected = printed['value']
            if check['predicate'] in ('exit_code', 'stdout_bytes_at_most', 'row_count') and (isinstance(expected, bool) or not isinstance(expected, (int, Decimal)) or expected != int(expected)):
                return unobserved('computation_failed')
        accepted = [expected] if check['predicate'] == 'exit_code' else check['exit_codes'] if check['exit_codes'] is not None else accepted
    if run['end']['code'] not in accepted:
        return ('not_held', None, 'wrong_exit'), size
    if check is not None:
        predicate = check['predicate']
        stream = 'stderr' if predicate == 'stderr_contains' else 'stdout'
        if predicate not in ('exit_code', 'stdout_bytes_at_most') and run[stream]['cut']:
            return unobserved('harness_fault')
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
    bad = checked = compared = 0
    underived = {}
    for directory in sorted(args.cases.iterdir()):
        case = load(directory / 'case.json')
        if case['kind'] != 'judge-checked':
            continue
        checked += 1
        subject = load(directory / case['input']['subject_file'])
        record = load(directory / 'input.json')
        preimages = load(directory / 'preimages.json')
        try:
            case_ids.check_pairing(record, preimages)
        except ValueError as exc:
            print(f"{case['id']}: {exc}")
            bad += 1
            continue
        recording = load(args.kit / 'behaviour' / (case['input']['form'] + '.json'))
        host = case['input']['host'] or {}
        blocked = (host.get('confinement', {}).get('applied') is False or
                   host.get('self_test') == 'failed' or host.get('private_copy') == 'changed' or
                   placeholder(subject['binary']))
        try:
            state = ((None, False, False) if blocked else discovery(subject, recording, host))
        except ValueError as exc:
            state = None
            discovery_error = str(exc)
        previous = {}

        def evaluate(digest):
            if digest in previous:
                return previous[digest]
            pre = preimages[digest]
            for earlier in pre['earlier']:
                evaluate(earlier)
            if state is None:
                raise ValueError(discovery_error)
            result = derive_all(pre, subject, recording, host, previous, state,
                                args.kit / 'subjects' / case['input']['form'] / 'seed')
            previous[digest] = result
            return result

        for evidence in record['evidence']:
            try:
                expected, size = evaluate(evidence['check_id'])
                pre = preimages[evidence['check_id']]
                if (expected[0] == 'held' and (has_placeholder(subject) or state[2]) and
                        pre['dimension'] not in ('bounding', 'idempotency', 'destructive-guard', 'side-effect-integrity', 'flag-conformance')):
                    expected, size = unobserved('placeholder_unfilled')
                precondition = None
                if expected[0] in ('held', 'not_held') and pre['expected'] and pre['expected']['precondition']:
                    _, value = computation(pre, pre['expected']['precondition']['computation'], subject, recording)
                    precondition = value is True
                compared += 1
                stated = tuple(evidence[k] for k in ('outcome', 'reason', 'failure_kind'))
                if stated != expected:
                    raise AssertionError(f'stated {stated}; recording says {expected}')
                if evidence['target_size'] != size:
                    raise AssertionError(f"target_size {evidence['target_size']}; recording says {size}")
                if evidence['precondition_observed'] != precondition:
                    raise AssertionError(f"precondition_observed differs: expected {precondition}")
            except AssertionError as exc:
                print(f"{case['id']} check {evidence['check_id']}: {exc}")
                bad += 1
            except (ValueError, KeyError, TypeError, StopIteration) as exc:
                reason = str(exc)
                underived[reason] = underived.get(reason, 0) + 1
                print(f"{case['id']} check {evidence['check_id']}: cannot derive: {reason}")
    if not checked:
        print('no judge-checked cases checked')
        bad += 1
    reasons = ', '.join(f'{reason}: {count}' for reason, count in sorted(underived.items())) or 'none'
    print(f'{checked} judge-checked cases: {compared} pieces compared; {sum(underived.values())} could not derive ({reasons}); {bad} outcome disagreements')
    return int(bad != 0 or bool(underived))


if __name__ == '__main__':
    sys.exit(main())
