#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────
# Test script: deploys two waves of compose stacks on one VM
#   Deployment A: standalone postgres + pgbouncer + postgres-exporter
#   Deployment B: patroni 3-node cluster + etcd(3) + pgbouncer + postgres-exporter
#   Deployment C: orioledb postgres + pgbouncer + postgres-exporter
# Then deploys A2/B2/C2 in parallel with different ports and verifies
# cross-wave connectivity.
# All checks run via docker exec (no host psql/pg_isready needed).
# ──────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/postgres-docker-compose.yaml"
SHARED_NETWORK="stroppy-test-net"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

log()  { echo -e "${YELLOW}[test]${NC} $*"; }
ok()   { echo -e "${GREEN}[PASS]${NC} $*"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $*"; FAIL=$((FAIL+1)); }

# Run pg_isready inside a container
pg_isready_in() {
    local container="$1" host="${2:-localhost}" port="${3:-5432}" user="${4:-postgres}"
    docker exec "$container" pg_isready -h "$host" -p "$port" -U "$user" >/dev/null 2>&1
}

# Run SQL inside a container
psql_in() {
    local container="$1" host="${2:-localhost}" port="${3:-5432}" sql="$4"
    docker exec -e SQL="$sql" "$container" \
        sh -c 'PGPASSWORD=postgres psql -h "$1" -p "$2" -U postgres -d postgres -tAc "$SQL"' _ "$host" "$port" \
        2>/dev/null | tr -d '[:space:]' || true
}

cleanup() {
    log "Cleaning up..."

    CLUSTER_NAME=test-a SHARED_NETWORK="$SHARED_NETWORK" PG_PORT=15432 PGBOUNCER_PORT=16432 PG_EXPORTER_PORT=19187 \
        docker compose -p test-a -f "$COMPOSE_FILE" \
        --profile standalone --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    CLUSTER_NAME=test-b SHARED_NETWORK="$SHARED_NETWORK" \
    ETCD_INITIAL_CLUSTER="etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380,etcd-3=http://etcd-3:2380" \
    ETCD_HOSTS="etcd-1:2379,etcd-2:2379,etcd-3:2379" \
    PATRONI1_PG_PORT=25432 PATRONI1_API_PORT=28008 \
    PATRONI2_PG_PORT=25433 PATRONI2_API_PORT=28009 \
    PATRONI3_PG_PORT=25434 PATRONI3_API_PORT=28010 \
    ETCD1_CLIENT_PORT=22379 PGBOUNCER_PORT=26432 PGBOUNCER_UPSTREAM_HOST=patroni-1 \
    PG_EXPORTER_PORT=29187 PG_EXPORTER_TARGET=patroni-1 \
        docker compose -p test-b -f "$COMPOSE_FILE" \
        --profile cluster --profile etcd --profile etcd-ha --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    CLUSTER_NAME=test-oriole SHARED_NETWORK="$SHARED_NETWORK" PG_PORT=33432 PGBOUNCER_PORT=64432 PG_EXPORTER_PORT=64587 \
        docker compose -p test-oriole -f "$COMPOSE_FILE" \
        --profile standalone --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    CLUSTER_NAME=test-a2 SHARED_NETWORK="$SHARED_NETWORK" PG_PORT=65022 PGBOUNCER_PORT=65032 PG_EXPORTER_PORT=65087 \
        docker compose -p test-a2 -f "$COMPOSE_FILE" \
        --profile standalone --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    CLUSTER_NAME=test-b2 SHARED_NETWORK="$SHARED_NETWORK" \
    ETCD_INITIAL_CLUSTER="etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380,etcd-3=http://etcd-3:2380" \
    ETCD_HOSTS="etcd-1:2379,etcd-2:2379,etcd-3:2379" \
    PATRONI1_PG_PORT=55432 PATRONI1_API_PORT=58008 \
    PATRONI2_PG_PORT=55433 PATRONI2_API_PORT=58009 \
    PATRONI3_PG_PORT=55434 PATRONI3_API_PORT=58010 \
    ETCD1_CLIENT_PORT=42379 PGBOUNCER_PORT=62432 PGBOUNCER_UPSTREAM_HOST=patroni-1 \
    PG_EXPORTER_PORT=59187 PG_EXPORTER_TARGET=patroni-1 \
        docker compose -p test-b2 -f "$COMPOSE_FILE" \
        --profile cluster --profile etcd --profile etcd-ha --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    CLUSTER_NAME=test-oriole2 SHARED_NETWORK="$SHARED_NETWORK" PG_PORT=65122 PGBOUNCER_PORT=65132 PG_EXPORTER_PORT=65187 \
        docker compose -p test-oriole2 -f "$COMPOSE_FILE" \
        --profile standalone --profile pgbouncer --profile postgres-exporter \
        down -v --remove-orphans 2>/dev/null || true

    docker network rm "$SHARED_NETWORK" 2>/dev/null || true
    log "Cleanup done."
}

trap cleanup EXIT

# ──────────────────────────────────────────────
# 0. Pre-flight: shared network
# ──────────────────────────────────────────────
log "=== PRE-FLIGHT ==="
cleanup
docker network inspect "$SHARED_NETWORK" >/dev/null 2>&1 \
    || docker network create --driver bridge "$SHARED_NETWORK"
log "Shared network '$SHARED_NETWORK' ready."

# ──────────────────────────────────────────────
# 1. Deploy A: standalone postgres + pgbouncer + exporter
# ──────────────────────────────────────────────
log "=== DEPLOYING A (standalone) ==="

export CLUSTER_NAME=test-a
export SHARED_NETWORK
export PG_IMAGE=postgres:17
export PG_EXTRA_PARAMS=
export PG_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=postgres
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=postgres

docker compose -p test-a -f "$COMPOSE_FILE" \
    --profile standalone --profile pgbouncer --profile postgres-exporter \
    up -d --build --wait --wait-timeout 120

docker exec test-a-postgres psql -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';"

log "Deployment A started."

# ──────────────────────────────────────────────
# 2. Deploy B: patroni cluster + etcd(3) + pgbouncer + exporter
# ──────────────────────────────────────────────
log "=== DEPLOYING B (patroni cluster) ==="

export CLUSTER_NAME=test-b
export ETCD_INITIAL_CLUSTER="etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380,etcd-3=http://etcd-3:2380"
export ETCD_HOSTS="etcd-1:2379,etcd-2:2379,etcd-3:2379"
export PATRONI1_PG_PORT=0
export PATRONI1_API_PORT=0
export PATRONI2_PG_PORT=0
export PATRONI2_API_PORT=0
export PATRONI3_PG_PORT=0
export PATRONI3_API_PORT=0
export ETCD1_CLIENT_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=patroni-1
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=patroni-1

docker compose -p test-b -f "$COMPOSE_FILE" \
    --profile cluster --profile etcd --profile etcd-ha --profile pgbouncer --profile postgres-exporter \
    up -d --build --wait --wait-timeout 180

log "Deployment B started."

# ──────────────────────────────────────────────
# 2.1 Deploy C: orioledb standalone
# ──────────────────────────────────────────────
log "=== DEPLOYING C (orioledb) ==="

export CLUSTER_NAME=test-oriole
export SHARED_NETWORK
export PG_IMAGE=orioledb/orioledb:latest-pg17
export PG_EXTRA_PARAMS="-c shared_preload_libraries=orioledb -c listen_addresses=*"
export PG_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=postgres
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=postgres

docker compose -p test-oriole -f "$COMPOSE_FILE" \
    --profile standalone --profile pgbouncer --profile postgres-exporter \
    up -d --build --wait --wait-timeout 120

docker exec test-oriole-postgres psql -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';"
docker exec test-oriole-postgres psql -U postgres -c "CREATE EXTENSION IF NOT EXISTS orioledb;"

log "Deployment C started."

# ──────────────────────────────────────────────
# 2.2 Deploy A2/B2/C2: second wave on same VM
# ──────────────────────────────────────────────
log "=== DEPLOYING SECOND WAVE (A2/B2/C2) ==="

export CLUSTER_NAME=test-a2
export SHARED_NETWORK
export PG_IMAGE=postgres:17
export PG_EXTRA_PARAMS=
export PG_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=postgres
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=postgres

docker compose -p test-a2 -f "$COMPOSE_FILE" \
    --profile standalone --profile pgbouncer --profile postgres-exporter \
    up -d --build --wait --wait-timeout 120

docker exec test-a2-postgres psql -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';"

export CLUSTER_NAME=test-b2
export ETCD_INITIAL_CLUSTER="etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380,etcd-3=http://etcd-3:2380"
export ETCD_HOSTS="etcd-1:2379,etcd-2:2379,etcd-3:2379"
export PATRONI1_PG_PORT=0
export PATRONI1_API_PORT=0
export PATRONI2_PG_PORT=0
export PATRONI2_API_PORT=0
export PATRONI3_PG_PORT=0
export PATRONI3_API_PORT=0
export ETCD1_CLIENT_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=patroni-1
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=patroni-1

docker compose -p test-b2 -f "$COMPOSE_FILE" \
    --profile cluster --profile etcd --profile etcd-ha --profile pgbouncer --profile postgres-exporter \
    up -d --build

export CLUSTER_NAME=test-oriole2
export SHARED_NETWORK
export PG_IMAGE=orioledb/orioledb:latest-pg17
export PG_EXTRA_PARAMS="-c shared_preload_libraries=orioledb -c listen_addresses=*"
export PG_PORT=0
export PGBOUNCER_PORT=0
export PGBOUNCER_UPSTREAM_HOST=postgres
export PG_EXPORTER_PORT=0
export PG_EXPORTER_TARGET=postgres

docker compose -p test-oriole2 -f "$COMPOSE_FILE" \
    --profile standalone --profile pgbouncer --profile postgres-exporter \
    up -d --build --wait --wait-timeout 120

docker exec test-oriole2-postgres psql -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';"
docker exec test-oriole2-postgres psql -U postgres -c "CREATE EXTENSION IF NOT EXISTS orioledb;"

log "Second wave started."

# Force MD5 for pgbouncer compatibility on Patroni
# We do this after 'up' but we need to know the leader.
# Actually, we can just do it on any node if it's primary, 
# or wait until leader is found in section 4.

# ──────────────────────────────────────────────
# 3. Verify Deployment A
# ──────────────────────────────────────────────
log "=== VERIFYING DEPLOYMENT A ==="

# 3.1 Standalone postgres is reachable (check from inside the container)
if pg_isready_in test-a-postgres localhost 5432 postgres; then
    ok "A: pg_isready on standalone postgres"
else
    fail "A: pg_isready on standalone postgres"
fi

# 3.2 SQL query
RESULT_A=$(psql_in test-a-postgres localhost 5432 "SELECT 1")
if [ "$RESULT_A" = "1" ]; then
    ok "A: SQL SELECT 1 via standalone postgres"
else
    fail "A: SQL SELECT 1 via standalone postgres (got: '$RESULT_A')"
fi

# 3.3 PgBouncer works (check from pgbouncer container connecting back to postgres)
RESULT_PGB_A=$(psql_in test-a-pgbouncer pgbouncer 6432 "SELECT 1")
if [ "$RESULT_PGB_A" = "1" ]; then
    ok "A: SQL via pgbouncer"
else
    fail "A: SQL via pgbouncer (got: '$RESULT_PGB_A')"
fi

# 3.4 Postgres exporter serves metrics
if docker exec test-a-postgres-exporter sh -c "wget -qO- http://localhost:9187/metrics | grep -q 'pg_up 1'" >/dev/null 2>&1; then
    ok "A: postgres-exporter metrics (pg_up=1)"
else
    fail "A: postgres-exporter metrics"
fi

# ──────────────────────────────────────────────
# 4. Verify Deployment B
# ──────────────────────────────────────────────
log "=== VERIFYING DEPLOYMENT B ==="

# 4.1 Etcd cluster health
ETCD_HEALTH=$(docker exec test-b-etcd-1 etcdctl endpoint health --endpoints=http://etcd-1:2379,http://etcd-2:2379,http://etcd-3:2379 2>&1 || echo "error")
HEALTHY_COUNT=$(echo "$ETCD_HEALTH" | grep -c "is healthy" || true)
if [ "$HEALTHY_COUNT" -eq 3 ]; then
    ok "B: etcd 3-node cluster healthy"
else
    fail "B: etcd cluster health (healthy=$HEALTHY_COUNT/3)"
    echo "  $ETCD_HEALTH"
fi

# 4.2 Patroni leader elected
sleep 5
LEADER=""
LEADER_NODE=""
for node in 1 2 3; do
    IN_RECOVERY=$(psql_in "test-b-patroni-${node}" localhost 5432 "SELECT pg_is_in_recovery()")
    if [ "$IN_RECOVERY" = "f" ]; then
        LEADER="patroni-${node}"
        LEADER_NODE="$node"
        break
    fi
done
if [ -n "$LEADER" ]; then
    ok "B: patroni leader elected (node $LEADER_NODE)"
    # Force MD5 for pgbouncer compatibility
    LEADER_CONTAINER="test-b-patroni-${LEADER_NODE}"
    docker exec "$LEADER_CONTAINER" psql -U postgres -c "ALTER SYSTEM SET password_encryption = 'md5';" >/dev/null 2>&1 || true
    docker exec "$LEADER_CONTAINER" psql -U postgres -c "SELECT pg_reload_conf();" >/dev/null 2>&1 || true
    docker exec "$LEADER_CONTAINER" psql -U postgres -c "ALTER USER postgres WITH PASSWORD 'postgres';" >/dev/null 2>&1 || true
else
    fail "B: patroni leader not found"
    for node in 1 2 3; do
        RECOVERY_STATE=$(psql_in "test-b-patroni-${node}" localhost 5432 "SELECT pg_is_in_recovery()")
        echo "  node=$node recovery=$RECOVERY_STATE"
    done
fi

# 4.3 Patroni cluster members
MEMBER_COUNT=0
for node in 1 2 3; do
    if pg_isready_in "test-b-patroni-${node}" localhost 5432 postgres; then
        MEMBER_COUNT=$((MEMBER_COUNT + 1))
    fi
done
if [ "$MEMBER_COUNT" -ge 2 ]; then
    ok "B: patroni cluster has $MEMBER_COUNT members"
else
    fail "B: patroni cluster members (got: $MEMBER_COUNT)"
fi

# 4.4 Can connect to patroni leader via pg_isready (inside container)
LEADER_CONTAINER="test-b-patroni-${LEADER_NODE:-1}"
if pg_isready_in "$LEADER_CONTAINER" localhost 5432 postgres; then
    ok "B: pg_isready on patroni leader ($LEADER_CONTAINER)"
else
    fail "B: pg_isready on patroni leader ($LEADER_CONTAINER)"
fi

# 4.5 SQL query through patroni leader
RESULT_B=$(psql_in "$LEADER_CONTAINER" localhost 5432 "SELECT 1")
if [ "$RESULT_B" = "1" ]; then
    ok "B: SQL SELECT 1 via patroni leader"
else
    fail "B: SQL SELECT 1 via patroni leader (got: '$RESULT_B')"
fi

# 4.6 Replication check - find leader and a replica
REPLICA_NODE=""
for node in 1 2 3; do
    if [ "$node" != "${LEADER_NODE:-0}" ]; then
        IN_RECOVERY=$(psql_in "test-b-patroni-${node}" localhost 5432 "SELECT pg_is_in_recovery()")
        if [ "$IN_RECOVERY" = "t" ]; then
            REPLICA_NODE="$node"
            break
        fi
    fi
done

if [ -n "$LEADER_NODE" ] && [ -n "$REPLICA_NODE" ]; then
    LEADER_C="test-b-patroni-${LEADER_NODE}"
    REPLICA_C="test-b-patroni-${REPLICA_NODE}"
    # Write to leader
    psql_in "$LEADER_C" localhost 5432 "CREATE TABLE IF NOT EXISTS repl_test(v int); INSERT INTO repl_test VALUES(42);" >/dev/null 2>&1 || true
    sleep 3
    # Read from replica
    REPL_VAL=$(psql_in "$REPLICA_C" localhost 5432 "SELECT v FROM repl_test LIMIT 1")
    if [ "$REPL_VAL" = "42" ]; then
        ok "B: replication works (leader=patroni-$LEADER_NODE -> replica=patroni-$REPLICA_NODE)"
    else
        fail "B: replication check (got: '$REPL_VAL' from replica=patroni-$REPLICA_NODE)"
    fi
else
    fail "B: could not determine leader/replica (leader=$LEADER_NODE, replica=$REPLICA_NODE)"
fi

# 4.7 PgBouncer on cluster (check from pgbouncer container)
RESULT_PGB_B=""
for _ in $(seq 1 10); do
    RESULT_PGB_B=$(psql_in test-b-pgbouncer pgbouncer 6432 "SELECT 1")
    if [ "$RESULT_PGB_B" = "1" ]; then
        break
    fi
    sleep 2
done
if [ "$RESULT_PGB_B" = "1" ]; then
    ok "B: SQL via pgbouncer"
else
    fail "B: SQL via pgbouncer (got: '$RESULT_PGB_B')"
fi

# 4.8 Postgres exporter on cluster
if docker exec test-b-postgres-exporter sh -c "wget -qO- http://localhost:9187/metrics | grep -q 'pg_up 1'" >/dev/null 2>&1; then
    ok "B: postgres-exporter metrics (pg_up=1)"
else
    fail "B: postgres-exporter metrics"
fi

# ──────────────────────────────────────────────
# 4.1 Verify Deployment C (OrioleDB)
# ──────────────────────────────────────────────
log "=== VERIFYING DEPLOYMENT C (OrioleDB) ==="

# 4.1.1 OrioleDB postgres is reachable
if pg_isready_in test-oriole-postgres localhost 5432 postgres; then
    ok "C: pg_isready on orioledb postgres"
else
    fail "C: pg_isready on orioledb postgres"
fi

# 4.1.2 SQL query and extension check
RESULT_C=$(psql_in test-oriole-postgres localhost 5432 "SELECT 1")
if [ "$RESULT_C" = "1" ]; then
    ok "C: SQL SELECT 1 via orioledb postgres"
else
    fail "C: SQL SELECT 1 via orioledb postgres (got: '$RESULT_C')"
fi

ORIOLE_EXT=$(psql_in test-oriole-postgres localhost 5432 "SELECT count(*) FROM pg_extension WHERE extname = 'orioledb'")
if [ "$ORIOLE_EXT" = "1" ]; then
    ok "C: orioledb extension installed"
else
    fail "C: orioledb extension NOT installed (got: '$ORIOLE_EXT')"
fi

# 4.1.2.1 OrioleDB table creation
docker exec test-oriole-postgres psql -U postgres -d postgres -c "DROP TABLE IF EXISTS o_test;"
docker exec test-oriole-postgres psql -U postgres -d postgres -c "CREATE TABLE o_test(id int) USING orioledb;"
docker exec test-oriole-postgres psql -U postgres -d postgres -c "INSERT INTO o_test VALUES(1);"
ORIOLE_TABLE=$(psql_in test-oriole-postgres localhost 5432 "SELECT count(*) FROM o_test;")
if [ "$ORIOLE_TABLE" = "1" ]; then
    ok "C: OrioleDB table created and queried"
else
    fail "C: OrioleDB table creation failed (got: '$ORIOLE_TABLE')"
fi

# 4.1.3 PgBouncer works
RESULT_PGB_C=$(psql_in test-oriole-pgbouncer pgbouncer 6432 "SELECT 1")
if [ "$RESULT_PGB_C" = "1" ]; then
    ok "C: SQL via pgbouncer"
else
    fail "C: SQL via pgbouncer (got: '$RESULT_PGB_C')"
fi

# 4.1.4 Postgres exporter serves metrics
if docker exec test-oriole-postgres-exporter sh -c "wget -qO- http://localhost:9187/metrics | grep -q 'pg_up 1'" >/dev/null 2>&1; then
    ok "C: postgres-exporter metrics (pg_up=1)"
else
    fail "C: postgres-exporter metrics"
fi

# ──────────────────────────────────────────────
# 5. Cross-deployment connectivity via shared network
# ──────────────────────────────────────────────
log "=== CROSS-DEPLOYMENT CONNECTIVITY ==="

# From deployment B (patroni-1), connect to deployment A (standalone postgres)
CROSS_RESULT=$(psql_in test-b-patroni-1 test-a-postgres 5432 "SELECT 42")
if [ "$CROSS_RESULT" = "42" ]; then
    ok "Cross: B(patroni-1) -> A(standalone) via $SHARED_NETWORK"
else
    fail "Cross: B(patroni-1) -> A(standalone) (got: '$CROSS_RESULT')"
fi

# From deployment A (postgres), connect to deployment B (patroni leader)
CROSS_RESULT2=$(psql_in test-a-postgres "test-b-patroni-${LEADER_NODE:-1}" 5432 "SELECT 99")
if [ "$CROSS_RESULT2" = "99" ]; then
    ok "Cross: A(standalone) -> B(patroni-$LEADER_NODE) via $SHARED_NETWORK"
else
    fail "Cross: A(standalone) -> B(patroni-$LEADER_NODE) (got: '$CROSS_RESULT2')"
fi

# From deployment B (patroni-1), connect to deployment C (orioledb)
CROSS_RESULT3=$(psql_in test-b-patroni-1 test-oriole-postgres 5432 "SELECT 101")
if [ "$CROSS_RESULT3" = "101" ]; then
    ok "Cross: B(patroni-1) -> C(orioledb) via $SHARED_NETWORK"
else
    fail "Cross: B(patroni-1) -> C(orioledb) (got: '$CROSS_RESULT3')"
fi

# From deployment C (orioledb), connect to deployment A (standalone)
CROSS_RESULT4=$(psql_in test-oriole-postgres test-a-postgres 5432 "SELECT 202")
if [ "$CROSS_RESULT4" = "202" ]; then
    ok "Cross: C(orioledb) -> A(standalone) via $SHARED_NETWORK"
else
    fail "Cross: C(orioledb) -> A(standalone) (got: '$CROSS_RESULT4')"
fi

# ──────────────────────────────────────────────
# 6. Verify second wave and cross-wave connectivity
# ──────────────────────────────────────────────
log "=== VERIFYING SECOND WAVE (A2/B2/C2) ==="

RESULT_A2=$(psql_in test-a2-postgres localhost 5432 "SELECT 1")
if [ "$RESULT_A2" = "1" ]; then
    ok "A2: SQL SELECT 1 via standalone postgres"
else
    fail "A2: SQL SELECT 1 via standalone postgres (got: '$RESULT_A2')"
fi

ETCD_HEALTH2=""
HEALTHY_COUNT2=0
for _ in $(seq 1 30); do
    ETCD_HEALTH2=$(docker exec test-b2-etcd-1 etcdctl endpoint health --endpoints=http://etcd-1:2379,http://etcd-2:2379,http://etcd-3:2379 2>&1 || echo "error")
    HEALTHY_COUNT2=$(echo "$ETCD_HEALTH2" | grep -c "is healthy" || true)
    if [ "$HEALTHY_COUNT2" -eq 3 ]; then
        break
    fi
    sleep 2
done
if [ "$HEALTHY_COUNT2" -eq 3 ]; then
    ok "B2: etcd 3-node cluster healthy"
else
    fail "B2: etcd cluster health (healthy=$HEALTHY_COUNT2/3)"
fi

LEADER2=""
LEADER_NODE2=""
for _ in $(seq 1 30); do
    for node in 1 2 3; do
        IN_RECOVERY=$(psql_in "test-b2-patroni-${node}" localhost 5432 "SELECT pg_is_in_recovery()")
        if [ "$IN_RECOVERY" = "f" ]; then
            LEADER2="patroni-${node}"
            LEADER_NODE2="$node"
            break
        fi
    done
    if [ -n "$LEADER2" ]; then
        break
    fi
    sleep 2
done
if [ -n "$LEADER2" ]; then
    ok "B2: patroni leader elected (node $LEADER_NODE2)"
else
    fail "B2: patroni leader not found"
fi

RESULT_C2=$(psql_in test-oriole2-postgres localhost 5432 "SELECT 1")
if [ "$RESULT_C2" = "1" ]; then
    ok "C2: SQL SELECT 1 via orioledb postgres"
else
    fail "C2: SQL SELECT 1 via orioledb postgres (got: '$RESULT_C2')"
fi

ORIOLE_EXT2=$(psql_in test-oriole2-postgres localhost 5432 "SELECT count(*) FROM pg_extension WHERE extname = 'orioledb'")
if [ "$ORIOLE_EXT2" = "1" ]; then
    ok "C2: orioledb extension installed"
else
    fail "C2: orioledb extension NOT installed (got: '$ORIOLE_EXT2')"
fi

log "=== CROSS-WAVE CONNECTIVITY ==="

CROSS_WAVE_1=$(psql_in test-b-patroni-1 test-a2-postgres 5432 "SELECT 303")
if [ "$CROSS_WAVE_1" = "303" ]; then
    ok "Wave1->Wave2: B -> A2 connectivity"
else
    fail "Wave1->Wave2: B -> A2 connectivity (got: '$CROSS_WAVE_1')"
fi

B2_SRC_CONTAINER="test-b2-patroni-${LEADER_NODE2:-1}"

CROSS_WAVE_2=""
for _ in $(seq 1 10); do
    CROSS_WAVE_2=$(psql_in "$B2_SRC_CONTAINER" test-a-postgres 5432 "SELECT 404")
    if [ "$CROSS_WAVE_2" = "404" ]; then
        break
    fi
    sleep 2
done
if [ "$CROSS_WAVE_2" = "404" ]; then
    ok "Wave2->Wave1: B2 -> A connectivity"
else
    fail "Wave2->Wave1: B2 -> A connectivity (got: '$CROSS_WAVE_2')"
fi

CROSS_WAVE_3=$(psql_in test-b-patroni-1 test-oriole2-postgres 5432 "SELECT 505")
if [ "$CROSS_WAVE_3" = "505" ]; then
    ok "Wave1->Wave2: B -> C2 connectivity"
else
    fail "Wave1->Wave2: B -> C2 connectivity (got: '$CROSS_WAVE_3')"
fi

CROSS_WAVE_4=""
for _ in $(seq 1 10); do
    CROSS_WAVE_4=$(psql_in "$B2_SRC_CONTAINER" test-oriole-postgres 5432 "SELECT 606")
    if [ "$CROSS_WAVE_4" = "606" ]; then
        break
    fi
    sleep 2
done
if [ "$CROSS_WAVE_4" = "606" ]; then
    ok "Wave2->Wave1: B2 -> C connectivity"
else
    fail "Wave2->Wave1: B2 -> C connectivity (got: '$CROSS_WAVE_4')"
fi

CROSS_WAVE_5=$(psql_in test-oriole2-postgres test-a2-postgres 5432 "SELECT 707")
if [ "$CROSS_WAVE_5" = "707" ]; then
    ok "Wave2 internal: C2 -> A2 connectivity"
else
    fail "Wave2 internal: C2 -> A2 connectivity (got: '$CROSS_WAVE_5')"
fi

CROSS_WAVE_6=$(psql_in test-a2-postgres "test-b2-patroni-${LEADER_NODE2:-1}" 5432 "SELECT 808")
if [ "$CROSS_WAVE_6" = "808" ]; then
    ok "Wave2 internal: A2 -> B2 connectivity"
else
    fail "Wave2 internal: A2 -> B2 connectivity (got: '$CROSS_WAVE_6')"
fi

# ──────────────────────────────────────────────
# 7. Summary
# ──────────────────────────────────────────────
echo ""
log "=== RESULTS ==="
echo -e "  ${GREEN}PASS: $PASS${NC}"
echo -e "  ${RED}FAIL: $FAIL${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
    log "Some tests failed."
    exit 1
fi

log "All tests passed!"
exit 0
