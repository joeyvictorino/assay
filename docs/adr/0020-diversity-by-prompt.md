# ADR 0020: Diversity by Prompt

## Status

Accepted.

## Context

When multiple models assess the same target, giving each the same system
prompt produces correlated coverage: models that share architectural
tendencies miss the same classes of vulnerability. Team-solve workflows
(memdoor) achieve better collective coverage by varying the instructions
each participant receives, so independently-prompted assessors explore
different parts of the attack surface.

## Decision

Run configurations may declare named **prompt variants** under a new
`prompts:` map. Each model entry may reference one by name via a
`prompt:` field. When a model has a variant, the probe agent uses that
text as its system prompt instead of the hardcoded default. Models
without a variant keep the default prompt.

The variant name is recorded in `ReflectionResult.PromptVariant` and
printed in the run report's self-reflection table so a reader can
correlate coverage differences with prompt differences.

### YAML shape

```yaml
prompts:
  auth-focus: |
    You are assessing ... Focus on authentication, session management
    and access-control weaknesses. ...
  injection-focus: |
    You are assessing ... Focus on injection attacks: SQL injection,
    XSS, OS command injection. ...

models:
  - { provider: anthropic, model: claude-sonnet-5-5, prompt: auth-focus }
  - { provider: llama, model: local-8b, prompt: injection-focus }
  - { provider: local-mistral, model: ministral-3b, prompt: auth-focus }
```

### Validation

- A model that names a `prompt:` key that does not exist in `prompts:`
  fails config validation.
- A model that names a `prompt:` when no `prompts:` map exists fails
  config validation.
- `prompts:` without any model referencing it is legal (unused variants
  are harmless).
- An empty `prompt:` field means "use the default"; it is the same as
  omitting it.

### Overlap implications

Two models with the same prompt variant are still independently scored
and appear in the overlap matrix. Prompt diversity improves the union
but does not change how overlap is computed (ADR 0007).

## Consequences

- Ensemble runs can use prompt diversity without code changes; only the
  YAML changes.
- The self-reflection table gains a Prompt column so reviewers can
  see which variant each model used.
- The default system prompt is still the fallback; single-model and
  legacy configs are unaffected.
