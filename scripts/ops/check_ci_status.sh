#!/usr/bin/env bash
set -euo pipefail

REPO="${1:-rodrigo-s-lange/easysmart_iiot_core}"
WORKFLOW_NAME="${2:-Backend CI}"

URL="https://api.github.com/repos/${REPO}/actions/runs?per_page=20"

python3 - "$URL" "$WORKFLOW_NAME" <<'PY'
import json
import sys
import urllib.request

url = sys.argv[1]
workflow_name = sys.argv[2]

req = urllib.request.Request(url, headers={"Accept": "application/vnd.github+json"})
with urllib.request.urlopen(req, timeout=20) as resp:
    data = json.load(resp)

runs = data.get("workflow_runs", [])
target = None
for run in runs:
    if run.get("name") == workflow_name:
        target = run
        break

if not target:
    print(f"No workflow run found for '{workflow_name}'")
    sys.exit(1)

print("workflow:", target.get("name"))
print("status:", target.get("status"))
print("conclusion:", target.get("conclusion"))
print("branch:", target.get("head_branch"))
print("event:", target.get("event"))
print("updated_at:", target.get("updated_at"))
print("url:", target.get("html_url"))

if target.get("status") != "completed" or target.get("conclusion") != "success":
    sys.exit(2)
PY
