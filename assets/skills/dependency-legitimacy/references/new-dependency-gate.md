# The gate — run in order, stop at the first failure

Why this exists in this form: LLMs fabricate package names at rates measured around 5%
for commercial models and ~20%+ for open-source ones, and roughly **43% of fabricated
names recur across identical prompts**. Reproducible hallucinations are registrable by an
attacker, which is what turned this from a nuisance into a live supply-chain vector with
confirmed incidents. The gate assumes the name is wrong until the registry says otherwise.

## G0 — Do we need it at all?

Can the standard library do it? Does a dependency already in the tree do it? Is it twenty
lines we would understand and own? A dependency is permanent, transitive and someone
else's security posture. Reject here whenever you honestly can.

## G1 — Is this the CANONICAL name?

Open the library's own current documentation and copy the install command from there.
Not from memory, not from a model, not from a blog. Then compare character by character.

Red shapes: the name is a shortened or "obvious" form of the real one
(`unused-imports` vs `eslint-plugin-unused-imports` — a documented real case); it differs
by one character from a very popular package; it uses a different separator (`-` vs `_`,
scoped vs unscoped); the org/scope is not the one the project actually publishes under.

## G2 — Does it exist, and what is its history?

Query the registry metadata WITHOUT installing:

```bash
npm view <pkg> time versions maintainers repository dist.tarball
pip index versions <pkg>            # or the JSON API: https://pypi.org/pypi/<pkg>/json
```

Read: creation date · number of versions · date of the last publish · maintainer count ·
download volume · description quality. **Red flags**: created days or weeks ago · a single
version · no repository field · a generic or machine-written description · a download
count that does not match how "well known" the name felt to you · a maintainer with no
other packages.

A name that feels familiar plus a package created last month is the exact signature of a
squat on a hallucination.

## G3 — Cross-registry check

Look the same name up in the other ecosystems. A meaningful share of names hallucinated
for Python exist on npm and vice versa, which lets an attacker plant a payload where a
developer of the other language will reach for it. A name that exists in both places with
unrelated content is a strong signal to stop.

## G4 — Does the linked repository match?

Follow the repository field. The repo must exist, correspond to the package, and be the
same one the official docs point to. Check activity, issues, release history. A package
pointing at a popular repo it has nothing to do with is a common disguise.

## G5 — Provenance and attestation

Prefer packages published through trusted publishing with signed provenance — a
cryptographic link between the published artifact, the source commit and the CI workflow
that built it (Sigstore-backed on both npm and PyPI; PyPI attestations now cover a large
and growing share of packages).

```bash
npm audit signatures
```

Note that this is NOT enforced at install time by the package manager: absence of
provenance is not proof of anything, but its presence is real evidence, and for a package
you are unsure about it should tip the decision.

## G6 — Known vulnerabilities

```bash
osv-scanner scan source -r .        # any ecosystem
pip-audit                            # Python; queries OSV/PyPA/GitHub advisories
npm audit --omit=dev
```

Run it on the whole resulting tree, not just the direct package.

## G7 — License

Verify the license and that it matches the project's policy, including transitively.
Copyleft arriving through a transitive dependency is a real and expensive surprise.

## G8 — Install safely

```bash
npm install --ignore-scripts <pkg>@<exact-version>
pip install --no-deps <pkg>==<exact-version>    # then review what it needs
```

Then: pin the exact version, commit the lockfile, and re-run the scan on the resulting
tree. Never leave a floating range on a package that made it through this gate under any
doubt.

## Verdict

Write one of three, with the evidence: **legítimo** (every gate passed, cite G2 and G4)
· **sospechoso** (a red flag; name it and do not install) · **no verificable** (registry
or docs unavailable; do not install, propose the alternative). "Probably fine" is not a
verdict.
