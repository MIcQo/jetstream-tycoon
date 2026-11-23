---
name: add-metric-alert
description: Add a Prometheus metric, alert rule and/or Grafana panel for a backend service.
---

# Add a metric, alert or panel

1. Metric (`prometheus/client_golang`):
   - Names in snake_case with units (`_seconds`, `_bytes`, `_total`).
   - Labels are low-cardinality only: route templates, enums. **Never** user, save or device IDs, or raw paths.
2. Register it in the service's metrics package and document it in the `docs/TECH_DESIGN.md` §21 table.
3. Alert: add it to `deploy/prometheus/rules/*.yml` with `for:`, `severity`, a `summary` and a `runbook_url` pointing to `docs/runbooks/<alert>.md`. Write that runbook.
4. Panel: edit the provisioned dashboard JSON in `deploy/grafana/dashboards/`. Analytics panels must query ClickHouse **rollups**, not raw events.
5. Check it: `promtool check rules deploy/prometheus/rules/*.yml`.
