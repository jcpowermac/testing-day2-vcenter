#!/bin/bash
#
# test-grid.sh — run the full test-e2e lifecycle N times on fresh clusters
#
# Usage:
#   ./hack/test-grid.sh --release-image <image> [--runs 10] [--install-dir <path>]
#
# Each run: create cluster → make test-e2e → collect JUnit → destroy cluster.
# Runs with existing JUnit XMLs in results/run-NN/ are skipped (resume support).
# After all runs, generates a markdown + HTML grid via cmd/aggregate-junit.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"

RELEASE_IMAGE=""
RUNS=10
INSTALL_DIR="$HOME/before-installer-testing/vsphere-ipi"
INSTALL_CONFIG_TEMPLATE="install-config-backup-dev.yaml"
RESULTS_DIR="$TEST_REPO/results"

usage() {
    cat <<EOF
Usage: $0 --release-image <image> [options]

Options:
  --release-image <image>   Required. OCP release image to install.
  --runs <n>                Number of runs (default: 10).
  --install-dir <path>      openshift-install working directory
                            (default: ~/before-installer-testing/vsphere-ipi).
  --install-config <file>   Install config template filename in install-dir
                            (default: install-config-backup-dev.yaml).
  --results-dir <path>      Where to store per-run results (default: <repo>/results).
  -h, --help                Show this help.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --release-image) RELEASE_IMAGE="$2"; shift 2 ;;
        --runs)          RUNS="$2"; shift 2 ;;
        --install-dir)   INSTALL_DIR="$2"; shift 2 ;;
        --install-config) INSTALL_CONFIG_TEMPLATE="$2"; shift 2 ;;
        --results-dir)   RESULTS_DIR="$2"; shift 2 ;;
        -h|--help)       usage ;;
        *)               echo "Unknown option: $1"; usage ;;
    esac
done

if [[ -z "$RELEASE_IMAGE" ]]; then
    echo "Error: --release-image is required"
    usage
fi

INSTALLER_BIN="$(cd "$INSTALL_DIR/.." && pwd)/openshift-install"
if [[ ! -x "$INSTALLER_BIN" ]]; then
    echo "Error: openshift-install not found at $INSTALLER_BIN"
    exit 1
fi

if [[ ! -f "$INSTALL_DIR/$INSTALL_CONFIG_TEMPLATE" ]]; then
    echo "Error: install config template not found at $INSTALL_DIR/$INSTALL_CONFIG_TEMPLATE"
    exit 1
fi

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

mkdir -p "$RESULTS_DIR"

log "=== Test Grid: $RUNS runs with $RELEASE_IMAGE ==="
log "Install dir:  $INSTALL_DIR"
log "Test repo:    $TEST_REPO"
log "Results dir:  $RESULTS_DIR"

GRID_START=$(date +%s)
PASSED_RUNS=0
FAILED_RUNS=0
SKIPPED_RUNS=0

for i in $(seq 1 "$RUNS"); do
    RUN_LABEL=$(printf "run-%02d" "$i")
    RUN_DIR="$RESULTS_DIR/$RUN_LABEL"
    mkdir -p "$RUN_DIR"

    # Resume support: skip if this run already has JUnit XMLs
    if ls "$RUN_DIR"/*.xml >/dev/null 2>&1; then
        log "=== $RUN_LABEL: already has results, skipping ==="
        SKIPPED_RUNS=$((SKIPPED_RUNS + 1))
        continue
    fi

    log "=== $RUN_LABEL: starting ($i/$RUNS) ==="
    RUN_START=$(date +%s)
    run_rc=0

    # --- Clean install directory ---
    log "$RUN_LABEL: cleaning install directory"
    cd "$INSTALL_DIR"
    # Remove previous cluster state but keep the template configs and installer
    find . -maxdepth 1 \
        ! -name '.' \
        ! -name 'install-config-backup*' \
        ! -name '*.example' \
        ! -name '*.sh' \
        -exec rm -rf {} + 2>/dev/null || true

    # --- Create cluster ---
    log "$RUN_LABEL: creating cluster"
    cp "$INSTALL_CONFIG_TEMPLATE" install-config.yaml
    export OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="$RELEASE_IMAGE"

    if "$INSTALLER_BIN" create cluster --log-level debug 2>&1 | tee "$RUN_DIR/install.log"; then
        log "$RUN_LABEL: cluster created successfully"
    else
        log "$RUN_LABEL: cluster creation FAILED (rc=$?)"
        echo "INSTALL_FAILED" > "$RUN_DIR/status"
        FAILED_RUNS=$((FAILED_RUNS + 1))
        # Try to destroy whatever was created
        "$INSTALLER_BIN" destroy cluster --log-level debug 2>&1 | tee "$RUN_DIR/destroy.log" || true
        continue
    fi

    export KUBECONFIG="$INSTALL_DIR/auth/kubeconfig"

    # --- Run tests ---
    log "$RUN_LABEL: running test-e2e"
    cd "$TEST_REPO"
    # Pull latest test code
    git pull --ff-only 2>/dev/null || true

    test_rc=0
    make test-e2e 2>&1 | tee "$RUN_DIR/test-e2e.log" || test_rc=$?

    # --- Collect JUnit XMLs ---
    if ls reports/*.xml >/dev/null 2>&1; then
        cp reports/*.xml "$RUN_DIR/"
        log "$RUN_LABEL: collected $(ls "$RUN_DIR"/*.xml 2>/dev/null | wc -l) JUnit XML files"
    else
        log "$RUN_LABEL: WARNING — no JUnit XML files produced"
    fi

    # Record test exit code
    echo "$test_rc" > "$RUN_DIR/test-exit-code"
    if [[ $test_rc -eq 0 ]]; then
        echo "PASS" > "$RUN_DIR/status"
        PASSED_RUNS=$((PASSED_RUNS + 1))
    else
        echo "FAIL" > "$RUN_DIR/status"
        FAILED_RUNS=$((FAILED_RUNS + 1))
    fi

    # Clean reports for next run
    rm -f reports/*.xml

    # --- Destroy cluster ---
    log "$RUN_LABEL: destroying cluster"
    cd "$INSTALL_DIR"
    "$INSTALLER_BIN" destroy cluster --log-level debug 2>&1 | tee "$RUN_DIR/destroy.log" || true

    RUN_END=$(date +%s)
    RUN_DURATION=$(( RUN_END - RUN_START ))
    log "$RUN_LABEL: finished in $(( RUN_DURATION / 60 ))m$(( RUN_DURATION % 60 ))s (test rc=$test_rc)"
    echo "$RUN_DURATION" > "$RUN_DIR/duration-seconds"
done

GRID_END=$(date +%s)
GRID_DURATION=$(( GRID_END - GRID_START ))

log "=== All runs complete ==="
log "Passed: $PASSED_RUNS  Failed: $FAILED_RUNS  Skipped (resumed): $SKIPPED_RUNS"
log "Total wall time: $(( GRID_DURATION / 3600 ))h$(( (GRID_DURATION % 3600) / 60 ))m"

# --- Generate grid report ---
log "Generating test grid..."
cd "$TEST_REPO"
go run ./cmd/aggregate-junit "$RESULTS_DIR"

log "Done. Results in $RESULTS_DIR/"
