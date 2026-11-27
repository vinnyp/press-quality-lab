"""Predict a scripted corpus ratchet; never run a subject or infer a verdict.

The derivation audit below is deliberately finite: it audits the sealed corpus,
not arbitrary subject files. Digests fail closed if an audited file changes.
Rule order: collection errata E-9 (QS2 is the fallback, not the first rule).
Reason mapping citations are lines of collection-rules.md in the sealed kit.
"""
import argparse
import hashlib
import json
import re
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
# collection-rules.md §2: levels 1–5 at lines 122–126;
# level 6: QO6 at 159–161 (order at 127);
# level 7: QO2 at 141, QO3 at 142, QO4 at 146, QO5 at 151;
# level 8 (order at 129): discovery QD2/QD3/QD5 at 177/185/189;
# probes QP1/QP2/QP3/QP4 at 198/202/203/204, store QP6–QP9
# at 209/210/213/217, secret QO7/QP10–QP13 at 162/225/237/249/252;
# authored predicates QA9 at 292.
LEVEL_RULE = {
    1: 'QO1',
    2: 'QS5',
    3: 'QD4',
    4: 'QA7',
    5: 'QH8',
    6: 'QO6',
    7: (
        'QO2',
        'QO3',
        'QO4',
        'QO5',
    ),
    8: (
        'QD2',
        'QD3',
        'QD5',
        'QP1',
        'QP2',
        'QP3',
        'QP4',
        'QP6',
        'QP7',
        'QP8',
        'QP9',
        'QO7',
        'QP10',
        'QP11',
        'QP12',
        'QP13',
        'QA9',
    ),
}


# credit-and-routes §1; collection-rules QP2–QP13 and QA9.
# QO7 combines invocation results at level 7, before the probe rules.
PROBE_LEVEL_EIGHT_RULES = {
    'discovery': (
        'QD2',
        'QD3',
        'QD5',
    ),
    'bare': ('QP2',),
    'byte_bound': ('QP3',),
    'exit_codes': ('QP4',),
    'store_unchanged': (
        'QP6',
        'QP7',
        'QP8',
        'QP9',
    ),
    'planted_secret': (
        'QP10',
        'QP11',
        'QP12',
        'QP13',
    ),
    None: ('QA9',),
}


# Only probes have independently buildable level-8 capabilities.
PROBE_LEVEL_EIGHT_BUILDS = {
    probe: rules for probe, rules in PROBE_LEVEL_EIGHT_RULES.items()
    if probe not in (None, 'discovery')
}
PROBE_LEVEL_EIGHT_BUILDS['discovery'] = (
    'QD2.bounds',
    'QD3',
    'QD5',
    'QO7.discovery',
)


def level_rules(level):
    rules = LEVEL_RULE[level]
    return (rules,) if isinstance(rules, str) else rules


RULE_ORDER = ['QS1'] + ['QS' + str(i) for i in range(3, 13)] + [
    'QP7', 'QP10', 'QA4', 'QA9', 'QS2', 'QE6']

# None means pre-collection construction/pre-check; record means after collection.
# An integer means observation support, needed only at that invocation's serving level.
# override means the QS5 step after level 8, independent of its level-2 step.
BUILD_PHASE = {
    'QA8.destructive_runbook': 4,
    'QP2': 8,
    'QP3': 8,
    'QP4': 8,
    'QP6': 8,
    'QP7': 8,
    'QP8': 8,
    'QP9': 8,
    'QP10': 8,
    'QP11': 8,
    'QP12': 8,
    'QP13': 8,
    'QD2.bounds': 8,
    'QD3': 8,
    'QD5': 8,
    'QO7.discovery': 8,
    'QO7.planted_secret': 7,
    'QS5.final_override': 'override',
    'QS5.qd3_trigger': 'override',
    'QS8.safe_read': 'safe_read',
    'QS9.safe_read': 'safe_read',
    'QE2.slice': None, 'QE2.placeholders': None, 'QE2.captures': None,
    'QE3.declared_commands': None, 'QE3.runbooks': None, 'QE3.scope': None,
    'QE3.placeholder_candidates': None, 'QP1': None,
    'QA9.empty_satisfiable': None, 'QA9.preconditions': None,
    'QE7.slice': None, 'QE7.asserts_emptiness': None,
    'QE8': 'record', 'QE9': 'record', 'QE10.record': 'record', 'QE11': 'record',
    'QS4': 'record', 'QT2': 'record', 'QT3': None, 'QT4': None,
    'QT5.recording_reads': None, 'QT5.subject_keys': 7,
    'QT5.computation_keys': 6, 'QS7.invocation_keys': 7,
    'QE14': 'record', 'QE16': 'record', 'QE17': 'record', 'QE20.portable': None,
    'QE5': None, 'QE6': None, 'QA3.slice': None,
    'QA3.capture_crediting': None, 'QA7.destructive_exclusion': None,
    'QE4': None, 'QE4.anchor': None,
}

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


def contains_placeholder(subject):
    if any(placeholder(subject.get(k)) for k in
           ('binary', 'declared_count', 'seed', 'store', 'secret')):
        return True
    if any(placeholder(v) for v in subject.get('facts', {}).values()):
        return True
    return any(placeholder(c.get(k)) for c in subject.get('commands', [])
               for k in ('class', 'writes'))


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
    # E-51: a captured step's command is also a dependency of the check.
    captures = (preimage.get('expected') or {}).get('computation', {}).get('captures', [])
    words = list(words)
    for runbook in subject.get('runbooks', []):
        if runbook['name'] == preimage.get('runbook'):
            words.extend(step['words'] for step in runbook['steps']
                         if step['name'] in captures)
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


def planted_secret_level(preimage, subject, host, runs):
    """QO7's combination, with E-35 through E-38, in deciding order."""
    if subject.get('secret') is None:
        return 8
    ordered = [(key, run) for words in preimage['parameters']['commands']
               for key, run in runs if key['words'] == words]
    if any(run['leak_scan']['leaks'] for _, run in ordered):
        return 8
    if any(run['end'].get('by') != 'budget' and
           any(a['invocation'] == key and a['by'] == 'engine'
               for a in host['kill_attribution']) for key, run in ordered):
        return 7
    if host.get('read_calibration', 'moved') == 'moved':
        for key, run in ordered:
            commands = [c for c in subject['commands']
                        if key['words'][:len(c['words'])] == c['words']]
            command = max(commands, key=lambda c: len(c['words']), default={})
            planted = run['planted']
            if (run['end']['kind'] == 'exit' and
                    run['end']['code'] in (command.get('exit_codes') or [0]) and
                    planted is not None and planted['atime_moved'] and planted['same_file']):
                control = host.get('read_control')
                killed = (control == 'killed' if control is not None else
                          planted['control_end'] == dict(kind='signal', signal='SIGKILL',
                                                         by='confinement'))
                if not killed:
                    return 8
    for _, run in ordered:
        if (run['markers']['could_not_start'] or
                run['end'].get('by') in ('confinement', 'budget')):
            return 7
    # QP13's bound, a proved read, and input_missing all decide at level 8.
    return 8


def deciding_level(evidence, preimage, subject, host, recording, all_evidence, preimages):
    reason = evidence['reason'] or evidence['failure_kind'] or evidence['outcome']
    runs = runs_for(preimage, recording)
    # E-15: grade-wide QO1 covers even checks that run nothing.
    # collection-rules.md lines 121-122 and 136: QH6 is tried at level 1.
    if reason == 'network_unverifiable':
        if (not host['confinement']['applied'] or host['self_test'] == 'failed' or
                any(key['computation'] is None and not r['markers']['confinement']
                    and (r['end']['kind'] == 'exit' or
                         r['end'].get('by') == 'before_marker') for key, r in runs)):
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
        if preimage['probe'] == 'planted_secret':
            return planted_secret_level(preimage, subject, host, runs)
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
    table_reason = ('held/predicate/input_missing/empty_target'
                    if reason in ('held', 'predicate', 'input_missing', 'empty_target') else reason)
    if table_reason in REASON_LEVELS:
        levels = REASON_LEVELS[table_reason]['levels']
        if len(levels) == 1:
            return levels[0]
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
    required = {
        'QE2.slice',
        'QE3.declared_commands',
        'QE3.runbooks',
        'QE3.scope',
        'QP1',
        'QA9.empty_satisfiable',
        'QE7.slice',
        'QE8',
        'QE9',
        'QE10.record',
        'QE11',
        'QS4',
        'QT2',
        'QT3',
        'QT4',
        'QT5.recording_reads',
        'QT5.subject_keys',
        'QS7.invocation_keys',
        'QE14',
        'QE16',
        'QE17',
        'QE20.portable',
        'QE5',
        'QE6',
        'QA3.slice',
    }
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


def ending(exit_code, rules=(), phase='deciding'):
    return {'exit_code': exit_code, 'rules': sorted(set(rules)), 'phase': phase}


def pending_rules(built):
    # A missing refusal may proceed only to an engine fault, never a verdict.
    capabilities = names(built['builds']) | names(built['rejects'])
    candidates = {rule for level in LEVEL_RULE if level not in built['levels']
                  for rule in level_rules(level)}
    candidates.update(rule for rule, phase in BUILD_PHASE.items()
                      if phase in (None, 'record'))
    candidates.add('QE6')
    gaps = {rule for rule in candidates if not supports(capabilities, rule)}
    partial = {rule.split('.')[0] for rule in capabilities}
    return {rule if rule.split('.')[0] in partial else rule.split('.')[0]
            for rule in gaps} | {
                'QE2',
                'QE6',
            }


def check_ending(level, built, subject, preimage, gaps):
    planning = {r for r in gaps if BUILD_PHASE[r] is None}
    if planning:
        return ending(70, planning, 'planning')
    for current in range(1, level + 1):
        keys = {r for r in gaps if BUILD_PHASE[r] == current}
        if keys:
            return ending(70, keys, 'observation')
        applies = current == 1 or trigger(current, subject, preimage)
        capabilities = names(built.get('builds', []))
        # QO7 combines planted-secret runs before their level-7 result is known.
        if (current == 7 and preimage['probe'] == 'planted_secret' and applies and
                current in built['levels'] and
                not supports(capabilities, 'QO7.planted_secret')):
            return ending(70, ['QO7'], 'sentinel')
        # QO7 also combines discovery requests decided at level 7.
        if (current == 7 and preimage['probe'] == 'discovery' and applies and
                current in built['levels'] and
                not supports(capabilities, 'QO7.discovery')):
            return ending(70, ['QO7'], 'sentinel')
        tokens = PROBE_LEVEL_EIGHT_BUILDS.get(preimage['probe'])
        probe_built = (current == 8 and tokens is not None and
                       all(supports(capabilities, token) for token in tokens))
        if current not in built['levels'] and applies and not probe_built:
            return ending(70, PROBE_LEVEL_EIGHT_RULES[preimage['probe']]
                          if current == 8 else level_rules(current), 'sentinel')
    return ending(None, level_rules(level))


def destructive_runbook(subject, preimage):
    """QA7/QA8: one destructive step makes the entire included runbook destructive."""
    for runbook in subject.get('runbooks', []):
        if runbook['name'] != preimage.get('runbook'):
            continue
        for step in runbook['steps']:
            candidates = [c for c in subject['commands']
                          if step['words'][:len(c['words'])] == c['words']]
            command = max(candidates, key=lambda c: len(c['words']), default={})
            if command.get('class') == 'destructive':
                return True
    return False


def required_dependencies(subject, document, preimages):
    # Required-set construction and pre-check, before collection or recording.
    return {r for r in dependencies(subject, document, preimages)
            if BUILD_PHASE[r] is None and r.split('.')[0] in {
                'QE2', 'QE3', 'QE4', 'QE5', 'QE6', 'QE7', 'QP1', 'QA9', 'QA3', 'QA7'}}


def safe_read_refusal(case, directory, built):
    """Pre-collection safe-read faults, before planning and the pre-check.

    Corpus inputs are assembled from the named form plus fixture overlays.
    This legacy phase preserves PR6's faults until U9 supplies full refusals.
    PR8 retires both safe-read tokens; its QS9 audit rejects contained links,
    not links naming the seed (errata E-17).
    """
    subject = load(directory / case['input']['subject_file'])
    capabilities = names(built['builds'])
    if supports(capabilities, 'QS8.safe_read'):
        named = [subject.get('binary')]

        def visit(value):
            if isinstance(value, dict):
                if 'argv' in value:
                    named.extend(value.get('declared_files', []))
                    if value['argv'] and not value['argv'][0].startswith('/'):
                        named.append(value['argv'][0])
                for child in value.values():
                    visit(child)
            elif isinstance(value, list):
                for child in value:
                    visit(child)
        visit(subject)
        for name in named:
            if not isinstance(name, str):
                continue
            parts = []
            unsafe = name.startswith('/')
            for part in name.split('/'):
                if part == '..':
                    if not parts:
                        unsafe = True
                    else:
                        parts.pop()
                elif part not in ('', '.'):
                    parts.append(part)
            if unsafe:
                return 'QS8'
    seed = subject.get('seed')
    if isinstance(seed, str) and supports(capabilities, 'QS9.safe_read'):
        for fixture in case['input'].get('fixtures', []):
            if (fixture['kind'] == 'link' and
                    (fixture['path'] == seed or fixture['path'].startswith(seed + '/'))):
                return 'QS9'
    return None


def predict_case(directory, built):
    case = load(directory / 'case.json')
    row = {'case': case['id'], 'kind': case['kind'], 'predicted': False}
    if case['input']['verifier'] == 'real' or case['id'] in ('case-069', 'case-070', 'case-143'):
        return dict(row, levels=[], blockers=['real verifier excluded'], detail='',
                    ending=ending(None, phase='excluded'), checks=[])
    if case['kind'] == 'derivation-only':
        result = derivation(case, directory, built)
        gaps = []
        if case['expect']['rule'] == 'QE6':
            subject = load(directory / case['input']['subject_file'])
            preimages = {str(i): c for i, c in enumerate(
                c for rb in subject['runbooks'] for st in rb['steps'] for c in st['checks'])}
            gaps = sorted(r for r in required_dependencies(subject, case['input'], preimages)
                          if not supports(names(built['builds']), r))
            result['blockers'] += ['builds lacks ' + r for r in gaps]
        result['ending'] = (ending(70, gaps, 'planning') if gaps else
                            ending(2, [case['expect']['rule']]) if not result['blockers'] else
                            ending(70, pending_rules(built), phase='unrefused'))
    else:
        document = load(directory / 'input.json')
        preimages = load(directory / 'preimages.json')
        subject = load(directory / case['input']['subject_file'])
        recording = load(KIT / 'behaviour' / (case['input']['form'] + '.json'))
        common_gaps = {r for r in dependencies(subject, document, preimages)
                       if not supports(names(built['builds']), r)}
        checks = []
        for evidence in sorted(document['evidence'], key=lambda e: e['check_id']):
            check = evidence['check_id']
            preimage = preimages[check]
            level = deciding_level(evidence, preimage, subject, case['input']['host'],
                                   recording, document['evidence'], preimages)
            gaps = {r for r in common_gaps if BUILD_PHASE[r] is None or
                    (BUILD_PHASE[r] == 7 and has_invocation(preimage))}
            if has_computation(preimage) and not supports(names(built['builds']), 'QT5.computation_keys'):
                gaps.add('QT5.computation_keys')
            check_end = check_ending(level, built, subject, preimage, gaps)
            if (check_end['phase'] == 'deciding' and level == 8 and
                    evidence['reason'] == 'placeholder_unfilled'):
                # QS5's final override follows level 8, independently of level 2.
                # An unbuilt earlier level still wins; this is reachable only
                # after check_ending has passed every level through 8.
                token = ('QS5.final_override' if contains_placeholder(subject)
                         else 'QS5.qd3_trigger')
                if not supports(names(built['builds']), token):
                    check_end = ending(70, [token], 'observation')
                else:
                    check_end = ending(None, ['QS5'])
            # QA8 is a fallback: preserve all earlier slices' existing blockers.
            if (check_end['phase'] == 'deciding' and
                    not any(BUILD_PHASE[r] == 'record' for r in common_gaps) and
                    destructive_runbook(subject, preimage) and
                    not supports(names(built['builds']), 'QA8.destructive_runbook')):
                check_end = ending(70, ['QA8'], 'sentinel')
            checks.append({'id': check, 'deciding_level': level, 'ending': check_end})
        planning = {r for c in checks if c['ending']['phase'] == 'planning'
                    for r in c['ending']['rules']}
        sentinels = {r for c in checks if c['ending']['phase'] in ('sentinel', 'observation')
                     for r in c['ending']['rules']}
        record = {r for r in common_gaps if BUILD_PHASE[r] == 'record'}
        case_end = (ending(70, planning, 'planning') if planning else
                    ending(70, sentinels, 'sentinel') if sentinels else
                    ending(70, record, 'record') if record else
                    ending(case['expect']['exit_code'], [case['expect'].get('rule')]
                           if case['expect'].get('rule') else []))
        blockers = (['builds lacks ' + r for r in sorted(planning)] if planning else
                    ['first sentinel: ' + r for r in sorted(sentinels)] if sentinels else
                    ['builds lacks ' + r for r in sorted(record)])
        result = {'levels': sorted({c['deciding_level'] for c in checks}),
                  'blockers': blockers, 'detail': '', 'checks': checks, 'ending': case_end}
    # Built schema refusals precede filesystem snapshots. Pending semantic
    # refusals can instead encounter a safe-read engine fault.
    if result['blockers']:
        refusal = safe_read_refusal(case, directory, built)
        if refusal:
            result['ending'] = ending(70, [refusal], 'safe_read')
            result['blockers'] = ['safe-read refusal: ' + refusal]
            result['checks'] = []
    return dict(row, **result, predicted=not result['blockers'])


def validate_rules(built):
    valid = set((ROOT / 'internal/docrefs/unmapped-rules.txt').read_text().splitlines())
    valid.update(load(ROOT / 'internal/docrefs/rule-ledger.json')['rules'])
    for group in ('rejects', 'builds'):
        for rule in names(built[group]):
            if not re.fullmatch(r'Q[A-Z][0-9]+(?:\.[A-Za-z0-9_]+)?', rule) or rule.split('.')[0] not in valid:
                raise ValueError('unknown rule: ' + rule)
            if group == 'builds' and rule not in BUILD_PHASE:
                raise ValueError('unclassified builds token: ' + rule)


def predict(built, cases=None):
    validate_rules(built)
    levels = built['levels']
    if any(type(n) is not int or n not in range(1, 9) for n in levels):
        raise ValueError('levels must be integers from 1 to 8')
    rows = [predict_case(d, built) for d in sorted((cases or KIT / 'cases').iterdir())
            if d.is_dir()]
    result = {'predicted': [r['case'] for r in rows if r['predicted']], 'cases': rows,
              'reason_to_level': REASON_LEVELS, 'built': built}
    if 'QS5.final_override' in names(built['builds']):
        result['placeholder_dependencies'] = {
            d.name: {key: depends(load(d / 'subject.json'), pre)
                     for key, pre in load(d / 'preimages.json').items()}
            for d in sorted((cases or KIT / 'cases').iterdir())
            if (d / 'preimages.json').exists()}
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--built', required=True, type=Path)
    parser.add_argument('--json', action='store_true')
    parser.add_argument('--out', type=Path)
    args = parser.parse_args()
    result = predict(load(args.built))
    if args.out:
        args.out.write_text(json.dumps(result, indent=2, sort_keys=True) + '\n', encoding='utf-8')
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
