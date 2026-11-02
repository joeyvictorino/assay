package report

const FindingsJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/joeyvictorino/assay/docs/findings-schema.json",
  "title": "Assay Findings",
  "description": "Schema for findings.redacted.json: the per-run list of security findings after validation and reconciliation.",
  "type": "array",
  "items": { "$ref": "#/$defs/finding" },
  "$defs": {
    "finding": {
      "type": "object",
      "required": ["id", "dedup_key", "run_id", "lab", "class", "location", "severity", "state", "summary", "model", "reconcile", "first_seen", "last_seen"],
      "properties": {
        "id": { "type": "string", "description": "Unique finding identifier (f- prefix + hash)." },
        "dedup_key": { "type": "string", "description": "Deduplication key: SHA-256 of (class, method, path_template, lab)." },
        "run_id": { "type": "string", "description": "Run that produced this finding." },
        "lab": { "type": "string", "description": "Lab the finding was reported against." },
        "class": {
          "type": "string",
          "enum": [
            "sqli", "xss-reflected", "xss-stored", "idor", "auth-missing",
            "auth-bypass", "default-creds", "open-redirect", "path-traversal",
            "ssrf", "info-disclosure", "security-headers", "cors-misconfig",
            "mass-assignment", "jwt-weak", "rate-limit-missing", "verbose-error",
            "sensitive-file", "csrf", "other"
          ],
          "description": "Vulnerability class from the closed taxonomy."
        },
        "cwe": { "type": "string", "description": "CWE identifier when known." },
        "location": { "$ref": "#/$defs/location" },
        "severity": {
          "type": "string",
          "enum": ["info", "low", "medium", "high", "critical"],
          "description": "Finding severity."
        },
        "state": {
          "type": "string",
          "enum": ["theorized", "validated", "refuted", "declined"],
          "description": "Finding state after deterministic validation."
        },
        "summary": { "type": "string", "maxLength": 280, "description": "Redacted one-line summary." },
        "evidence": {
          "type": "array",
          "items": { "$ref": "#/$defs/evidence" },
          "description": "Control-plane evidence supporting the finding."
        },
        "model": { "$ref": "#/$defs/model_ref" },
        "prompt_hash": { "type": "string", "description": "SHA-256 of the system prompt used." },
        "tool_call_ids": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Tool call IDs the model claimed produced this finding."
        },
        "control_plane": {
          "type": "array",
          "items": { "$ref": "#/$defs/audit_ref" },
          "description": "Audit-log references for control-plane verification."
        },
        "reconcile": { "$ref": "#/$defs/reconcile_status" },
        "validation": { "$ref": "#/$defs/validation_result" },
        "remediation": { "$ref": "#/$defs/remediation" },
        "first_seen": { "type": "string", "format": "date-time" },
        "last_seen": { "type": "string", "format": "date-time" }
      },
      "additionalProperties": false
    },
    "location": {
      "type": "object",
      "required": ["method", "path_template"],
      "properties": {
        "method": { "type": "string", "description": "HTTP method." },
        "path_template": { "type": "string", "description": "Normalized URL path." },
        "param": { "type": "string", "description": "Vulnerable parameter name." },
        "component": { "type": "string", "description": "Application component." }
      },
      "additionalProperties": false
    },
    "model_ref": {
      "type": "object",
      "required": ["provider", "model"],
      "properties": {
        "provider": { "type": "string" },
        "model": { "type": "string" },
        "agent": { "type": "string" }
      },
      "additionalProperties": false
    },
    "audit_ref": {
      "type": "object",
      "required": ["run_id", "seq"],
      "properties": {
        "run_id": { "type": "string" },
        "seq": { "type": "integer", "minimum": 0 }
      },
      "additionalProperties": false
    },
    "evidence": {
      "type": "object",
      "required": ["kind"],
      "properties": {
        "kind": {
          "type": "string",
          "enum": ["http-exchange", "marker-reflected", "header-missing", "auth-bypass", "tool-output"],
          "description": "Type of evidence."
        },
        "tool_call_id": { "type": "string" },
        "audit_seq": { "type": "integer", "minimum": 0 },
        "request_digest": { "type": "string" },
        "response_digest": { "type": "string" },
        "status_code": { "type": "integer" },
        "marker": { "type": "string" }
      },
      "additionalProperties": false
    },
    "reconcile_status": {
      "type": "object",
      "required": ["checked", "match"],
      "properties": {
        "checked": { "type": "boolean", "description": "Whether reconciliation ran for this finding." },
        "match": { "type": "boolean", "description": "Whether claimed tool calls matched control-plane records." },
        "reasons": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Mismatch reasons when match is false."
        }
      },
      "additionalProperties": false
    },
    "validation_result": {
      "type": "object",
      "required": ["state", "checker", "checked_at"],
      "properties": {
        "state": {
          "type": "string",
          "enum": ["theorized", "validated", "refuted", "declined"],
          "description": "Post-validation state."
        },
        "checker": { "type": "string", "description": "Checker that validated or refuted." },
        "evidence": {
          "type": "array",
          "items": { "$ref": "#/$defs/evidence" }
        },
        "note": { "type": "string" },
        "checked_at": { "type": "string", "format": "date-time" }
      },
      "additionalProperties": false
    },
    "remediation": {
      "type": "object",
      "required": ["verified", "draft"],
      "properties": {
        "code_diff": { "type": "string", "description": "Unified diff (synthetic lab only)." },
        "config_change": { "type": "string", "description": "Human-readable configuration step." },
        "virtual_patch": { "type": "string", "description": "WAF rule text." },
        "verified": { "type": "boolean", "description": "Whether the remediation was rebuilt and re-checked." },
        "draft": { "type": "boolean", "description": "Whether this is model-drafted text, not verified." }
      },
      "additionalProperties": false
    }
  }
}`
