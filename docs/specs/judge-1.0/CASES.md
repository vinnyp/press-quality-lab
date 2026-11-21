Each case directory contains input.json and case.json. The latter supplies id, rules, side and partial expect fields.
Run: uv run runner.py --judge "<command>" --cases cases
A stray file makes the runner refuse. Use PYTHONDONTWRITEBYTECODE=1.
Each case runs twice. A successful run requires matching documents, selected fields, list order and process codes.
