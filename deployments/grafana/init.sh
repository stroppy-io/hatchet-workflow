#!/bin/sh
# Build the `public` Grafana organisation that anonymous share-link viewers land
# in (GF_AUTH_ANONYMOUS_ORG_NAME=public).
#
# Everything here goes through Grafana's HTTP API rather than file provisioning:
# provisioning files are read at startup and Grafana REFUSES TO BOOT when one of
# them names an organisation that does not exist — and only Grafana's own API can
# create an org. So the org, its datasource and its dashboards are created here,
# after Grafana is up. Until then anonymous requests simply 404 (fail closed).
#
# The public org holds exactly ONE datasource, pointing at the gateway's
# share-scoped metrics proxy. That is the isolation boundary: an anonymous viewer
# cannot reach Main Org.'s unscoped vmselect datasource, and every query the
# scoped one serves is pinned to the single run the share token grants.
#
# Idempotent: safe to re-run on every deploy.
set -eu

GRAFANA_URL="${GRAFANA_URL:-http://grafana:3000}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:?GRAFANA_PASSWORD is required}"
PUBLIC_ORG="${PUBLIC_ORG:-public}"
PUBLIC_METRICS_URL="${PUBLIC_METRICS_URL:?PUBLIC_METRICS_URL is required}"
DASHBOARD_DIR="${DASHBOARD_DIR:-/dashboards}"
FOLDER_UID=stroppy-public

auth="$GRAFANA_USER:$GRAFANA_PASSWORD"
api() { curl -s -u "$auth" -H 'Content-Type: application/json' "$@"; }

echo "grafana-init: waiting for $GRAFANA_URL"
i=0
until curl -fsS "$GRAFANA_URL/api/health" >/dev/null 2>&1; do
	i=$((i + 1))
	if [ "$i" -ge 150 ]; then
		echo "grafana-init: grafana did not become healthy" >&2
		exit 1
	fi
	sleep 2
done

# 1) The org. An existing one answers 409/412.
code=$(api -o /tmp/org.json -w '%{http_code}' -X POST "$GRAFANA_URL/api/orgs" -d "{\"name\":\"$PUBLIC_ORG\"}")
case "$code" in
200 | 409 | 412) ;;
*)
	echo "grafana-init: create org failed (http $code): $(cat /tmp/org.json)" >&2
	exit 1
	;;
esac
org_id=$(api "$GRAFANA_URL/api/orgs/name/$PUBLIC_ORG" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
if [ -z "$org_id" ]; then
	echo "grafana-init: could not resolve org '$PUBLIC_ORG'" >&2
	exit 1
fi
echo "grafana-init: org '$PUBLIC_ORG' = id $org_id"

# 2) Act as that org for the rest of this script.
api -o /dev/null -X POST "$GRAFANA_URL/api/user/using/$org_id"

# 3) The single scoped datasource. Recreated so a changed URL / keepCookies takes
#    effect. Dashboards reference it by NAME, and names are per-org.
api -o /dev/null -X DELETE "$GRAFANA_URL/api/datasources/name/VictoriaMetrics" || true
code=$(api -o /tmp/ds.json -w '%{http_code}' -X POST "$GRAFANA_URL/api/datasources" -d "{
  \"name\": \"VictoriaMetrics\",
  \"uid\": \"victoriametrics-public\",
  \"type\": \"prometheus\",
  \"access\": \"proxy\",
  \"url\": \"$PUBLIC_METRICS_URL\",
  \"isDefault\": true,
  \"jsonData\": { \"keepCookies\": [\"stroppy_share\"] }
}")
if [ "$code" != "200" ]; then
	echo "grafana-init: create datasource failed (http $code): $(cat /tmp/ds.json)" >&2
	exit 1
fi
echo "grafana-init: scoped datasource -> $PUBLIC_METRICS_URL"

# 4) Folder for the dashboards (409 = already there).
api -o /dev/null -X POST "$GRAFANA_URL/api/folders" -d "{\"uid\":\"$FOLDER_UID\",\"title\":\"Stroppy\"}" || true

# 5) The dashboards themselves. Every file carries "id": null, so it can be
#    posted verbatim; overwrite keeps re-runs idempotent.
for f in "$DASHBOARD_DIR"/*.json; do
	[ -e "$f" ] || continue
	name=$(basename "$f")
	payload=/tmp/dash.json
	{
		printf '{"folderUid":"%s","overwrite":true,"dashboard":' "$FOLDER_UID"
		cat "$f"
		printf '}'
	} >"$payload"
	code=$(api -o /tmp/resp.json -w '%{http_code}' -X POST "$GRAFANA_URL/api/dashboards/db" --data-binary "@$payload")
	if [ "$code" != "200" ]; then
		echo "grafana-init: import $name failed (http $code): $(head -c 300 /tmp/resp.json)" >&2
		exit 1
	fi
	echo "grafana-init: imported $name"
done

echo "grafana-init: done"
