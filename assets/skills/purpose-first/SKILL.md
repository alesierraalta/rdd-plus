---
name: purpose-first
description: "Trigger: 'explícame', 'resúmeme', 'no entiendo', 'qué significa esto', explain plainly, high-level summary. Answer with the purpose and the consequence before any mechanism, translate every identifier, and leave the depth to explicacion-interactiva."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
---

## Activation Contract

Apply when the operator asks what something is, what it means, what changed, or says they do not
understand — a defect, a diff, a decision, a log line, a review finding. This is the short lane:
purpose first, mechanism last, and only the mechanism the operator needs in order to decide or to stop
worrying. For an exhaustive, interactive or document-grade explanation, hand off to
`explicacion-interactiva` or `explainer-docs` instead of expanding here.

## Hard Rules

- The first sentence says what the thing is **for** and why it matters, and it carries no identifier.
  If the first sentence needs a file name to make sense, it is not a first sentence.
- No file, flag, function, field or error code reaches the operator without its translation in the same
  breath: what it touches and what breaks without it. Parentheses after the plain sentence, never
  before it.
- Jargon is a defect, not a shortcut. "The message embeds the ledger's Run cell without the sanitizer"
  is jargon; "a file in the repository can make the assistant say whatever it wants" is the point.
- Always say what happens if nobody touches it, and what changes once it is fixed. A consequence the
  operator can picture beats an accurate mechanism they cannot.
- Severity is translated, never repeated: "critical" becomes what a user would experience.
- A few short paragraphs, one idea each. Depth is offered, never dumped: close with one line offering
  the detail, and give it only if it is asked for.

## Decision Gates

| The operator said | Answer with |
|---|---|
| "explícame" / "resúmeme" / "no entiendo" | purpose, consequence, then one line of mechanism |
| "¿qué significa esto?" about a finding or a review | what it lets happen, what it costs, what the fix changes |
| "¿qué cambió?" about work just finished | the behaviour that changed for the user, not the files that moved |
| "explícame a fondo" / "al milímetro" | hand off to `explicacion-interactiva` |
| a decision is pending | the fork in plain words: what each option gets and what it costs |

## Execution Steps

1. Write the point in one sentence with no identifiers in it.
2. Add the consequence: what breaks today, or what stops being a risk.
3. Add the mechanism only if it changes the decision, translating each term as it appears.
4. Check the draft against the operator's own words: could someone who never read the code repeat this
   after one read?
5. Stop there, and offer the detail in one closing line.

## Output Contract

Three or four short paragraphs at most, in this order: what it is about, why it matters, what happens
if it is left alone, and — only when it earns its place — how it works. Every technical term carries
its meaning beside it. The last line offers the deep explanation; it never delivers it unasked.

The failure this prevents, from a real report: *"la inyección con quote, con un test de celda
hostil"* — true, precise, and unreadable for the person who has to decide. The same fact at the
operator's altitude: *"Un archivo del repositorio puede hacer que el asistente diga lo que ese archivo
quiera. Es un agujero de seguridad, y se cierra saneando ese texto y probándolo con un archivo
malicioso."*
