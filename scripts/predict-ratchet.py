"""Predict a scripted corpus ratchet; never run a subject or infer a verdict.

The derivation audit below is deliberately finite: it audits the sealed corpus,
not arbitrary subject files. Digests fail closed if an audited file changes.
Rule order: collection errata E-9 (QS2 is the fallback, not the first rule).
Reason mapping citations are lines of collection-rules.md in the sealed kit.
"""
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
KIT = ROOT / 'docs/specs/conformance-collection-1.0'
REASON_LEVELS = {
    'network_unverifiable': {'levels': [1, 7], 'lines': '122,135-142',
                             'when': 'QO1 host/marker failure: 1; QO3 kill: 7'},
    'placeholder_unfilled': {'levels': [2, 8], 'lines': '123,131',
                             'when': 'dependency: 2; final QS5 override: after 8'},
    'precondition_unmet': {'levels': [3, 8], 'lines': '124,129,184,188-192',
                           'when': 'QD4 missing declared command: 3; QD2/QD5 discovery: 8'},
    'earlier_step_failed': {'levels': [4], 'lines': '125', 'when': 'QA7'},
    'harness_fault': {'levels': [5, 6, 7, 8], 'lines': '126-129,150,152,164,168',
                      'when': 'private copy: 5; cut capture: 6; invocation: 7; own rule: 8'},
    'computation_failed': {'levels': [6], 'lines': '127,159-161', 'when': 'QA6/QO6'},
    'could_not_start': {'levels': [7], 'lines': '128,141', 'when': 'QO2'},
    'over_budget': {'levels': [7], 'lines': '128,146-149', 'when': 'QO4'},
    'crashed': {'levels': [7], 'lines': '128,151-152', 'when': 'QO5'},
    'wrong_exit': {'levels': [7], 'lines': '128,153-157', 'when': 'QO5'},
    'held/predicate/input_missing/empty_target': {
        'levels': [8], 'lines': '129,158,165-170', 'when': 'probe or predicate'},
}
LEVEL_RULE = {1: 'QO1', 2: 'QS5', 3: 'QD4', 4: 'QA7', 5: 'QH8',
              6: 'QA6/QA4', 7: 'QO2-QO5', 8: 'probe/predicate'}
RULE_ORDER = ['QS1'] + ['QS' + str(i) for i in range(3, 13)] + [
    'QP7', 'QP10', 'QA4', 'QA9', 'QS2', 'QE6']

# Each audit lists ALL violated named rules, plus QS2 if the schema also
# rejects. A semantic part is not covered by a schema-only implementation.
# Source coordinates and rule text accompany every part in the output.
DERIVATION_AUDIT = {}  # populated below, before main


def load(path):
    return json.loads(path.read_text(encoding='utf-8'))


def names(entries):
    """Allow terse strings or entries carrying an implementation rationale."""
    return {e if isinstance(e, str) else e['rule'] for e in entries}


def supports(capabilities, rule):
    return rule in capabilities or rule.split('.')[0] in capabilities


def placeholder(value):
    return isinstance(value, dict) and set(value) == {'placeholder'}


def contains_placeholder(value):
    if placeholder(value):
        return True
    if isinstance(value, dict):
        return any(contains_placeholder(v) for v in value.values())
    if isinstance(value, list):
        return any(contains_placeholder(v) for v in value)
    return False


def depends(subject, preimage):
    """QS5's dependency list; a fact placeholder alone is a final override."""
    probe = preimage['probe']
    if placeholder(subject['binary']):
        return True
    if probe == 'discovery':
        return placeholder(subject['declared_count'])
    if any(placeholder(subject[k]) for k in ('seed', 'store')):
        return True
    if probe == 'planted_secret':
        if placeholder(subject['secret']):
            return True
        secret = subject['secret']
        words = secret.get('commands', []) if isinstance(secret, dict) else []
    else:
        words = [preimage['words']] if preimage['words'] is not None else []
    for invocation in words:
        candidates = [c for c in subject['commands']
                      if invocation[:len(c['words'])] == c['words']]
        if candidates:
            command = max(candidates, key=lambda c: len(c['words']))
            if placeholder(command['class']) or placeholder(command['writes']):
                return True
    return False


def has_computation(preimage):
    expected = preimage['expected'] or {}
    return 'computation' in expected or expected.get('precondition') is not None


def has_invocation(preimage):
    if preimage['words'] is not None or preimage['probe'] == 'discovery':
        return True
    return bool(preimage['probe'] == 'planted_secret' and
                preimage['parameters'].get('commands'))


def trigger(level, subject, preimage):
    return {2: lambda: depends(subject, preimage),
            3: lambda: preimage['probe'] is not None,
            4: lambda: bool(preimage['earlier']),
            5: lambda: True,
            6: lambda: has_computation(preimage),
            7: lambda: has_invocation(preimage),
            8: lambda: True}[level]()


def runs_for(preimage, recording):
    """Only first confined runs, never the unconfined positive controls."""
    result = []
    for invocation in recording['invocations']:
        key = invocation['key']
        if preimage['probe'] == 'discovery':
            match = key['home'] == 'empty'
        elif preimage['probe'] == 'planted_secret':
            match = key['home'] == 'planted'
        elif preimage['probe'] is None:
            match = (key['runbook'], key['step']) == (preimage['runbook'], preimage['step'])
        else:
            match = key['home'] == 'seed' and key['words'] == preimage['words']
        if match:
            result.append((key, invocation['confined'][0]))
    return result


def deciding_level(evidence, preimage, subject, host, recording, all_evidence, preimages):
    reason = evidence['reason'] or evidence['failure_kind'] or evidence['outcome']
    runs = runs_for(preimage, recording)
    if reason == 'network_unverifiable':
        if (not host['confinement']['applied'] or host['self_test'] == 'failed' or
                any(not r['markers']['confinement'] for _, r in runs)):
            return 1
        return 7
    if reason == 'placeholder_unfilled':
        return 2 if depends(subject, preimage) else 8
    if reason == 'precondition_unmet':
        if preimage['probe'] != 'discovery':
            return 3 if preimage['probe'] is not None else 8
        # QD4 affects every generated check. QD2/QD5 affects only discovery.
        others = [e for e in all_evidence if
                  preimages[e['check_id']]['probe'] not in (None, 'discovery')]
        return 3 if any(e['reason'] == 'precondition_unmet' for e in others) else 8
    if reason == 'harness_fault':
        if host['private_copy'] == 'changed':
            return 5
        expected = preimage['expected'] or {}
        captures = expected.get('computation', {}).get('captures', [])
        for invocation in recording['invocations']:
            key = invocation['key']
            if (key['runbook'] == preimage['runbook'] and key['step'] in captures and
                    invocation['confined'][0]['stdout']['cut']):
                return 6
        for key, run in runs:
            if key['computation'] is not None:
                continue
            end = run['end']
            if end.get('by') == 'budget':
                return 7
            if any(a['invocation'] == key and a['by'] == 'engine'
                   for a in host['kill_attribution']):
                return 7
        return 8
    if reason == 'earlier_step_failed':
        return 4
    if reason == 'computation_failed':
        return 6
    if reason in ('could_not_start', 'over_budget', 'crashed', 'wrong_exit'):
        return 7
    if reason in ('held', 'predicate', 'input_missing', 'empty_target'):
        return 8
    raise ValueError('unmapped evidence reason: ' + str(reason))


def check_block(level, levels, subject, preimage):
    for earlier in range(1, level):
        applies = earlier == 1 or trigger(earlier, subject, preimage)
        if earlier not in levels and applies:
            return f'level {earlier} ({LEVEL_RULE[earlier]}) trigger holds; exit 70'
    if level not in levels:
        return f'deciding level {level} ({LEVEL_RULE[level]}) unbuilt'
    return None


def in_scope(dimension, scope):
    # Judge rulebook section 3, lines 58-69.
    special = {'destructive-guard': 'destructive', 'steering': 'steering',
               'tool-description-efficacy': 'compare', 'cli-mcp-parity': None}
    return special.get(dimension, 'conformance') in scope


def dependencies(subject, document, preimages):
    """Construction is required even when QO1 decides every observation.

    U5/U33 promise the kinds in 003/004. Named extra parts identify features
    explicitly left to U39; outcome levels cannot provide these capabilities.
    """
    required = {'QE2.slice', 'QE3.declared_commands', 'QE3.runbooks', 'QE3.scope',
                'QP1', 'QA9.empty_satisfiable', 'QE7.slice', 'QE8', 'QE9',
                'QE10.record', 'QE11', 'QS4', 'QT2', 'QT3', 'QT4', 'QT5.recording_reads',
                'QT5.subject_keys', 'QS7.invocation_keys', 'QE14', 'QE16',
                'QE17', 'QE20.portable', 'QE5', 'QE6', 'QA3.slice'}
    if contains_placeholder(subject):
        required.add('QE2.placeholders')
        required.add('QE3.placeholder_candidates')
    if document['baseline'] is not None:
        required.add('QE4')
    if document['request'] == 'anchor':
        required.add('QE4.anchor')
    for preimage in preimages.values():
        expected = preimage['expected'] or {}
        if expected.get('precondition') is not None:
            required.update(('QA9.preconditions', 'QE7.asserts_emptiness'))
        if expected.get('computation', {}).get('captures'):
            required.update(('QA3.capture_crediting', 'QE2.captures'))
    if any(c['class'] == 'destructive' for c in subject['commands']):
        # Excluding destructive runbooks is U39, rather than U33's ordinary
        # runbooks in the anchors. An included runbook needs no exclusion.
        destructive = [c['words'] for c in subject['commands'] if c['class'] == 'destructive']
        if 'destructive' not in document['scope'] and any(
                step['words'][:len(words)] == words for runbook in subject['runbooks']
                for step in runbook['steps'] for words in destructive
                if any(in_scope(c['dimension'], document['scope'])
                       for st in runbook['steps'] for c in st['checks'])):
            required.add('QA7.destructive_exclusion')
    return required


def derivation(case, directory, built):
    audit = DERIVATION_AUDIT.get(case['id'])
    if audit is None:
        raise ValueError('no subject-file audit for ' + case['id'])
    data = (directory / case['input']['subject_file']).read_bytes()
    if hashlib.sha256(data).hexdigest() != audit['sha256']:
        raise ValueError('subject changed; re-audit rule order: ' + case['id'])
    violations = sorted(audit['violations'], key=lambda r: RULE_ORDER.index(r.split('.')[0]))
    first = violations[0]
    expected = case['expect']['rule']
    earlier = [r for r in violations if RULE_ORDER.index(r.split('.')[0]) < RULE_ORDER.index(expected)]
    blockers = []
    if earlier:
        blockers.append('earlier violation: ' + ', '.join(earlier))
    if first.split('.')[0] != expected:
        blockers.append('first refusal is ' + first + ', expected ' + expected)
    if not supports(names(built['rejects']), first):
        blockers.append('rejects lacks ' + first)
    return {'levels': [], 'blockers': blockers,
            'detail': audit['citation'] + '; violations: ' + ', '.join(violations)}


def predict_case(directory, built):
    case = load(directory / 'case.json')
    row = {'case': case['id'], 'kind': case['kind'], 'predicted': False}
    if case['input']['verifier'] == 'real' or case['id'] in ('case-069', 'case-070', 'case-143'):
        return dict(row, levels=[], blockers=['real verifier excluded'], detail='')
    if case['kind'] == 'derivation-only':
        result = derivation(case, directory, built)
    else:
        document = load(directory / 'input.json')
        preimages = load(directory / 'preimages.json')
        subject = load(directory / case['input']['subject_file'])
        recording = load(KIT / 'behaviour' / (case['input']['form'] + '.json'))
        blockers = []
        levels = []
        for evidence in document['evidence']:
            check = evidence['check_id']
            preimage = preimages[check]
            level = deciding_level(evidence, preimage, subject, case['input']['host'],
                                   recording, document['evidence'], preimages)
            levels.append(level)
            if level >= 6 and has_computation(preimage):
                if not supports(names(built['builds']), 'QT5.computation_keys'):
                    blockers.append(check + ': builds lacks QT5.computation_keys (U39)')
            if (evidence['reason'] == 'placeholder_unfilled' and level == 8
                    and 2 not in built['levels']):
                blockers.append(check + ': QS5 final override requires level 2')
            block = check_block(level, built['levels'], subject, preimage)
            if block:
                blockers.append(check + ': ' + block)
        for rule in sorted(dependencies(subject, document, preimages)):
            if not supports(names(built['builds']), rule):
                blockers.append('builds lacks ' + rule)
        result = {'levels': sorted(set(levels)), 'blockers': blockers, 'detail': ''}
    return dict(row, **result, predicted=not result['blockers'])


def predict(built, cases=None):
    levels = built['levels']
    if any(type(n) is not int or n not in range(1, 9) for n in levels):
        raise ValueError('levels must be integers from 1 to 8')
    rows = [predict_case(d, built) for d in sorted((cases or KIT / 'cases').iterdir())
            if d.is_dir()]
    return {'predicted': [r['case'] for r in rows if r['predicted']], 'cases': rows,
            'reason_to_level': REASON_LEVELS, 'built': built}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--built', required=True, type=Path)
    parser.add_argument('--json', action='store_true')
    args = parser.parse_args()
    result = predict(load(args.built))
    if args.json:
        print(json.dumps(result, indent=2, sort_keys=True))
        return
    print('Predicted: ' + ', '.join(result['predicted']))
    print('case | kind | predicted | deciding levels or blockers')
    for row in result['cases']:
        # The table names the first blocking check; JSON retains EVERY check.
        detail = row['blockers'][0] if row['blockers'] else 'levels ' + str(row['levels'])
        if row['kind'] == 'derivation-only':
            detail += '; ' + row['detail']
        print(f"{row['case']} | {row['kind']} | {'yes' if row['predicted'] else 'no'} | {detail}")


DERIVATION_AUDIT = {'case-100': {'citation': 'case-100/subject.json:81,82; QS2 lines 43-48: repeated declared_count '
                          'key',
              'sha256': 'ecd9dd216c0f205f4f4da5c701b136c7fa0c7dcfb5d510c3bcfa524840924cbe',
              'violations': ['QS2']},
 'case-101': {'citation': 'case-101/subject.json:5; QS4 line 61: subject fact must be boolean',
              'sha256': 'e0a2f5f05159ff95840e98475454df4dd89589eb7dbaaafbb7c92d6e86fbc953',
              'violations': ['QS4', 'QS2']},
 'case-102': {'citation': 'case-102/subject.json:11; QS3 lines 49-60: commands field is absent '
                          'before declared_count',
              'sha256': '7db0d2a062f5f2c7811c4d4bdbd45671fcb1dfb2451400115dcf58eb1825d153',
              'violations': ['QS3.schema', 'QS2']},
 'case-103': {'citation': 'case-103/subject.json:51; QS5 line 62: placeholder in exit_codes is '
                          'forbidden',
              'sha256': '486e4fcdc1f8fd03e8e5be8d86cbaafff0b72efcbb2a51ceeb752e82b6151a7c',
              'violations': ['QS5.schema', 'QS2']},
 'case-104': {'citation': 'case-104/subject.json:46,82,106; QS6 line 88: duplicate command words '
                          'count',
              'sha256': '238ba5bde1339631f63d849e5b2433596d9d56497c5cb446b3b15611d7084afd',
              'violations': ['QS6.semantic']},
 'case-105': {'citation': 'case-105/subject.json:14,72; bare binary line 35: bare binary cannot be '
                          'destructive',
              'sha256': 'fccc22b50a1ffe21614152bbfce94c0cc95fa261a53cfdc13c42576c9e317fb9',
              'violations': ['QS6.semantic']},
 'case-106': {'citation': 'case-106/subject.json:130; QS6 line 88: step names no declared nonbare '
                          'command',
              'sha256': '2f8a00b1ffe9b84239376071b5d95b39f4c128578714dd1b7b4b87c84b1dafc0',
              'violations': ['QS6.semantic']},
 'case-107': {'citation': 'case-107/subject.json:103; QS7 lines 93-97: offline words name no '
                          'command',
              'sha256': '3c643bc78b69665657a9781d7267771b8b451b06395f4a0024a1f4ed8c074b62',
              'violations': ['QS7.semantic']},
 'case-108': {'citation': 'case-108/subject.json:105; QS7 lines 93-97: offline argument is '
                          'forbidden',
              'sha256': '7d96c856c8037344f8b243bdf31a60abeff8f416d318d7e3f29b87981593a57f',
              'violations': ['QS7.semantic']},
 'case-109': {'citation': 'case-109/subject.json:118; QS7 lines 93-97: two settings set the same '
                          'environment name',
              'sha256': 'baa98a51563d70613f1fb95b237495f0ec5383dcdb6a4e7d1b179d394f14e875',
              'violations': ['QS7.semantic']},
 'case-110': {'citation': 'case-110/subject.json:3; QS8 line 100: binary leaves assembled '
                          'directory',
              'sha256': 'aa2daa0a417fde567bb409706b739184b78e8363f4e6ebdc3bf517ad19ffb533',
              'violations': ['QS8.semantic']},
 'case-111': {'citation': 'case-111/subject.json:83; QS9 line 106: store not supplied by seed',
              'sha256': '677fab486ef67a90494902f8ff004cc34125090d6d31281c1ad6a9bf5ed2333c',
              'violations': ['QS9.semantic']},
 'case-112': {'citation': 'case-112/subject.json:82; QS9 lines 101-104: case fixture adds '
                          'seed/linked.json as a symbolic link',
              'sha256': 'f7e739405d8dcbec121840546c1b6580897d046ad99e1b024194e15d54662fa6',
              'violations': ['QS9.semantic']},
 'case-113': {'citation': 'case-113/subject.json:125,476; QS10 line 108: runbook read repeated',
              'sha256': '517cba5647408f9d9aee06397cc03e773980cb95f92aff61bbfd340b62f78024',
              'violations': ['QS10']},
 'case-114': {'citation': 'case-114/subject.json:140; QS11 line 112: literal nested deeper than '
                          '48; schema permits arbitrary literal',
              'sha256': '5a0793729b55d3bd5766f0b485957465d9da1ef5735347de402c47c581542bd9',
              'violations': ['QS11.literal']},
 'case-115': {'citation': 'case-115/subject.json:141; QS11 line 113: literal not representable '
                          'losslessly as binary64',
              'sha256': 'c890849495109869fb1c8ac73f0352026c5c9b9875dd4fa1bdb3ab0f9c10b3b4',
              'violations': ['QS11.literal']},
 'case-116': {'citation': 'case-116/subject.json:128; QS11 line 114: step name holds U+0009; '
                          'schema pattern rejects it',
              'sha256': 'bbcb38ed4e81880cae6f60655559f0b023819dc0796222cbad872bf7db904bda',
              'violations': ['QS11.schema', 'QS2']},
 'case-117': {'citation': 'case-117/subject.json:3; QS12 line 115: binary and computation '
                          'executable are the same file',
              'sha256': '3def4d28e1802976b2c8371c7252b552419a46c38c486294097d3a24542c1569',
              'violations': ['QS12']},
 'case-118': {'citation': 'case-118/subject.json:109; QP7 lines 210-212: bookkeeping path is the '
                          'declared store',
              'sha256': '014574b738f16940b63a8754f5f39c14cda61646c091b57d4c2012c1b52be53e',
              'violations': ['QP7']},
 'case-119': {'citation': 'case-119/subject.json:113; QP10 lines 225-234: secret path not supplied '
                          'by seed',
              'sha256': 'b92ee1fc344be0afd4fc06e16054442d6403c16e4f3b8cb10513d4576c5c15f1',
              'violations': ['QP10.semantic']},
 'case-120': {'citation': 'case-120/subject.json:113; QP10 lines 225-234: secret path equals '
                          'store; token also absent',
              'sha256': '6a419b4f4adc316167b725f2f9cf055fb12a673f420b56d52866d2fb408dc90c',
              'violations': ['QP10.semantic']},
 'case-121': {'citation': 'case-121/subject.json:116; QP10 lines 225-234: token absent from secret '
                          'file',
              'sha256': 'dc5bca705004bc5fbba934ff08d8b51e80fbb0b840d6d5454ca39aad08ee931e',
              'violations': ['QP10.semantic']},
 'case-122': {'citation': 'case-122/subject.json:112; QP10 lines 225-234: secret command remove '
                          'has destructive class',
              'sha256': 'cb7de06747c89fecd4b651de23e12eafb6fe0370988056c5aabcd6b0733847c5',
              'violations': ['QP10.semantic']},
 'case-123': {'citation': 'case-123/subject.json:197; QA4 line 275: capture names a later step',
              'sha256': '4d51bdbbb799481161fa411a54324449d4f2672e52317519af87616888a9fe44',
              'violations': ['QA4.captures']},
 'case-124': {'citation': 'case-124/subject.json:196; QA4 line 270: absolute computation '
                          'executable outside /bin and /usr/bin',
              'sha256': 'f6071f86bb86779922fc24984fb6c1c1972da87a9bdff1fa4a0bd2fdacd4e63f',
              'violations': ['QA4.executable']},
 'case-125': {'citation': 'case-125/subject.json:328; QA9 line 308: exit_code expected value is a '
                          'string',
              'sha256': '5bb4f2132bb073e1c7d3379a51b68d1b5dfc69973e61399e26e9e5165a6c643a',
              'violations': ['QA9.validation']},
 'case-126': {'citation': 'case-126/subject.json:328,363; QA9 line 309: exit_code check also gives '
                          'exit_codes',
              'sha256': '99ddebf8d742b683e681b51b8bde5f86b08eee3ded3847363ae7d12e3179c5e2',
              'violations': ['QA9.validation']},
 'case-127': {'citation': 'case-127/subject.json:229; QA9 line 313: emptiness assertion lacks '
                          'precondition',
              'sha256': 'cd3b603a95febd5f43cd5af84071ec1a3058079d3e5d82bcaa0db52b25472120',
              'violations': ['QA9.preconditions']},
 'case-128': {'citation': 'case-128/subject.json:165,192,301,428; QA9 line 307: stdout_json_equals '
                          'requires null path',
              'sha256': '19af3e31909996ada81cbcfaf8547b506e1f30b6a6832c166b4bf996c1ddb8f9',
              'violations': ['QA9.validation']},
 'case-129': {'citation': 'case-129/subject.json:2; QS1 line 42: unsupported subject version',
              'sha256': '224a893277cf16c90f99e9ea2b8ae29b9b29cc0f1b0780f427b9ca3ccf86619a',
              'violations': ['QS1', 'QS2']},
 'case-136': {'citation': 'case-136/subject.json:476; collection-contract.md QE6 lines 129-140: '
                          'marker on Safety dimension rejected by pre-check X4',
              'sha256': 'edd0a814e47c0bd2792a5108033006ddb1323f4cb55d8a7c3cadf66705342c4d',
              'violations': ['QE6']}}

if __name__ == '__main__':
    main()
