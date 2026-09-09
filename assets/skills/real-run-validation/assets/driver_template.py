"""Real-run validation driver (template).

NOT a unit test. A throwaway script that exercises the REAL built artifact
with REAL inputs and PRINTS observable behavior, so a human/agent can confirm
it actually works. Keep this in a scratchpad/temp dir — never in the repo diff.

Adapt: swap the import, the real input, and the happy/failure calls.
"""
from __future__ import annotations

# 1) Import the REAL module under test (run with the right venv / PYTHONPATH).
# from my_pkg.some_module import Thing, ThingError

REAL_INPUT = b"a realistic, representative payload — not foo/bar"


def line(title: str) -> None:
    print("\n" + "=" * 64 + f"\n{title}\n" + "=" * 64)


def main() -> None:
    # thing = Thing(...)  # construct with realistic config

    line("HAPPY PATH — expected effect / exact round-trip")
    # result = thing.do(REAL_INPUT)
    # print(f"input   : {REAL_INPUT!r}")
    # print(f"output  : {result!r}")
    # print(f"matches?: {check(result)}")

    line("FAILURE PATH — must fail as intended, not silently")
    # try:
    #     thing.do(corrupt(REAL_INPUT))
    #     print("!!! BUG: bad input was accepted !!!")
    # except ThingError as exc:
    #     print(f"rejected -> {type(exc).__name__}: {exc}")

    print("\nCompare every observed line above against the intended contract.")


if __name__ == "__main__":
    main()
