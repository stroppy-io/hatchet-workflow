#!/usr/bin/env bash
# Audit and smoke-test system test presets through the public Connect API.
#
# Typical stage usage:
#   set -a; . ./.env; set +a
#   ./scripts/stage-preset-harness.sh audit
#   PRESET_NAME_REGEX='single|Self-check / PostgreSQL' ./scripts/stage-preset-harness.sh smoke
#
# Output is intentionally compact:
#   - stdout: one-line progress suitable for long loops
#   - report dir: JSONL with full normalized records and per-run failure logs

set -euo pipefail

MODE="${1:-audit}"

BASE="${STROPPY_BASE_URL:-${PUBLIC_SERVER_ADDR:-https://stage.cloud.stroppy.io}}"
LOGIN="${STROPPY_ADMIN_LOGIN:-admin}"
PASSWORD="${STROPPY_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}"
TENANT_SLUG="${STROPPY_TENANT_SLUG:-default}"
TENANT_ID="${STROPPY_TENANT_ID:-}"
PROVIDER="${STROPPY_PROVIDER:-PROVIDER_YANDEX}"

PRESET_NAME_REGEX="${PRESET_NAME_REGEX:-}"
PRESET_ID_REGEX="${PRESET_ID_REGEX:-}"
PRESET_DB_KIND_REGEX="${PRESET_DB_KIND_REGEX:-}"
PRESET_LIMIT="${PRESET_LIMIT:-0}"

SMOKE_DURATION="${SMOKE_DURATION:-60s}"
SMOKE_VUS="${SMOKE_VUS:-1}"
SMOKE_POOL_SIZE="${SMOKE_POOL_SIZE:-2}"
SMOKE_SCALE_FACTOR="${SMOKE_SCALE_FACTOR:-1}"
RUN_TIMEOUT_SECONDS="${RUN_TIMEOUT_SECONDS:-3600}"
RUN_POLL_SECONDS="${RUN_POLL_SECONDS:-15}"
LOG_LIMIT="${LOG_LIMIT:-250}"
TEMPORAL_DESCRIBE="${TEMPORAL_DESCRIBE:-1}"

DELETE_DRAFTS="${DELETE_DRAFTS:-1}"
RUN_READY_ONLY="${RUN_READY_ONLY:-1}"
REPORT_DIR="${REPORT_DIR:-.stage-preset-harness/$(date -u +%Y%m%dT%H%M%SZ)}"
REPORT_JSONL="$REPORT_DIR/report.jsonl"
PRESETS_JSONL="$REPORT_DIR/presets.jsonl"

case "$MODE" in
	audit|smoke|list) ;;
	*)
		echo "usage: $0 [list|audit|smoke]" >&2
		exit 2
		;;
esac

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "missing dependency: $1" >&2
		exit 2
	}
}

need curl
need jq

if [[ -z "$PASSWORD" ]]; then
	echo "STROPPY_ADMIN_PASSWORD or ADMIN_PASSWORD is required" >&2
	exit 2
fi

mkdir -p "$REPORT_DIR"
: >"$REPORT_JSONL"
: >"$PRESETS_JSONL"

api_public() {
	local path="$1"
	local payload="$2"
	curl -fsS \
		-H "Content-Type: application/json" \
		--data-binary "$payload" \
		"$BASE$path"
}

api_auth() {
	local path="$1"
	local payload="$2"
	curl -fsS \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $TOKEN" \
		--data-binary "$payload" \
		"$BASE$path"
}

api_auth_maybe() {
	local path="$1"
	local payload="$2"
	local out="$3"
	local status
	status="$(curl -sS \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $TOKEN" \
		--data-binary "$payload" \
		-o "$out" \
		-w "%{http_code}" \
		"$BASE$path")"
	printf '%s' "$status"
}

jsonl() {
	jq -c "$@" >>"$REPORT_JSONL"
}

log_event() {
	local level="$1"
	shift
	printf '%s %s\n' "$level" "$*"
}

TOKEN="$(api_public /cloud.v1.api.IamService/Login "$(jq -cn --arg login "$LOGIN" --arg password "$PASSWORD" '{login:$login,password:$password}')" | jq -r '.tokens.accessToken // empty')"
if [[ -z "$TOKEN" ]]; then
	echo "login failed" >&2
	exit 1
fi

if [[ -z "$TENANT_ID" ]]; then
	TENANT_ID="$(api_auth /cloud.v1.api.IamService/ListMyTenants '{}' | jq -r --arg slug "$TENANT_SLUG" '
		.tenants[]
		| select(
			((.slug // "") == $slug)
			or (((.entity.name // .name // "") | ascii_downcase) == ($slug | ascii_downcase))
		)
		| (.entity.id // .id)
	' | head -n1)"
fi
if [[ -z "$TENANT_ID" ]]; then
	echo "tenant not found: $TENANT_SLUG" >&2
	exit 1
fi

log_event INFO "base=$BASE tenant=$TENANT_ID provider=$PROVIDER report=$REPORT_DIR"

list_presets() {
	local token=""
	local fetched=0
	while :; do
		local req resp next
		req="$(jq -cn --arg tenant "$TENANT_ID" --arg token "$token" '{tenantId:$tenant,isSystem:true,page:{size:100,token:$token}}')"
		resp="$(api_auth /cloud.v1.api.TestPresetService/ListTestPresets "$req")"
		jq -c '.presets[]?' <<<"$resp" >>"$PRESETS_JSONL"
		fetched=$((fetched + $(jq '.presets | length' <<<"$resp")))
		next="$(jq -r '.nextPageToken // ""' <<<"$resp")"
		[[ -z "$next" ]] && break
		token="$next"
	done
	log_event INFO "listed=$fetched"
}

filtered_presets() {
	jq -c \
		--arg name_re "$PRESET_NAME_REGEX" \
		--arg id_re "$PRESET_ID_REGEX" \
		--arg db_re "$PRESET_DB_KIND_REGEX" \
		--argjson limit "$PRESET_LIMIT" '
		def matches_or_empty($re; $value):
			($re == "") or (($value // "") | test($re; "i"));

		[.[]
			| select(matches_or_empty($name_re; .entity.name))
			| select(matches_or_empty($id_re; .entity.id))
			| select(matches_or_empty($db_re; (.summary.dbKind // .test.database.kind)))
		]
		| if $limit > 0 then .[:$limit] else . end
		| .[]
	' -s "$PRESETS_JSONL"
}

preset_brief() {
	jq -cr '[
		(.entity.id // ""),
		(.entity.name // ""),
		(.summary.dbKind // .test.database.kind // ""),
		(.summary.protocol // .test.workload.protocol // ""),
		(.test.workload.script // "")
	] | @tsv'
}

normalize_workload() {
	jq -c \
		--argjson vus "$SMOKE_VUS" \
		--arg duration "$SMOKE_DURATION" \
		--argjson pool "$SMOKE_POOL_SIZE" \
		--argjson scale "$SMOKE_SCALE_FACTOR" '
		.execution = {
			vus: $vus,
			duration: $duration,
			quiet: true,
			noThresholds: true
		}
		| .parameters = ((.parameters // {}) + {
			poolSize: $pool,
			scaleFactor: $scale
		})
	'
}

draft_errors_tsv() {
	jq -r '.draft.errors[]? | [(.field // ""), (.message // "")] | @tsv'
}

delete_draft() {
	local draft_id="$1"
	[[ "$DELETE_DRAFTS" != "1" || -z "$draft_id" ]] && return 0
	api_auth /cloud.v1.api.TestWizardService/DeleteTestWizardDraft "$(jq -cn --arg tenant "$TENANT_ID" --arg draft "$draft_id" '{tenantId:$tenant,draftId:$draft}')" >/dev/null || true
}

audit_preset() {
	local preset="$1"
	local launch="$2"
	local preset_id name db_kind protocol script started draft_id patch payload ready overrides status_file http_status
	local normalized_db normalized_workload
	preset_id="$(jq -r '.entity.id' <<<"$preset")"
	name="$(jq -r '.entity.name' <<<"$preset")"
	db_kind="$(jq -r '.summary.dbKind // .test.database.kind // ""' <<<"$preset")"
	protocol="$(jq -r '.summary.protocol // .test.workload.protocol // ""' <<<"$preset")"
	script="$(jq -r '.test.workload.script // ""' <<<"$preset")"

	normalized_db="$(jq -c '.test.database' <<<"$preset")"
	normalized_workload="$(jq -c '.test.workload' <<<"$preset" | normalize_workload)"

	payload="$(jq -cn --arg tenant "$TENANT_ID" --arg name "preset harness audit: $name" --arg preset "$preset_id" '{tenantId:$tenant,name:$name,testPresetId:$preset}')"
	started="$(api_auth /cloud.v1.api.TestWizardService/StartTestWizard "$payload")"
	draft_id="$(jq -r '.draft.entity.id // .draft.id // empty' <<<"$started")"
	if [[ -z "$draft_id" ]]; then
		jsonl -n \
			--arg phase "audit" --arg preset_id "$preset_id" --arg name "$name" --arg db_kind "$db_kind" --arg protocol "$protocol" \
			'{phase:$phase,presetId:$preset_id,name:$name,dbKind:$db_kind,protocol:$protocol,ready:false,error:"missing draft id"}'
		log_event FAIL "AUDIT preset=$preset_id name=$name reason=missing_draft_id"
		return 1
	fi

	payload="$(jq -cn \
		--arg tenant "$TENANT_ID" \
		--arg draft "$draft_id" \
		--arg provider "$PROVIDER" \
		--argjson db "$normalized_db" \
		--argjson workload "$normalized_workload" \
		'{tenantId:$tenant,draftId:$draft,provider:$provider,database:$db,workload:$workload}')"

	status_file="$REPORT_DIR/patch-$preset_id.json"
	http_status="$(api_auth_maybe /cloud.v1.api.TestWizardService/PatchTestWizard "$payload" "$status_file")"
	if [[ "$http_status" != "200" ]]; then
		jq -cn \
			--arg phase "audit" --arg preset_id "$preset_id" --arg name "$name" --arg db_kind "$db_kind" --arg protocol "$protocol" \
			--arg status "$http_status" --rawfile body "$status_file" \
			'{phase:$phase,presetId:$preset_id,name:$name,dbKind:$db_kind,protocol:$protocol,ready:false,httpStatus:$status,error:$body}' >>"$REPORT_JSONL"
		log_event FAIL "AUDIT preset=$preset_id db=$db_kind name=$name http=$http_status"
		delete_draft "$draft_id"
		return 1
	fi

	ready="$(jq -r '.draft.ready // false' "$status_file")"
	if [[ "$ready" != "true" ]]; then
		overrides="$(jq -c '[.draft.infrastructurePlan.machines[]? | {nodeId, yandex}] | map(select(.yandex != null))' "$status_file")"
		if [[ "$overrides" != "[]" ]]; then
			payload="$(jq -cn --arg tenant "$TENANT_ID" --arg draft "$draft_id" --argjson overrides "$overrides" '{tenantId:$tenant,draftId:$draft,machineOverrides:$overrides}')"
			http_status="$(api_auth_maybe /cloud.v1.api.TestWizardService/PatchTestWizard "$payload" "$status_file")"
			[[ "$http_status" == "200" ]] && ready="$(jq -r '.draft.ready // false' "$status_file")"
		fi
	fi

	local errors_json machine_count
	errors_json="$(jq -c '[.draft.errors[]? | {field:(.field // ""),message:(.message // "")}]' "$status_file")"
	machine_count="$(jq -r '.draft.infrastructurePlan.machines | length // 0' "$status_file")"

	jq -cn \
		--arg phase "audit" \
		--arg preset_id "$preset_id" \
		--arg name "$name" \
		--arg db_kind "$db_kind" \
		--arg protocol "$protocol" \
		--arg script "$script" \
		--arg draft_id "$draft_id" \
		--argjson ready "$ready" \
		--argjson machine_count "$machine_count" \
		--argjson errors "$errors_json" \
		'{phase:$phase,presetId:$preset_id,name:$name,dbKind:$db_kind,protocol:$protocol,script:$script,draftId:$draft_id,ready:$ready,machineCount:$machine_count,errors:$errors}' >>"$REPORT_JSONL"

	if [[ "$ready" != "true" ]]; then
		log_event FAIL "AUDIT preset=$preset_id db=$db_kind name=$name ready=false errors=$(jq -r '.draft.errors | length // 0' "$status_file")"
		delete_draft "$draft_id"
		return 1
	fi

	log_event OK "AUDIT preset=$preset_id db=$db_kind machines=$machine_count name=$name"

	if [[ "$launch" != "1" ]]; then
		delete_draft "$draft_id"
		return 0
	fi

	launch_run "$preset_id" "$name" "$db_kind" "$protocol" "$draft_id"
}

classify_failure() {
	local text="$1"
	case "$text" in
		*'curl: (35)'*'wrong version number'*|*'SSL routines::wrong version number'*) printf 'binary/download-proxy' ;;
		*'no free CIDR'*|*'AcquireNetworkActivity'*) printf 'terraform/network' ;;
		*'TerraformApplyActivity'*|*'terraform apply'*|*'InvalidArgument desc'*|*'core fraction'*) printf 'terraform/provider' ;;
		*'apt-get'*|*'InRelease'*|*'package'*|*'PGDG'*|*'Clearsigned file'*) printf 'apt/install' ;;
		*'stroppy-agent'*|*'/agent/binary'*|*'Failed at step EXEC'*) printf 'agent/bootstrap' ;;
		*'systemctl'*|*'healthcheck'*|*'pg_isready'*|*'accepting connections'*) printf 'database/start' ;;
		*'step/900_run_stroppy'*|*'k6'*|*'stroppy run'*|*'Run failed'*) printf 'stroppy/workload' ;;
		*) printf 'unknown' ;;
	esac
}

temporal_describe() {
	local run_id="$1"
	local out="$2"
	: >"$out"
	[[ "$TEMPORAL_DESCRIBE" != "1" ]] && return 0
	command -v docker >/dev/null 2>&1 || return 0
	docker compose --profile prod ps temporal >/dev/null 2>&1 || return 0
	docker compose --profile prod exec -T temporal \
		temporal workflow describe \
		--address temporal:7233 \
		--namespace default \
		--workflow-id "test-run/$run_id" >"$out" 2>/dev/null || true
}

temporal_failure_text() {
	local file="$1"
	awk '
		/^Results:/ {in_results=1}
		in_results {print}
	' "$file" | head -n 80
}

query_logs() {
	local run_id="$1"
	local out="$2"
	local req
	req="$(jq -cn --arg tenant "$TENANT_ID" --arg run "$run_id" --argjson limit "$LOG_LIMIT" '{tenantId:$tenant,runId:$run,direction:"LOG_SCROLL_DIRECTION_OLDER",limit:$limit}')"
	api_auth /cloud.v1.api.TestRunOverviewService/QueryLogs "$req" >"$out" || true
}

launch_run() {
	local preset_id="$1"
	local name="$2"
	local db_kind="$3"
	local protocol="$4"
	local draft_id="$5"
	local finish run_id status deadline req rec logs_file temporal_file flat_logs temporal_text class error_text failure_text

	finish="$(api_auth /cloud.v1.api.TestWizardService/FinishTestWizard "$(jq -cn --arg tenant "$TENANT_ID" --arg draft "$draft_id" '{tenantId:$tenant,draftId:$draft,start:true,saveAsPreset:false,inTenantRating:true,inGlobalRating:false}')")"
	run_id="$(jq -r '.run.entity.id // .run.id // empty' <<<"$finish")"
	status="$(jq -r '.run.status // empty' <<<"$finish")"
	if [[ -z "$run_id" ]]; then
		jq -cn --arg phase "launch" --arg preset_id "$preset_id" --arg name "$name" --arg error "missing run id" '{phase:$phase,presetId:$preset_id,name:$name,error:$error}' >>"$REPORT_JSONL"
		log_event FAIL "LAUNCH preset=$preset_id name=$name reason=missing_run_id"
		return 1
	fi

	log_event RUN "LAUNCH preset=$preset_id run=$run_id status=$status name=$name"
	jq -cn --arg phase "launch" --arg preset_id "$preset_id" --arg name "$name" --arg db_kind "$db_kind" --arg protocol "$protocol" --arg run_id "$run_id" --arg status "$status" \
		'{phase:$phase,presetId:$preset_id,name:$name,dbKind:$db_kind,protocol:$protocol,runId:$run_id,status:$status}' >>"$REPORT_JSONL"

	req="$(jq -cn --arg tenant "$TENANT_ID" --arg id "$run_id" '{tenantId:$tenant,id:$id}')"
	deadline=$(( $(date +%s) + RUN_TIMEOUT_SECONDS ))
	while :; do
		rec="$(api_auth /cloud.v1.api.TestRunService/GetTestRun "$req")"
		status="$(jq -r '.run.status // empty' <<<"$rec")"
		case "$status" in
			STATUS_COMPLETED|STATUS_FAILED|STATUS_CANCELLED|STATUS_SKIPPED) break ;;
		esac
		if [[ "$(date +%s)" -gt "$deadline" ]]; then
			status="TIMEOUT"
			break
		fi
		sleep "$RUN_POLL_SECONDS"
	done

	logs_file="$REPORT_DIR/logs-$run_id.json"
	query_logs "$run_id" "$logs_file"
	temporal_file="$REPORT_DIR/temporal-$run_id.txt"
	temporal_describe "$run_id" "$temporal_file"
	flat_logs="$(jq -r '.lines[]? | [(.observedAt // ""),(.nodeExecutionId // ""),(.stream // ""),(.line // "")] | @tsv' "$logs_file" | tail -n 80)"
	temporal_text="$(temporal_failure_text "$temporal_file")"
	error_text="$(jq -r '.run.error // ""' <<<"${rec:-{}}")"
	failure_text="$(printf '%s\n%s\n%s\n' "$error_text" "$temporal_text" "$flat_logs")"
	class="$(classify_failure "$failure_text")"

	jq -cn \
		--arg phase "result" \
		--arg preset_id "$preset_id" \
		--arg name "$name" \
		--arg db_kind "$db_kind" \
		--arg protocol "$protocol" \
		--arg run_id "$run_id" \
		--arg status "$status" \
		--arg class "$class" \
		--arg error "$error_text" \
		--arg failure "$temporal_text" \
		--arg logs_file "$logs_file" \
		--arg temporal_file "$temporal_file" \
		'{phase:$phase,presetId:$preset_id,name:$name,dbKind:$db_kind,protocol:$protocol,runId:$run_id,status:$status,class:$class,error:$error,failure:$failure,logsFile:$logs_file,temporalFile:$temporal_file}' >>"$REPORT_JSONL"

	if [[ "$status" == "STATUS_COMPLETED" ]]; then
		log_event OK "DONE preset=$preset_id run=$run_id status=$status name=$name"
	else
		log_event FAIL "DONE preset=$preset_id run=$run_id status=$status class=$class name=$name logs=$logs_file"
	fi
}

summarize() {
	log_event INFO "summary"
	jq -r -s '
		{
			audit_total: map(select(.phase=="audit")) | length,
			audit_ready: map(select(.phase=="audit" and .ready==true)) | length,
			results_total: map(select(.phase=="result")) | length,
			results_completed: map(select(.phase=="result" and .status=="STATUS_COMPLETED")) | length,
			failures: map(select((.phase=="audit" and .ready!=true) or (.phase=="result" and .status!="STATUS_COMPLETED")))
		}
		| "audit_total=\(.audit_total) audit_ready=\(.audit_ready) results_total=\(.results_total) results_completed=\(.results_completed) failures=\(.failures|length)"
	' "$REPORT_JSONL"
	jq -r '
		select((.phase=="audit" and .ready!=true) or (.phase=="result" and .status!="STATUS_COMPLETED"))
		| [
			.phase,
			(.status // (if .ready then "ready" else "not_ready" end)),
			(.class // "wizard/model"),
			.presetId,
			.dbKind,
			.name
		] | @tsv
	' "$REPORT_JSONL" || true
}

list_presets

if [[ "$MODE" == "list" ]]; then
	filtered_presets | while IFS= read -r preset; do
		preset_brief <<<"$preset"
	done
	exit 0
fi

processed=0
failed=0
# Read presets into an array first. Iterating over `< <(filtered_presets)`
# breaks in smoke mode because curl/jq inside launch_run consume the loop's
# stdin (the process substitution), draining the remaining presets so only the
# first one ever runs.
presets=()
while IFS= read -r preset; do
	[[ -n "$preset" ]] && presets+=("$preset")
done < <(filtered_presets)
for preset in "${presets[@]}"; do
	processed=$((processed + 1))
	if ! audit_preset "$preset" "$([[ "$MODE" == "smoke" ]] && echo 1 || echo 0)"; then
		failed=$((failed + 1))
		if [[ "$MODE" == "smoke" && "$RUN_READY_ONLY" == "1" ]]; then
			continue
		fi
	fi
done

log_event INFO "processed=$processed failed=$failed report=$REPORT_JSONL"
summarize
