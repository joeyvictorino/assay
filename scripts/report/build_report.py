#!/usr/bin/env python3
"""Render an assay results directory as a four-page A4 PDF.

Usage:
    build_report.py RESULTS_DIR [--out FILE] [--sample]

Reads run.json (required), findings.redacted.json, overlap.json,
reconcile.json, costs.json and ci.json (all optional) and renders the same
sections, in the same order, as internal/report.Markdown:

    1 Scope statement   2 Corpus   3 Findings   4 Overlap   5 Evaluation
    6 Reconciliation   7 Policy decisions   8 Evidence model
    9 Limitations   10 Runtime

The layout is fixed at four pages; a PageBreak separates the groups
(1-2 | 3-4 | 5-7 | 8-10) and tables are truncated with a note rather than
allowed to spill onto a fifth page. --sample titles the document SAMPLE,
stamps every page, and suffixes every number with "[synthetic]".

Only the standard library and reportlab (pinned in requirements.txt) are
used, so it runs from a clean clone after `pip install -r requirements.txt`.
"""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.platypus import (
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)

NO_GROUND_TRUTH = "No ground truth for this lab; recall only against the reference list."
STATES = ("validated", "theorized", "refuted", "declined")
MAX_ROWS = 12  # per table, so four pages stay four pages

# Reason codes shared with internal/reconcile (ported from orbit-ir).
ALL_REASONS = (
    "missing-transcript",
    "missing-control-plane-event",
    "duplicate-transcript-tool-call-id",
    "duplicate-control-tool-call-id",
    "agent-id-mismatch",
    "trace-id-mismatch",
    "tool-name-mismatch",
    "argument-digest-mismatch",
)


class Inputs:
    """The files of one results directory, missing ones as empty values."""

    def __init__(self, directory: Path) -> None:
        self.directory = directory
        self.run = self._load("run.json", required=True)
        self.findings = self._load("findings.redacted.json") or []
        self.overlap = self._load("overlap.json") or {}
        self.reconcile = self._load("reconcile.json")
        self.costs = self._load("costs.json") or {}
        self.ci = self._load("ci.json") or {}

    def _load(self, name: str, required: bool = False):
        path = self.directory / name
        if not path.is_file():
            if required:
                raise SystemExit(f"build_report: missing {path}")
            return None
        try:
            return json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            raise SystemExit(f"build_report: {path}: invalid JSON: {exc.msg}") from exc

    def matrix(self) -> dict:
        if isinstance(self.overlap, dict) and isinstance(self.overlap.get("overlap"), dict):
            return self.overlap["overlap"]
        return self.run.get("overlap") or {}

    def precision(self) -> dict:
        if isinstance(self.overlap, dict) and self.overlap.get("precision"):
            return self.overlap["precision"]
        return self.run.get("precision") or {}


class Renderer:
    def __init__(self, inputs: Inputs, sample: bool) -> None:
        self.i = inputs
        self.sample = sample
        base = getSampleStyleSheet()
        self.h1 = ParagraphStyle("h1", parent=base["Title"], fontSize=20, leading=24, alignment=TA_LEFT, spaceAfter=4)
        self.h2 = ParagraphStyle("h2", parent=base["Heading2"], fontSize=13, leading=16, spaceBefore=10, spaceAfter=4)
        self.body = ParagraphStyle("body", parent=base["BodyText"], fontSize=9.5, leading=13)
        self.small = ParagraphStyle("small", parent=self.body, fontSize=8, leading=10, textColor=colors.HexColor("#555555"))
        self.cell = ParagraphStyle("cell", parent=self.body, fontSize=7.5, leading=9)
        self.mono = ParagraphStyle("mono", parent=self.body, fontName="Courier", fontSize=8, leading=10)

    # ----- formatting helpers -------------------------------------------------
    def n(self, value, digits: int | None = None) -> str:
        """Format a number; in sample mode every number carries the label."""
        if value is None:
            return "-"
        if isinstance(value, bool):
            text = "yes" if value else "no"
        elif isinstance(value, float) or digits is not None:
            text = f"{float(value):.{digits if digits is not None else 3}f}"
        else:
            text = str(value)
        return f"{text} [synthetic]" if self.sample else text

    def p(self, text: str, style: ParagraphStyle | None = None) -> Paragraph:
        return Paragraph(text, style or self.body)

    def table(self, header: list[str], rows: list[list[str]], widths: list[float] | None = None) -> list:
        truncated = len(rows) > MAX_ROWS
        rows = rows[:MAX_ROWS]
        data = [[self.p(f"<b>{h}</b>", self.cell) for h in header]]
        for r in rows:
            data.append([self.p(str(c), self.cell) for c in r])
        t = Table(data, colWidths=widths, repeatRows=1, hAlign="LEFT")
        t.setStyle(TableStyle([
            ("GRID", (0, 0), (-1, -1), 0.25, colors.HexColor("#999999")),
            ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#e8e6e1")),
            ("VALIGN", (0, 0), (-1, -1), "TOP"),
            ("LEFTPADDING", (0, 0), (-1, -1), 3),
            ("RIGHTPADDING", (0, 0), (-1, -1), 3),
            ("TOPPADDING", (0, 0), (-1, -1), 2),
            ("BOTTOMPADDING", (0, 0), (-1, -1), 2),
        ]))
        out: list = [t]
        if truncated:
            out.append(self.p(f"Table truncated to {MAX_ROWS} rows to keep the fixed page count; the JSON files hold the rest.", self.small))
        return out

    # ----- sections -------------------------------------------------------------
    def title_block(self) -> list:
        run = self.i.run
        title = "SAMPLE REPORT (SYNTHETIC DATA)" if self.sample else f"assay run {run.get('run_id', '-')}"
        ci_url = self.i.ci.get("run_url") or run.get("ci_run_url") or "-"
        out = [self.p(title, self.h1)]
        if self.sample:
            out.append(self.p("Every figure in this document is synthetic and labelled as such. "
                              "It exists to show the report layout and is not a result.", self.body))
        out += [
            self.p(f"Run id: <b>{run.get('run_id', '-')}</b> &nbsp; Git SHA: <font face='Courier'>{run.get('git_sha', '-') or '-'}</font>"),
            self.p(f"CI run: {ci_url}"),
            self.p(f"Mode: {run.get('mode', '-')} &nbsp; Verdict: <b>{run.get('verdict', '-')}</b> &nbsp; "
                   f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')}", self.small),
            Spacer(1, 6),
        ]
        return out

    def scope_statement(self) -> list:
        scope = self.i.run.get("scope") or {}
        return [
            self.p("1. Scope statement", self.h2),
            self.p(
                f"Every request in this run passed the signed scope gate (decision <b>{scope.get('effect', '-')}</b>, "
                f"reason <font face='Courier'>{scope.get('reason', '-')}</font>). Targets were the authorized lab services "
                "listed under Corpus and nothing else. Findings start <i>theorized</i>; only a deterministic checker in an "
                "authorized lab can mark one <i>validated</i> or <i>refuted</i>. No transcript is retained; the audit log "
                "holds digests and bounded metadata only."
            ),
        ]

    def corpus(self) -> list:
        run = self.i.run
        models = sorted({m.get("ref", {}).get("model", "-") for m in run.get("models", [])})
        labs = run.get("labs") or []
        ci = self.i.ci
        rows = [
            ["Run id", run.get("run_id", "-")],
            ["Labs", ", ".join(labs) or "-"],
            ["Models", ", ".join(models) or "-"],
            ["CI run URL", ci.get("run_url") or run.get("ci_run_url") or "-"],
            ["CI run id / attempt", f"{ci.get('run_id', '-')} / {ci.get('attempt', '-')}"],
            ["Workflow", ci.get("workflow", "-")],
            ["Commit", ci.get("sha") or run.get("git_sha") or "-"],
        ]
        return [self.p("2. Corpus", self.h2)] + self.table(["Field", "Value"], rows, [40 * mm, 130 * mm])

    def findings(self) -> list:
        run = self.i.run
        rows = []
        for m in sorted(run.get("models", []), key=lambda x: x.get("ref", {}).get("model", "")):
            by = m.get("findings_by_state") or {}
            total = sum(int(by.get(s, 0)) for s in STATES)
            rows.append([m.get("ref", {}).get("model", "-")] + [self.n(int(by.get(s, 0))) for s in STATES] + [self.n(total)])
        if not rows:
            counts: dict[str, dict[str, int]] = {}
            for f in self.i.findings:
                name = (f.get("model") or {}).get("model") or "(unattributed)"
                counts.setdefault(name, {s: 0 for s in STATES})
                counts[name][f.get("state", "theorized")] = counts[name].get(f.get("state", "theorized"), 0) + 1
            for name in sorted(counts):
                by = counts[name]
                rows.append([name] + [self.n(by.get(s, 0)) for s in STATES] + [self.n(sum(by.values()))])
        out = [self.p("3. Findings by state and model", self.h2)]
        if not rows:
            out.append(self.p("No findings were recorded."))
            return out
        out += self.table(["Model", "Validated", "Theorized", "Refuted", "Declined", "Total"], rows)
        keys = {f.get("dedup_key") or f.get("id") for f in self.i.findings}
        if self.i.findings:
            out.append(self.p(f"{self.n(len(self.i.findings))} finding record(s); {self.n(len(keys))} distinct dedup key(s).", self.small))
        return out

    def overlap(self) -> list:
        m = self.i.matrix()
        out = [self.p("4. Overlap", self.h2)]
        models = m.get("models") or []
        if not models:
            out.append(self.p("No overlap matrix (fewer than one model with findings)."))
            return out
        out.append(self.p(f"Union {self.n(m.get('union', 0))}, intersection {self.n(m.get('intersection', 0))} "
                          f"across {self.n(len(models))} model(s)."))
        pair_rows = []
        pairwise = m.get("pairwise") or []
        for a in range(len(models)):
            for b in range(a + 1, len(models)):
                if a < len(pairwise) and b < len(pairwise[a]):
                    c = pairwise[a][b]
                    pair_rows.append([models[a], models[b], self.n(c.get("both", 0)), self.n(c.get("only_a", 0)),
                                      self.n(c.get("only_b", 0)), self.n(float(c.get("jaccard", 0.0)), 3)])
        if len(models) < 2:
            pair_rows.append([models[0], "-", "-", "-", "-", "-"])
        out += self.table(["A", "B", "Both", "Only A", "Only B", "Jaccard"], pair_rows)
        out.append(Spacer(1, 4))
        unique = m.get("unique_per_model") or {}
        out += self.table(["Model", "Unique keys"], [[name, self.n(len(unique.get(name, [])))] for name in models], [80 * mm, 40 * mm])
        return out

    def evaluation(self) -> list:
        prf = self.i.precision()
        out = [self.p("5. Evaluation", self.h2)]
        if not prf:
            out.append(self.p(NO_GROUND_TRUTH))
            return out
        rows = []
        for name in sorted(prf):
            p = prf[name]
            rows.append([name, self.n(p.get("tp", 0)), self.n(p.get("fp", 0)), self.n(p.get("fn", 0)),
                         self.n(float(p.get("precision", 0)), 3), self.n(float(p.get("recall", 0)), 3), self.n(float(p.get("f1", 0)), 3)])
        out += self.table(["Model", "TP", "FP", "FN", "Precision", "Recall", "F1"], rows)
        out.append(self.p("Ground truth is consulted after detection only; it never changes a finding.", self.small))
        return out

    def reconciliation(self) -> list:
        out = [self.p("6. Reconciliation", self.h2)]
        rec = self.i.reconcile
        if rec is not None:
            mismatches = rec.get("mismatches") or []
            out.append(self.p(f"Checked {self.n(rec.get('checked', 0))} tool call id(s); {self.n(len(mismatches))} mismatch(es)."))
            counts = rec.get("reason_counts") or {}
            if counts:
                out += self.table(["Reason", "Count"], [[r, self.n(counts[r])] for r in sorted(counts)], [90 * mm, 30 * mm])
            if mismatches:
                out.append(Spacer(1, 4))
                rows = [[m.get("tool_call_id", "-"), ", ".join(m.get("reasons") or []), m.get("claimed") or "-",
                         m.get("actual") or "-", m.get("confidence", "-")] for m in mismatches]
                out += self.table(["Tool call id", "Reasons", "Claimed", "Actual", "Confidence"], rows,
                                  [30 * mm, 70 * mm, 25 * mm, 25 * mm, 20 * mm])
            out.append(self.p("Findings whose tool calls appear above were downgraded to theorized.", self.small))
            return out
        counts = self.i.run.get("reconcile_reason_counts") or {}
        if counts:
            out += self.table(["Reason", "Count"], [[r, self.n(counts[r])] for r in sorted(counts)], [90 * mm, 30 * mm])
        else:
            out.append(self.p("No reconciliation mismatches recorded."))
        return out

    def policy(self) -> list:
        denies = self.i.run.get("policy_denies_by_reason") or {}
        out = [self.p("7. Policy decisions", self.h2)]
        if not denies:
            out.append(self.p("No policy denials recorded."))
            return out
        out += self.table(["Deny reason", "Count"], [[r, self.n(denies[r])] for r in sorted(denies)], [110 * mm, 30 * mm])
        return out

    def evidence(self) -> list:
        run = self.i.run
        return [
            self.p("8. Evidence model", self.h2),
            self.p("model call &rarr; claimed tool call &rarr; policy decision &rarr; executed tool call &rarr; gate decision "
                   "&rarr; HTTP exchange digest &rarr; finding &rarr; checker evidence", self.mono),
            Spacer(1, 4),
        ] + self.table(["Field", "Value"], [
            ["Audit head hash", run.get("audit_head_hash") or "-"],
            ["Audit record count", self.n(run.get("audit_record_count", 0))],
            ["Audit key id", "see the audit log header (not carried in run.json)"],
            ["Transcript retention", "none (zero data retention sink)"],
        ], [45 * mm, 125 * mm])

    def limitations(self) -> list:
        run = self.i.run
        items = [
            "Labs are deliberately vulnerable services; numbers describe recovery on known targets, not expected accuracy on production systems.",
            "Dedup keys are lexical (class, method, templated path, parameter); semantically identical findings at different paths stay distinct.",
            "Classes without a non-destructive checker remain theorized and are never counted as validated.",
            "Refusals are recorded and never retried; a model that declines a lab contributes zero findings for it by design.",
            "Provider-side data handling is governed by each provider's agreement and is not measured or claimed here.",
        ]
        if run.get("mode") == "degraded":
            items.append("This run was degraded: at least one configured model was dropped for a missing key, so cross-model comparisons are partial.")
        if run.get("truncated"):
            items.append("The run hit a budget or latency cap and was truncated; counts are lower bounds.")
        if not self.i.precision():
            items.append("No ground truth was available; precision cannot be stated for this run.")
        if self.sample:
            items.append("This is a SAMPLE rendered from generated data; nothing here was measured.")
        return [self.p("9. Limitations", self.h2)] + [self.p(f"&bull; {t}") for t in items]

    def runtime(self) -> list:
        run = self.i.run
        started, finished = run.get("started"), run.get("finished")
        duration = "-"
        try:
            if started and finished:
                a = datetime.fromisoformat(str(started).replace("Z", "+00:00"))
                b = datetime.fromisoformat(str(finished).replace("Z", "+00:00"))
                duration = f"{int((b - a).total_seconds())} s"
        except ValueError:
            duration = "-"
        rows = [
            ["Started", str(started or "-")],
            ["Finished", str(finished or "-")],
            ["Duration", self.n(duration) if self.sample and duration != "-" else duration],
            ["Spent USD", self.n(float(run.get("spent_usd", 0.0)), 4)],
            ["Budget cap USD", self.n(float(run.get("budget_cap_usd", 0.0)), 2)],
            ["Truncated", self.n(bool(run.get("truncated", False)))],
            ["Verdict / exit code", f"{run.get('verdict', '-')} / {self.n(run.get('exit_code', 0))}"],
        ]
        out = [self.p("10. Runtime", self.h2)] + self.table(["Field", "Value"], rows, [45 * mm, 125 * mm])
        models = sorted(run.get("models", []), key=lambda x: x.get("ref", {}).get("model", ""))
        if models:
            out.append(Spacer(1, 4))
            out += self.table(
                ["Model", "Cost USD", "p50 ms", "p95 ms", "Refusals", "Failovers"],
                [[m.get("ref", {}).get("model", "-"), self.n(float((m.get("usage") or {}).get("cost_usd", 0.0)), 4),
                  self.n(m.get("p50_ms", 0)), self.n(m.get("p95_ms", 0)), self.n(m.get("refusals", 0)), self.n(m.get("failovers", 0))]
                 for m in models],
            )
        return out

    # ----- document -----------------------------------------------------------
    def story(self) -> list:
        return (
            self.title_block() + self.scope_statement() + self.corpus()
            + [PageBreak()] + self.findings() + self.overlap()
            + [PageBreak()] + self.evaluation() + self.reconciliation() + self.policy()
            + [PageBreak()] + self.evidence() + self.limitations() + self.runtime()
        )

    def decorate(self, canvas, doc) -> None:
        canvas.saveState()
        canvas.setFont("Helvetica", 8)
        canvas.setFillColor(colors.HexColor("#666666"))
        label = "assay results report"
        if self.sample:
            label = "SAMPLE: all figures synthetic"
            canvas.saveState()
            canvas.setFont("Helvetica-Bold", 60)
            canvas.setFillColor(colors.Color(0.85, 0.85, 0.85, alpha=0.35))
            canvas.translate(A4[0] / 2, A4[1] / 2)
            canvas.rotate(35)
            canvas.drawCentredString(0, 0, "SAMPLE")
            canvas.restoreState()
        canvas.drawString(18 * mm, 10 * mm, label)
        canvas.drawRightString(A4[0] - 18 * mm, 10 * mm, f"page {doc.page} of 4")
        canvas.restoreState()

    def build(self, out: Path) -> None:
        out.parent.mkdir(parents=True, exist_ok=True)
        doc = SimpleDocTemplate(
            str(out), pagesize=A4,
            leftMargin=18 * mm, rightMargin=18 * mm, topMargin=16 * mm, bottomMargin=18 * mm,
            title="SAMPLE assay report (synthetic)" if self.sample else f"assay run {self.i.run.get('run_id', '')}",
            author="assay", subject="security-validation harness run report",
        )
        doc.build(self.story(), onFirstPage=self.decorate, onLaterPages=self.decorate)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("results_dir", type=Path)
    ap.add_argument("--out", type=Path, default=None, help="output PDF (default RESULTS_DIR/report.pdf)")
    ap.add_argument("--sample", action="store_true", help="title the document SAMPLE and label every number synthetic")
    args = ap.parse_args(argv)
    if not args.results_dir.is_dir():
        print(f"build_report: {args.results_dir} is not a directory", file=sys.stderr)
        return 2
    inputs = Inputs(args.results_dir)
    out = args.out or (args.results_dir / "report.pdf")
    Renderer(inputs, args.sample).build(out)
    print(f"build_report: wrote {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
