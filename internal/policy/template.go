package policy

const templateMinimal = `# Minimal assay policy
version: "1"
default_effect: deny

agents:
  default:
    max_tier: high
    allowed_tools: []
    max_delegation_depth: 0
    budget:
      max_usd: 1.0
      max_latency_ms: 30000

rules:
  - id: allow-all-reads
    effect: allow
    match:
      capabilities: [http.read]
    rationale: "Allow all read-only operations"
`

const templateStandard = `# Standard assay policy — three agent tiers
version: "1"
default_effect: deny

agents:
  recon:
    max_tier: medium
    allowed_tools: [http_get, dns_lookup]
    max_delegation_depth: 1
    budget:
      max_usd: 0.5
      max_latency_ms: 30000
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post, http_put]
    max_delegation_depth: 0
    budget:
      max_usd: 2.0
      max_latency_ms: 60000
  validate:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget:
      max_usd: 1.0
      max_latency_ms: 30000

rules:
  - id: allow-recon-read
    effect: allow
    match:
      agents: [recon]
      capabilities: [http.read]
    rationale: "Recon agents may read freely within their tier"
  - id: allow-probe-write
    effect: allow
    match:
      agents: [probe]
      capabilities: [http.read, http.write]
    rationale: "Probe agents need read and write for exploitation"
  - id: deny-recon-write
    effect: deny
    match:
      agents: [recon]
      capabilities: [http.write]
    rationale: "Recon must not write — discovery only"
  - id: allow-validate-read
    effect: allow
    match:
      agents: [validate]
      capabilities: [http.read, http.write]
    rationale: "Validators confirm findings with targeted requests"

dangerous_combinations:
  - [http_post, exec_shell]
`

const templateStrict = `# Strict assay policy — tight budgets, explicit denies
version: "1"
default_effect: deny

agents:
  recon:
    max_tier: low
    allowed_tools: [http_get, dns_lookup]
    max_delegation_depth: 0
    budget:
      max_usd: 0.25
      max_latency_ms: 15000
      max_tokens: 50000
  probe:
    max_tier: medium
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget:
      max_usd: 1.0
      max_latency_ms: 30000
      max_tokens: 100000
  validate:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget:
      max_usd: 0.5
      max_latency_ms: 15000
      max_tokens: 50000

rules:
  - id: allow-recon-read
    effect: allow
    match:
      agents: [recon]
      capabilities: [http.read]
    rationale: "Minimal read access for discovery"
  - id: deny-recon-write
    effect: deny
    match:
      agents: [recon]
      capabilities: [http.write, auth.login]
    rationale: "Recon must never write or authenticate"
  - id: allow-probe-targeted
    effect: allow
    match:
      agents: [probe]
      capabilities: [http.read, http.write]
    rationale: "Probe may read and write within budget"
  - id: deny-probe-auth
    effect: deny
    match:
      agents: [probe]
      capabilities: [auth.login]
    rationale: "No credential brute-forcing"
  - id: allow-validate-read
    effect: allow
    match:
      agents: [validate]
      capabilities: [http.read]
    rationale: "Validators confirm with targeted reads"
  - id: deny-delegation
    effect: deny
    match:
      capabilities: [delegate]
    rationale: "No agent may delegate in strict mode"

dangerous_combinations:
  - [http_post, exec_shell]
  - [auth.login, report]
  - [http_write, delegate]

weights:
  sqli: 1.5
  xss-reflected: 1.2
  auth-bypass: 1.5
  idor: 1.0
`

// GenerateTemplate returns a starter policy YAML for the given preset.
func GenerateTemplate(preset string) (string, error) {
	switch preset {
	case "minimal":
		return templateMinimal, nil
	case "standard":
		return templateStandard, nil
	case "strict":
		return templateStrict, nil
	default:
		return "", ErrInvalidPolicy
	}
}

// TemplatePresets returns the available preset names.
func TemplatePresets() []string {
	return []string{"minimal", "standard", "strict"}
}
