#!/usr/bin/env python3
"""Independent KTD15 count and schema oracle for corpus runner exports."""
import argparse
import json
from pathlib import Path
import sys


def expected_cases(cases, pending):
    excluded = {line.strip() for line in pending.read_text().splitlines() if line.strip() and not line.strip().startswith('#')}
    expected = set()
    found = set()
    for directory in sorted(cases.iterdir()):
        case = json.loads((directory / 'case.json').read_text())
        if case['id'] != directory.name:
            raise ValueError('case id mismatch: ' + directory.name)
        found.add(case['id'])
        if (case['kind'] == 'judge-checked' and case['input']['verifier'] == 'scripted'
                and case['id'] not in excluded):
            expected.add(case['id'])
    if excluded - found:
        raise ValueError('unknown pending cases')
    return expected


def validate(kit, cases, pending, copies):
    # Import only in validation, so count controls run with plain isolated Python.
    expected = expected_cases(cases, pending)
    wanted = {name + suffix for name in expected for suffix in ('.record.json',)}
    actual = {p.name for p in copies.iterdir()}
    # Baselines are validated only when a real path stores one.
    baselines = {name + '.baseline.json' for name in expected} & actual
    wanted |= baselines
    if actual != wanted:
        raise ValueError('export file set differs: missing=' + repr(sorted(wanted - actual))
                         + ' extra=' + repr(sorted(actual - wanted)))
    if not expected:
        count = len({line.strip() for line in pending.read_text().splitlines()
                     if line.strip() and not line.strip().startswith('#')})
        print(f'nothing to validate: {count} pending')
        return
    import jsonschema
    from referencing import Registry, Resource
    # Resolve only resources in the sealed schema directory; never fetch refs.
    registry = Registry()
    for path in sorted((kit / 'schema').glob('*.schema.json')):
        document = json.loads(path.read_text())
        resource = Resource.from_contents(document)
        registry = registry.with_resource(path.name, resource)
    # Schema refs resolve against the sealed kit, not implementation copies.
    for suffix, schema_name in (('.record.json', 'record'), ('.baseline.json', 'baseline')):
        schema_path = kit / 'schema' / (schema_name + '.schema.json')
        schema = json.loads(schema_path.read_text())
        validator = jsonschema.Draft202012Validator(
            schema, registry=registry)
        for name in sorted(expected):
            if schema_name == 'baseline' and name + suffix not in baselines:
                continue
            document = json.loads((copies / (name + suffix)).read_text())
            validator.validate(document)
            if schema_name == 'record' and document['judge_rejected']:
                raise ValueError('corpus record was judge rejected: ' + name)
    print(f'corpus exports: {len(expected)} records and {len(baselines)} baselines validated')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--kit', type=Path, required=True)
    parser.add_argument('--pending', type=Path, required=True)
    parser.add_argument('--copies', type=Path, required=True)
    args = parser.parse_args()
    validate(args.kit, args.kit / 'cases', args.pending, args.copies)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError) as error:
        sys.exit(str(error))
