#!/usr/bin/env python3
"""Prove every grader can fail before any of them is allowed to pass.

A grader that returns True on an input where it must return False is decorative: it measures
the scaffold, a naming convention, or nothing at all. Each grader below gets a positive fixture
it must accept and a negative fixture it must reject. No model calls, no cost.
"""
import importlib.util, os, sys, tempfile

spec = importlib.util.spec_from_file_location("ev", os.path.join(os.path.dirname(os.path.abspath(__file__)), "run_evals.py"))
ev = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ev)

PLAN = "docs/testing/test-plan.md"

def workspace(plan_text, files=None):
    ws = tempfile.mkdtemp(prefix="selftest-")
    os.makedirs(os.path.join(ws, os.path.dirname(PLAN)), exist_ok=True)
    with open(os.path.join(ws, PLAN), "w", encoding="utf-8") as fh:
        fh.write(plan_text)
    for rel in files or []:
        p = os.path.join(ws, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        open(p, "w").close()
    return ws

HEADER = ("| Id | Finding | Severity | Data safe? | Evidence id | Status | Verdict by | Reason | Fingerprint |\n"
          "|---|---|---|---|---|---|---|---|---|\n")
LEDGER_HEADER = ("| Id | Claim | Executed | Inputs | Observed | Mutation | Reproduction | Label (`observado` / `razonado`, literal) |\n"
                 "|---|---|---|---|---|---|---|---|\n")

def plan(findings="", ledger="", extra=""):
    return (f"# Test plan\n\n## Ranked targets\n\n| Target | Status |\n|---|---|\n| a | pending |\n\n"
            f"## Execution log\n\n| Date | Target |\n|---|---|\n\n"
            f"## Findings\n\n{HEADER}{findings}\n"
            f"## Evidence ledger\n\nOne row per `observado` conclusion.\n\n{LEDGER_HEADER}{ledger}\n{extra}")

CASES = [
    # (name, grader, positive workspace, negative workspace)
    ("findings_have_evidence: placeholder row is not a finding",
     {"type": "findings_have_evidence", "path": PLAN},
     plan(findings="| (none) | | | | | | | | |\n"),
     plan(findings="| F1 | leak | M | yes | | open | me | - | - |\n", ledger="| E1 | c | cmd | i | o | m | r | observado |\n")),

    ("findings_have_evidence: cited id must exist in the ledger",
     {"type": "findings_have_evidence", "path": PLAN},
     plan(findings="| F1 | leak | M | yes | E1 | open | me | - | - |\n", ledger="| E1 | c | cmd | i | o | m | r | observado |\n"),
     plan(findings="| F1 | leak | M | yes | E9 | open | me | - | - |\n", ledger="| E1 | c | cmd | i | o | m | r | observado |\n")),

    ("file_regex observado: must not match the template header alone",
     {"type": "file_regex", "path": PLAN, "section": "Evidence ledger", "pattern": r"^\|.*\|\s*observado\s*\|?\s*$"},
     plan(ledger="| E1 | c | cmd | i | o | m | r | observado |\n"),
     plan(ledger="")),

    ("section_rows: an empty table has zero rows",
     {"type": "section_rows", "path": PLAN, "section": "Findings", "max": 0},
     plan(findings=""),
     plan(findings="| F1 | leak | M | yes | E1 | open | me | - | - |\n")),

    ("no_razonado_in_findings",
     {"type": "no_razonado_in_findings", "path": PLAN},
     plan(findings="| F1 | leak | M | yes | E1 | open | me | - | - |\n"),
     plan(findings="| F1 | leak | M | yes | E1 | razonado | me | - | - |\n")),

    ("ranked_rows_equal",
     {"type": "ranked_rows_equal", "path": PLAN, "count": 1},
     plan(),
     plan().replace("| a | pending |\n", "| a | pending |\n| b | pending |\n")),
]

def main():
    failures = []
    for name, g, pos, neg in CASES:
        ok_pos, d_pos = ev.grade(g, workspace(pos), {"output": "", "tool_inputs": []})
        ok_neg, d_neg = ev.grade(g, workspace(neg), {"output": "", "tool_inputs": []})
        if not ok_pos:
            failures.append(f"{name}: rejected its POSITIVE fixture ({d_pos})")
        if ok_neg:
            failures.append(f"{name}: ACCEPTED its negative fixture ({d_neg}) — this grader cannot fail, so its passes mean nothing")
        status = "ok" if (ok_pos and not ok_neg) else "BROKEN"
        print(f"  [{status}] {name}")

    # file_exists / file_not_exists on a real path pair
    ws = workspace(plan(), files=["tests/a.test.js"])
    checks = [
        ({"type": "file_exists", "path": "tests/a.test.js"}, True),
        ({"type": "file_exists", "path": "tests/missing.js"}, False),
        ({"type": "dir_file_count", "path": "tests", "count": 1}, True),
        ({"type": "dir_file_count", "path": "tests", "count": 2}, False),
    ]
    for g, expected in checks:
        got, det = ev.grade(g, ws, {"output": "", "tool_inputs": []})
        label = ev.grader_name(g)
        if got != expected:
            failures.append(f"{label}: expected {expected}, got {got} ({det})")
        print(f"  [{'ok' if got == expected else 'BROKEN'}] {label} -> {expected}")

    print()
    if failures:
        print(f"{len(failures)} grader(s) cannot be trusted:")
        for f in failures:
            print("  -", f)
        return 1
    print("every grader accepted its positive fixture and rejected its negative one")
    return 0

if __name__ == "__main__":
    sys.exit(main())
