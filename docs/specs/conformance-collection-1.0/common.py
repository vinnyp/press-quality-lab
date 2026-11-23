# /// script
# dependencies = ["jsonschema==4.23.0"]
# ///
"""Bounded execution, document integrity, and partial comparison utilities."""

import re
import hashlib
import json
import math
import os
from pathlib import Path
import selectors
import shlex
import signal
import sys
sys.dont_write_bytecode = True
import io
import subprocess
import time
import jsonschema

LIMIT = 2 * 1024 * 1024
INPUT_LIMIT = 32 * 1024 * 1024
OUTPUT_LIMIT = 32 * 1024 * 1024


class Error(Exception):
    pass


def load(data):
    """Parse JSON and reject nonfinite constants or excessive nesting.

    Args:
        data: Raw bytes to parse or send unchanged to standard input.
    """
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate object key: " + key)
            result[key] = value
        return result

    try:
        value = json.loads(
            data, object_pairs_hook=unique, parse_constant=lambda x: (_ for _ in ()).throw(ValueError(x))
        )
        stack = [(value, 0)]
        while stack:
            node, depth = stack.pop()
            if depth > 60:
                raise ValueError("document nesting limit")
            if isinstance(node, dict):
                stack.extend(((v, depth + 1) for v in node.values()))
            elif isinstance(node, list):
                stack.extend(((v, depth + 1) for v in node))
        return value
    except (ValueError, RecursionError, UnicodeError) as exc:
        raise Error("invalid JSON: " + str(exc)) from exc


def read(path, limit=LIMIT):
    """Read a file within the local byte limit.

    Args:
        path: File path or selected JSON field path.
    """
    with open(path, "rb") as f:
        data = f.read(limit + 1)
    if len(data) > limit:
        raise Error("file size limit: " + str(path))
    return data


def manifest(root, name):
    """Verify digest entries and exact file coverage.

    Args:
        root: Root directory containing the relevant documents.
        name: Manifest filename or selected document name.
    """
    entries = {}
    for line in io.StringIO(read(root / name).decode("utf-8")):
        digest, separator, rel = line.rstrip("\n").partition("  ")
        if not separator:
            raise Error("invalid digest entry")
        path = root / rel
        if Path(rel).is_absolute() or ".." in Path(rel).parts or any(q.is_symlink() for q in (path, *path.parents) if q != root.parent):
            raise Error("unsafe digest path")
        if rel in entries or len(digest) != 64:
            raise Error("invalid digest entry")
        entries[rel] = digest
        if file_digest(path) != digest:
            raise Error("digest mismatch: " + rel)
    if any(p.is_symlink() for p in root.rglob("*")):
        raise Error("unsafe digest path: symlink")
    actual = {
        str(p.relative_to(root))
        for p in root.rglob("*")
        if p.is_file() and p != root / name and not (name == "MANI" + "FEST.sha256" and p == root / "SEALS.log")
    }
    if actual != set(entries):
        raise Error("digest file coverage mismatch")
    return entries



FIELDS = {
    "verdict": {"token", "sub_label", "plain"},
    "own_verdict": {"token", "sub_label", "plain"},
    "counts": {"xfail", "xpass"},
    "checks": {
        "result",
        "rule",
        "reason",
        "detail",
        "k",
        "n",
        "to_run",
        "lower",
        "upper",
        "d_lo",
        "d_hi",
    },
    "dimensions": {"state", "rule", "reason"},
    "groups": {"state", "rule"},
    "comparison": {"result", "rule", "next_action", "new", "entries"},
    "anchor": {"result", "rule", "reason", "checks", "baseline", "baseline_count", "replaced"},
}
DIRECT = {
    "outcome",
    "exit_code",
    "rejected_by",
    "stale_markers",
    "set_aside_count",
    "earlier_pools_count",
}
LIST_FIELDS = {"set_aside", "earlier_pools"}
BASE_FIELDS = {"kind", "dimension", "result", "tier", "condition", "k", "n"}


def validate_expect(expect):
    """Reject unsupported expectation keys and field shapes.

    Args:
        expect: Partial expectation object to validate or compare.
    """
    if not isinstance(expect, dict) or set(expect) - DIRECT - set(FIELDS) - LIST_FIELDS:
        raise Error("unknown expect key or invalid expect")
    for key, value in expect.items():
        if key in ("set_aside", "earlier_pools"):
            if not isinstance(value, list):
                raise Error("expect list required: " + key)
            for entry in value:
                fields = {"check_id", "arm", "condition", "trial_index", "reason"} if key == "set_aside" else {"check_id", "arm", "condition", "k", "n", "rule"}
                if not isinstance(entry, dict) or set(entry) != fields:
                    raise Error("malformed expect entry: " + key)
                if not isinstance(entry["check_id"], str) or not isinstance(entry["condition"], str) or entry["arm"] not in (None, "A", "B"):
                    raise Error("malformed expect entry: " + key)
                if key == "set_aside":
                    numbers = [entry["trial_index"]]
                    good = entry["reason"] in ("wrong_tier", "other_arm", "earlier_pool")
                else:
                    numbers = [n for n in (entry["k"], entry["n"]) if n is not None]
                    good = isinstance(entry["rule"], str) and re.fullmatch(r"P[013-5]", entry["rule"]) is not None
                if not good or any(isinstance(n, bool) or not isinstance(n, (int, float)) or isinstance(n, float) and not math.isfinite(n) or n < 0 or n != int(n) for n in numbers):
                    raise Error("malformed expect entry: " + key)
        if key not in FIELDS or value is None:
            continue
        if not isinstance(value, dict):
            raise Error("expect object required: " + key)
        if key in ("checks", "dimensions", "groups"):
            for fields in value.values():
                if not isinstance(fields, dict) or set(fields) - FIELDS[key]:
                    raise Error("unknown expect field: " + key)
        else:
            if set(value) - FIELDS[key]:
                raise Error("unknown expect field: " + key)
            for nested, allowed in [
                ("entries", {"class", "rule"}),
                ("baseline", BASE_FIELDS),
                ("replaced", BASE_FIELDS),
            ]:
                if nested in value:
                    if not isinstance(value[nested], dict):
                        raise Error("expect mapping required")
                    for fields in value[nested].values():
                        if fields is None and nested in ("baseline", "replaced"):
                            continue
                        if not isinstance(fields, dict) or set(fields) - allowed:
                            raise Error("unknown expect nested field")


MISSING = "<missing>"


def comparisons(expect, output):
    """Return selected paths with their expected and actual values.

    Args:
        expect: Partial expectation object to validate or compare.
        output: Parsed output value to compare.
    """
    validate_expect(expect)
    pairs = []

    def field(path, expected, actual):
        """Append one selected path and its compared values.

        Args:
            path: File path or selected JSON field path.
            expected: Selected expected value or field mapping.
            actual: Actual output value or collection.
        """
        pairs.append((path, expected, actual))

    def partial(path, expected, actual):
        """Collect selected fields from an output object.

        Args:
            path: File path or selected JSON field path.
            expected: Selected expected value or field mapping.
            actual: Actual output value or collection.
        """
        for k, v in expected.items():
            field(
                path + "." + k,
                v,
                actual.get(k, MISSING) if isinstance(actual, dict) else MISSING,
            )

    def keyed(path, expected, actual, identity):
        """Compare selected entries by their identity field.

        Args:
            path: File path or selected JSON field path.
            expected: Selected expected value or field mapping.
            actual: Actual output value or collection.
            identity: Field identifying each keyed entry.
        """
        index = (
            {v[identity]: v for v in actual if isinstance(v, dict) and isinstance(v.get(identity), str)}
            if isinstance(actual, list)
            else {}
        )
        for k, v in expected.items():
            if v is None:
                field(path + "." + k, False, k in index if isinstance(actual, list) else MISSING)
            elif k not in index:
                field(path + "." + k, v, MISSING)
            else:
                partial(path + "." + k, v, index[k])

    out = output if isinstance(output, dict) else {}
    for k, v in expect.items():
        got = out.get(k, MISSING)
        if v is None:
            field(k, v, got)
        elif k in ("checks", "dimensions", "groups"):
            keyed(
                k,
                v,
                got,
                {"checks": "id", "dimensions": "dimension", "groups": "group"}[k],
            )
        elif k in ("comparison", "anchor"):
            obj = got if isinstance(got, dict) else {}
            for sub, val in v.items():
                if sub == "entries":
                    keyed(k + ".entries", val, obj.get(sub), "check_id")
                elif sub == "baseline":
                    keyed(
                        k + ".baseline",
                        val,
                        (
                            (obj.get("baseline") or {}).get("entries")
                            if isinstance(obj.get("baseline"), dict)
                            else None
                        ),
                        "check_id",
                    )
                elif sub == "replaced":
                    keyed(k + ".replaced", val, obj.get("replaced") if isinstance(obj.get("replaced"), list) else None, "check_id")
                elif sub == "baseline_count":
                    field(
                        k + ".baseline_count",
                        val,
                        (
                            len(obj["baseline"]["entries"])
                            if isinstance(obj.get("baseline"), dict)
                            and isinstance(obj["baseline"].get("entries"), list)
                            else MISSING
                        ),
                    )
                else:
                    field(k + "." + sub, val, obj.get(sub, MISSING))
        elif k in FIELDS:
            partial(k, v, got)
        elif k.endswith("_count"):
            field(
                k,
                v,
                (
                    len(out.get(k[:-6], []))
                    if isinstance(out.get(k[:-6]), list)
                    else MISSING
                ),
            )
        else:
            field(k, v, got)
    return pairs


def order_errors(o, schema, expect):
    """Return contract ordering and duplicate-identity violations.

    Args:
        o: Schema-valid output document.
        schema: Output schema used for validation and list ordering.
        expect: Required selected fields, including bounds to check.
    """
    dims = schema["properties"]["dimensions"]["items"]["properties"]["dimension"][
        "enum"
    ]
    groups = schema["properties"]["groups"]["items"]["properties"]["group"]["enum"]
    errors = []

    def check(path, values, key):
        """Record a violation when values differ from their sorted order.

        Args:
            path: File path or selected JSON field path.
            values: Values whose ordering will be checked.
            key: Sort key for the contract order.
        """
        if values != sorted(values, key=key):
            errors.append(path + " wrong order")

    for name, identity, order in [
        ("checks", "id", None),
        ("dimensions", "dimension", dims),
        ("groups", "group", groups),
    ]:
        check(
            name, o[name], lambda v: order.index(v[identity]) if order else v[identity]
        )
        ids = [v[identity] for v in o[name]]
        if len(ids) != len(set(ids)):
            errors.append(name + " duplicate identity")
    if o["scope"] is not None:
        check(
            "scope",
            o["scope"],
            ["conformance", "destructive", "steering", "compare"].index,
        )
    check(
        "stale_markers",
        o["stale_markers"],
        lambda v: (dims.index(v["dimension"]), v["record"]),
    )
    markers = [(v["dimension"], v["record"]) for v in o["stale_markers"]]
    if len(markers) != len(set(markers)):
        errors.append("stale_markers duplicate marker")
    for entry in o["checks"]:
        selected = expect.get("checks", {}).get(entry["id"], {})
        if any(entry.get(k) == "-0.000000" for k in ("lower", "upper", "d_lo", "d_hi") if k in selected):
            errors.append("bounds negative zero")
    for name in ("set_aside", "earlier_pools"):
        check(
            name,
            o[name],
            lambda v: (v["check_id"], [None, "A", "B"].index(v["arm"]), v["condition"])
            + (
                (
                    v["trial_index"],
                    ["wrong_tier", "other_arm", "earlier_pool"].index(v["reason"]),
                )
                if name == "set_aside"
                else ()
            ),
        )
    for name in ("comparison", "anchor"):
        obj = o[name]
        if obj is None:
            continue
        for sub in (
            ("entries", "new") if name == "comparison" else ("checks", "replaced")
        ):
            check(
                name + "." + sub,
                obj[sub],
                lambda v: v["check_id"] if isinstance(v, dict) else v,
            )
        if name == "anchor" and obj["baseline"] is not None:
            check(
                "anchor.baseline.entries",
                obj["baseline"]["entries"],
                lambda v: v["check_id"],
            )
    for path, values in (
        ("comparison.entries", (o.get("comparison") or {}).get("entries", [])),
        ("anchor.replaced", (o.get("anchor") or {}).get("replaced", [])),
        ("anchor.baseline.entries", ((o.get("anchor") or {}).get("baseline") or {}).get("entries", [])),
    ):
        ids = [v["check_id"] for v in values]
        if len(ids) != len(set(ids)):
            errors.append(path + " duplicate identity")
    return errors


def execute(command, data, timeout, *, cwd=None, env=None, output_limit=OUTPUT_LIMIT, binary=False, output_file=None):
    """Run a command with bounded time and captured output.

    Args:
        command: Command string parsed into arguments without a shell.
        data: Raw bytes to parse or send unchanged to standard input.
        timeout: Positive per-invocation time limit in seconds.
    """
    if not math.isfinite(timeout) or timeout <= 0:
        raise Error("timeout must be positive and finite")
    args = getattr(shlex, "sp" + "lit")(command)
    if not args:
        raise Error("empty command")
    started = time.monotonic()
    raw = bytearray()
    output_bytes = 0
    source = data if hasattr(data, "read") else io.BytesIO(data)
    source.seek(0)
    pending = b""
    problem = None
    try:
        with subprocess.Popen(
            args,
            cwd=cwd, env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        ) as proc:
            sel = selectors.DefaultSelector()
            for stream, event in (
                (proc.stdin, selectors.EVENT_WRITE),
                (proc.stdout, selectors.EVENT_READ),
            ):
                os.set_blocking(stream.fileno(), False)
                sel.register(stream, event)
            deadline = time.monotonic() + timeout
            try:
                while sel.get_map():
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        problem = "timeout"
                        break
                    for key, _ in sel.select(min(remaining, 0.1)):
                        if key.fileobj is proc.stdin:
                            if not pending:
                                pending = source.read(65536)
                            try:
                                if pending:
                                    written = os.write(proc.stdin.fileno(), pending)
                                    pending = pending[written:]
                                else:
                                    sel.unregister(proc.stdin)
                                    proc.stdin.close()
                            except BrokenPipeError:
                                sel.unregister(proc.stdin)
                                proc.stdin.close()
                        else:
                            chunk = os.read(proc.stdout.fileno(), 65536)
                            if not chunk:
                                sel.unregister(proc.stdout)
                            output_bytes += len(chunk)
                            allowed = chunk[:max(0, output_limit - (output_bytes - len(chunk)))]
                            if output_file is None:
                                raw.extend(allowed)
                            else:
                                output_file.write(allowed)
                            if output_bytes > output_limit:
                                problem = "output limit"
                                break
                    if problem:
                        break
                if not problem:
                    try:
                        proc.wait(timeout=max(0.001, deadline - time.monotonic()))
                    except subprocess.TimeoutExpired:
                        problem = "timeout"
            finally:
                sel.close()
                proc.poll()
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except (ProcessLookupError, PermissionError):
                    pass
                proc.wait()
            if binary:
                return {"exit_code": proc.returncode, "raw": bytes(raw), "problem": problem, "wall_seconds": time.monotonic() - started}
            try:
                text = raw.decode("utf-8")
            except UnicodeError:
                text = raw.decode("utf-8", errors="replace")
                problem = problem or "invalid UTF-8 output"
            return {"exit_code": proc.returncode, "raw": text, "problem": problem, "wall_seconds": time.monotonic() - started}
    except OSError as exc:
        return {
            "exit_code": None,
            "raw": "",
            "problem": "could not execute: " + str(exc),
            "wall_seconds": time.monotonic() - started,
        }


def assess(run, expect, schema):
    """Validate one run and return its pass/fail reasons.

    Args:
        run: Captured process status, raw output, and execution problem.
        expect: Partial expectation object to validate or compare.
        schema: Output schema used for validation and list ordering.
    """
    reasons = []
    obj = None
    valid = False
    if run["problem"]:
        reasons.append(run["problem"])
    try:
        obj = load(run["raw"])
        jsonschema.Draft202012Validator(schema).validate(obj)
        valid = True
    except (Error, jsonschema.ValidationError, RecursionError) as exc:
        reasons.append("invalid output: " + str(exc)[:300])
    pairs = comparisons(expect, obj)
    if valid:
        reasons += integer_errors(run["raw"], obj, schema)
        reasons += order_errors(obj, schema, expect)
        if run["exit_code"] != obj["exit_code"]:
            reasons.append("process exit differs from exit_code")
    for path, want, got in pairs:
        if want != got:
            reasons.append(
                f"{path}: expected {json.dumps(want)}, got {json.dumps(got)}"
            )
    return reasons


def evaluate(command, data, expect, schema, timeout, *, cwd=None, env=None, run_command=None):
    """Run twice and record validation and repeatability failures per case."""
    validate_expect(expect)
    runs = []
    reasons = []
    for i in range(2):
        started = time.monotonic()
        try:
            run = run_command(command, data, timeout) if run_command is not None else execute(command, data, timeout, cwd=cwd, env=env)
        except Exception as exc:
            run = {"exit_code": None, "raw": "", "problem": "evaluation error: " + str(exc), "wall_seconds": time.monotonic() - started}
        runs.append(run)
        try:
            errors = assess(run, expect, schema)
        except Exception as exc:
            errors = ["evaluation error: " + str(exc)]
        reasons.extend(f"run {i + 1}: {reason}" for reason in errors)
    try:
        equal = load(runs[0]["raw"]) == load(runs[1]["raw"])
    except Error:
        equal = False
    if not equal:
        reasons.append("two runs differ or could not be compared")
    return {"pass": not reasons, "reasons": reasons, "runs": runs}


def file_digest(path):
    """Hash files in bounded chunks; input payloads have a separate ceiling."""
    digest = hashlib.sha256()
    total = 0
    with open(path, "rb") as stream:
        while True:
            chunk = stream.read(65536)
            if not chunk:
                break
            total += len(chunk)
            if total > INPUT_LIMIT:
                raise Error("file size limit: " + str(path))
            digest.update(chunk)
    return digest.hexdigest()


def integer_errors(raw, obj, schema):
    """Walk original tokens and schema paths without rewriting number spellings."""
    decoder = json.JSONDecoder()
    pos = 0
    errors = []

    def variants(node, seen=()):
        if not isinstance(node, dict):
            raise Error("invalid schema node")
        yield node
        ref = node.get("$ref")
        if ref is not None:
            if not isinstance(ref, str) or not ref.startswith("#/") or ref in seen:
                raise Error("unsupported integer schema reference")
            target = schema
            for part in getattr(ref[2:], "sp" + "lit")("/"):
                target = target[part.replace("~1", "/").replace("~0", "~")]
            yield from variants(target, (*seen, ref))
        for name in ("oneOf", "anyOf", "allOf",):
            for child in node.get(name, []):
                yield from variants(child, seen)

    def space():
        nonlocal pos
        while pos < len(raw) and raw[pos] in " \t\r\n":
            pos += 1

    def walk(value, nodes):
        nonlocal pos
        space()
        options = [v for node in nodes for v in variants(node)]
        if isinstance(value, dict):
            if raw[pos] != "{":
                raise Error("object token mismatch")
            pos += 1
            for index, (expected_key, child) in enumerate(value.items()):
                space()
                if index:
                    if raw[pos] != ",": raise Error("object separator")
                    pos += 1
                    space()
                key, pos = decoder.raw_decode(raw, pos)
                if key != expected_key: raise Error("object key mismatch")
                space()
                if raw[pos] != ":": raise Error("object colon")
                pos += 1
                walk(child, [v["properties"][key] for v in options if isinstance(v.get("properties"), dict) and key in v["properties"]])
            space()
            if raw[pos] != "}": raise Error("object end")
            pos += 1
        elif isinstance(value, list):
            if raw[pos] != "[": raise Error("array token mismatch")
            pos += 1
            for index, child in enumerate(value):
                space()
                if index:
                    if raw[pos] != ",": raise Error("array separator")
                    pos += 1
                walk(child, [v["items"] for v in options if "items" in v])
            space()
            if raw[pos] != "]": raise Error("array end")
            pos += 1
        else:
            start = pos
            parsed, pos = decoder.raw_decode(raw, pos)
            if parsed != value: raise Error("value token mismatch")
            integer = any(v.get("type") == "integer" or isinstance(v.get("type"), list) and "integer" in v["type"] for v in options)
            if integer and isinstance(value, (int, float)) and not isinstance(value, bool) and any(c in raw[start:pos] for c in ".eE"):
                errors.append("integer written with a fraction or exponent")
            elif integer and raw[start:pos] == "-0":
                errors.append("integer written as -0")
    try:
        walk(obj, [schema])
        space()
        if pos != len(raw): raise Error("trailing tokens")
    except (Error, ValueError, TypeError, KeyError, IndexError, RecursionError) as exc:
        return ["integer scan could not complete: " + str(exc)]
    return errors


def draft(root):
    """Validate the unsealed schemas and exercise every example location locally."""
    import copy
    from collections import Counter
    from referencing import Registry, Resource

    def refuse_remote(uri):
        raise Error("nonlocal schema reference: " + uri)

    registry = Registry(retrieve=refuse_remote)
    schemas = {}
    files = sorted((root / "schema").iterdir())
    if not files:
        raise Error("no schemas")
    files.append(root.parent / "judge-1.0/schema/judge-input.schema.json")
    for path in files:
        schema = load(read(path))
        jsonschema.Draft202012Validator.check_schema(schema)
        uri = path.resolve().as_uri()
        schemas[path.name] = (schema, uri)
        resource = Resource.from_contents(schema)
        registry = registry.with_resource(uri, resource)
        registry = registry.with_resource(schema.get("$id", uri), resource)

    def check_refs(node, resolver):
        if isinstance(node, dict):
            if "$ref" in node:
                resolver.lookup(node["$ref"])
            for child in node.values():
                check_refs(child, resolver)
        elif isinstance(node, list):
            for child in node:
                check_refs(child, resolver)

    for schema, uri in schemas.values():
        check_refs(schema, registry.resolver(uri))

    def validator(schema, resolver):
        return jsonschema.Draft202012Validator(schema, registry=registry, _resolver=resolver)

    def parts(schema, resolver, value):
        if not isinstance(schema, dict):
            return [(schema, resolver)]
        result = [(schema, resolver)]
        if "$ref" in schema:
            resolved = resolver.lookup(schema["$ref"])
            result += parts(resolved.contents, resolved.resolver, value)
        for key in ("allOf", "anyOf", "oneOf"):
            for branch in schema.get(key, []):
                if key == "allOf" or validator(branch, resolver).is_valid(value):
                    result += parts(branch, resolver, value)
        return result

    def locations(value, constraints, path=()):
        expanded = [part for schema, resolver in constraints for part in parts(schema, resolver, value)]
        yield path, value, expanded, constraints
        children = value.items() if isinstance(value, dict) else enumerate(value) if isinstance(value, list) else []
        for key, child in children:
            selected = []
            for schema, resolver in expanded:
                if not isinstance(schema, dict):
                    continue
                if isinstance(value, dict):
                    properties = schema.get("properties", {})
                    selected.append((properties.get(key, schema.get("additionalProperties", True)), resolver))
                else:
                    prefix = schema.get("prefixItems", [])
                    selected.append((prefix[key] if key < len(prefix) else schema.get("items", True), resolver))
            yield from locations(child, selected, path + (key,))

    def type_allowed(schema, candidate, resolver):
        if not isinstance(schema, dict):
            return bool(schema)
        if "$ref" in schema:
            resolved = resolver.lookup(schema["$ref"])
            if not type_allowed(resolved.contents, candidate, resolved.resolver):
                return False
        for key in ("allOf", "anyOf", "oneOf"):
            if key in schema:
                allowed = [type_allowed(branch, candidate, resolver) for branch in schema[key]]
                if not (all(allowed) if key == "allOf" else any(allowed)):
                    return False
        checker = jsonschema.Draft202012Validator.TYPE_CHECKER
        if "type" in schema:
            types = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]
            return any(checker.is_type(candidate, kind) for kind in types)
        if "const" in schema or "enum" in schema:
            values = schema.get("enum", [schema.get("const")])
            kinds = ("null", "boolean", "object", "array", "string", "number")
            return any(checker.is_type(candidate, kind) and any(checker.is_type(v, kind) for v in values) for kind in kinds)
        return True

    counts = {kind: Counter() for kind in ("unknown_field", "wrong_type", "missing_required")}
    failures = []
    examples = sorted((root / "examples").glob("*.json"))
    if not examples:
        raise Error("no examples")
    for path in examples:
        name = "subject-file.schema.json" if path.name == "subject-file.json" else "record.schema.json"
        schema, uri = schemas[name]
        value = load(read(path))
        checks = [(value, schema, uri, path.name)]
        if path.name.startswith("record-"):
            judge_schema, judge_uri = schemas["judge-input.schema.json"]
            checks.append((value["judge_input"], judge_schema, judge_uri, path.name + "/judge_input"))
        for document, schema, uri, label in checks:
            resolver = registry.resolver(uri)
            check = validator(schema, resolver)
            check.validate(document)
            for location, original, constraints, raw_constraints in locations(document, [(schema, resolver)]):
                proposals = {}
                wrong = next((v for v in (None, False, 0, 0.5, "wrong", [], {})
                              if any(not type_allowed(s, v, r) for s, r in raw_constraints)), ...)
                if wrong is ...:
                    counts["wrong_type"]["skipped: unconstrained type"] += 1
                else:
                    proposals["wrong_type"] = wrong
                if isinstance(original, dict):
                    closed = any(isinstance(s, dict) and s.get("additionalProperties") is False for s, _ in constraints)
                    if closed:
                        unknown = "__draft_unknown__"
                        while unknown in original:
                            unknown += "_"
                        proposals["unknown_field"] = dict(original, **{unknown: None})
                    else:
                        counts["unknown_field"]["skipped: open object/map"] += 1
                    required = sorted({k for s, _ in constraints if isinstance(s, dict) for k in s.get("required", [])})
                    if required:
                        proposals["missing_required"] = {k: v for k, v in original.items() if k != required[0]}
                    else:
                        counts["missing_required"]["skipped: no required fields"] += 1
                else:
                    for kind in ("unknown_field", "missing_required"):
                        counts[kind]["skipped: not an object"] += 1
                for kind, replacement in proposals.items():
                    mutant = copy.deepcopy(document)
                    if location:
                        parent = mutant
                        for key in location[:-1]:
                            parent = parent[key]
                        parent[location[-1]] = replacement
                    else:
                        mutant = replacement
                    counts[kind]["made"] += 1
                    if not check.is_valid(mutant):
                        counts[kind]["rejected"] += 1
                    else:
                        failures.append(f"accepted {kind} mutant: {label} {location}")
    for kind, count in counts.items():
        print(f"{kind}: made={count['made']} rejected={count['rejected']}; " + "; ".join(f"{why}={n}" for why, n in sorted(count.items()) if why.startswith("skipped:")))
    for failure in failures:
        print(failure, file=sys.stderr)
    return int(bool(failures))
