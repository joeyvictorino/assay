package report

const RunSchemaVersion = "1.0.0"

const RunJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/joeyvictorino/assay/blob/main/docs/run-schema.json",
  "title": "Assay Run Report",
  "description": "Output written to run.json after each assay run. Documents models, labs, findings, costs, timing, overlap, attack paths, reflection scores and the overall verdict.",
  "type": "object",
  "required": ["run_id", "git_sha", "mode", "started", "finished", "scope", "models", "labs", "audit_head_hash", "audit_record_count", "budget_cap_usd", "spent_usd", "truncated", "verdict", "exit_code"],
  "properties": {
    "run_id": {
      "type": "string",
      "description": "Unique identifier for this run."
    },
    "git_sha": {
      "type": "string",
      "description": "Git commit SHA of the assay harness at run time."
    },
    "ci_run_url": {
      "type": "string",
      "description": "URL of the CI run that produced this report, if any."
    },
    "mode": {
      "type": "string",
      "enum": ["full", "degraded"],
      "description": "Whether every model completed (full) or some failed over (degraded)."
    },
    "config": {
      "type": "string",
      "description": "Name of the run configuration."
    },
    "label": {
      "type": "string",
      "description": "Human-readable label describing the run variant."
    },
    "started": {
      "type": "string",
      "format": "date-time",
      "description": "ISO-8601 UTC timestamp when the run began."
    },
    "finished": {
      "type": "string",
      "format": "date-time",
      "description": "ISO-8601 UTC timestamp when the run ended."
    },
    "scope": { "$ref": "#/$defs/decision" },
    "models": {
      "type": "array",
      "items": { "$ref": "#/$defs/model_result" },
      "description": "Per-model summary of findings, usage and latency."
    },
    "labs": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Lab names exercised during the run."
    },
    "overlap": {
      "oneOf": [{ "$ref": "#/$defs/overlap_matrix" }, { "type": "null" }],
      "description": "Cross-model finding overlap matrix, absent when only one model ran."
    },
    "precision": {
      "type": "object",
      "additionalProperties": { "$ref": "#/$defs/prf" },
      "description": "Per-model precision/recall against ground-truth labs."
    },
    "chains": {
      "type": "array",
      "items": { "$ref": "#/$defs/attack_path" },
      "description": "Reconstructed attack paths linking findings into chains."
    },
    "reflection": {
      "type": "array",
      "items": { "$ref": "#/$defs/reflection_result" },
      "description": "Self- and peer-reflection scores per lab and model."
    },
    "reconcile_reason_counts": {
      "type": "object",
      "additionalProperties": { "type": "integer" },
      "description": "Counts of reconciliation reasons across all findings."
    },
    "policy_denies_by_reason": {
      "type": "object",
      "additionalProperties": { "type": "integer" },
      "description": "Counts of policy denials grouped by reason code."
    },
    "audit_head_hash": {
      "type": "string",
      "description": "SHA-256 of the final audit-log record (hash chain head)."
    },
    "audit_record_count": {
      "type": "integer",
      "minimum": 0,
      "description": "Total records written to the audit log during the run."
    },
    "budget_cap_usd": {
      "type": "number",
      "minimum": 0,
      "description": "Maximum budget in USD that the run was allowed to spend."
    },
    "spent_usd": {
      "type": "number",
      "minimum": 0,
      "description": "Actual spend in USD across all models."
    },
    "truncated": {
      "type": "boolean",
      "description": "True if the run hit a budget or time limit before completing."
    },
    "weighted_score": {
      "type": "number",
      "description": "Weighted ensemble score across all models (ADR 0020)."
    },
    "verdict": {
      "type": "string",
      "enum": ["PASS", "BLOCKED", "ERROR"],
      "description": "Overall result: PASS when no blocking findings, BLOCKED when at least one, ERROR on infrastructure failure."
    },
    "exit_code": {
      "type": "integer",
      "enum": [0, 1, 2],
      "description": "0 = pass, 1 = blocked, 2 = error."
    }
  },
  "$defs": {
    "model_ref": {
      "type": "object",
      "required": ["provider", "model", "agent"],
      "properties": {
        "provider": { "type": "string", "description": "API provider (e.g. anthropic, openai)." },
        "model": { "type": "string", "description": "Model identifier." },
        "agent": { "type": "string", "description": "Agent profile name that ran this model." }
      }
    },
    "usage": {
      "type": "object",
      "required": ["input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "cost_usd", "latency_ms"],
      "properties": {
        "input_tokens": { "type": "integer", "minimum": 0 },
        "output_tokens": { "type": "integer", "minimum": 0 },
        "cache_read_tokens": { "type": "integer", "minimum": 0 },
        "cache_write_tokens": { "type": "integer", "minimum": 0 },
        "cost_usd": { "type": "number", "minimum": 0 },
        "latency_ms": { "type": "integer", "minimum": 0 }
      }
    },
    "model_result": {
      "type": "object",
      "required": ["ref", "findings_by_state", "usage", "p50_ms", "p95_ms", "refusals", "failovers"],
      "properties": {
        "ref": { "$ref": "#/$defs/model_ref" },
        "findings_by_state": {
          "type": "object",
          "additionalProperties": { "type": "integer" },
          "description": "Counts of findings grouped by state (validated, theorized, refuted, declined)."
        },
        "usage": { "$ref": "#/$defs/usage" },
        "p50_ms": { "type": "integer", "minimum": 0, "description": "Median latency in milliseconds." },
        "p95_ms": { "type": "integer", "minimum": 0, "description": "95th percentile latency in milliseconds." },
        "refusals": { "type": "integer", "minimum": 0, "description": "Number of model refusals." },
        "failovers": { "type": "integer", "minimum": 0, "description": "Number of failover attempts." }
      }
    },
    "decision": {
      "type": "object",
      "required": ["effect", "reason", "rationale"],
      "properties": {
        "effect": { "type": "string", "enum": ["allow", "deny"] },
        "reason": { "type": "string", "description": "Machine-readable reason code." },
        "rationale": { "type": "string", "description": "Human-readable explanation." },
        "matched_rules": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Policy rule IDs that matched."
        },
        "policy_hash": { "type": "string", "description": "SHA-256 of the policy document." }
      }
    },
    "prf": {
      "type": "object",
      "required": ["tp", "fp", "fn", "precision", "recall", "f1"],
      "properties": {
        "tp": { "type": "integer", "minimum": 0 },
        "fp": { "type": "integer", "minimum": 0 },
        "fn": { "type": "integer", "minimum": 0 },
        "precision": { "type": "number", "minimum": 0, "maximum": 1 },
        "recall": { "type": "number", "minimum": 0, "maximum": 1 },
        "f1": { "type": "number", "minimum": 0, "maximum": 1 }
      }
    },
    "overlap_matrix": {
      "type": "object",
      "required": ["models", "keys", "membership", "pairwise", "unique_per_model", "union", "intersection", "inputs"],
      "properties": {
        "models": { "type": "array", "items": { "type": "string" } },
        "keys": { "type": "array", "items": { "type": "string" } },
        "membership": {
          "type": "object",
          "additionalProperties": { "type": "array", "items": { "type": "integer" } }
        },
        "pairwise": {
          "type": "array",
          "items": {
            "type": "array",
            "items": { "$ref": "#/$defs/pair_cell" }
          }
        },
        "unique_per_model": {
          "type": "object",
          "additionalProperties": { "type": "array", "items": { "type": "string" } }
        },
        "union": { "type": "integer", "minimum": 0 },
        "intersection": { "type": "integer", "minimum": 0 },
        "inputs": { "type": "array", "items": { "$ref": "#/$defs/file_digest" } }
      }
    },
    "pair_cell": {
      "type": "object",
      "required": ["both", "only_a", "only_b", "jaccard"],
      "properties": {
        "both": { "type": "integer", "minimum": 0 },
        "only_a": { "type": "integer", "minimum": 0 },
        "only_b": { "type": "integer", "minimum": 0 },
        "jaccard": { "type": "number", "minimum": 0, "maximum": 1 }
      }
    },
    "file_digest": {
      "type": "object",
      "required": ["name", "sha256", "bytes"],
      "properties": {
        "name": { "type": "string" },
        "sha256": { "type": "string" },
        "bytes": { "type": "integer", "minimum": 0 }
      }
    },
    "path_step": {
      "type": "object",
      "required": ["type", "id"],
      "properties": {
        "type": { "type": "string", "enum": ["finding", "precondition", "fact", "objective"] },
        "id": { "type": "string" },
        "note": { "type": "string" }
      }
    },
    "attack_path": {
      "type": "object",
      "required": ["id", "lab", "objective", "path", "confidence", "source_refs"],
      "properties": {
        "id": { "type": "string" },
        "lab": { "type": "string" },
        "objective": { "type": "string", "enum": ["account-takeover", "data-access", "privilege-escalation", "code-exec"] },
        "path": { "type": "array", "items": { "$ref": "#/$defs/path_step" } },
        "confidence": { "type": "string", "enum": ["validated", "theorized", "refuted", "declined"] },
        "source_refs": { "type": "array", "items": { "type": "string" } }
      }
    },
    "reflection_result": {
      "type": "object",
      "required": ["lab", "model", "scored", "revisions", "passed", "validated", "refuted", "theorized"],
      "properties": {
        "lab": { "type": "string" },
        "model": { "type": "string" },
        "reviewer": { "type": "string", "description": "Model that scored the report." },
        "reviewer_kind": { "type": "string", "enum": ["self", "peer"] },
        "prompt_variant": { "type": "string", "description": "Prompt variant name (ADR 0020)." },
        "scored": { "type": "boolean" },
        "scores": { "type": "array", "items": { "type": "integer", "minimum": 1, "maximum": 10 } },
        "revisions": { "type": "integer", "minimum": 0 },
        "final_score": { "type": "integer", "minimum": 0, "maximum": 10 },
        "passed": { "type": "boolean" },
        "validated": { "type": "integer", "minimum": 0 },
        "refuted": { "type": "integer", "minimum": 0 },
        "theorized": { "type": "integer", "minimum": 0 },
        "note": { "type": "string" }
      }
    }
  }
}`
