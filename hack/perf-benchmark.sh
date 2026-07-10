#!/bin/bash
#
# perf-benchmark.sh — A/B provisioning performance comparison
#
# Usage:
#   ./hack/perf-benchmark.sh --baseline-image <image> --pr-image <image> [options]
#
# For each image: create cluster → make test-perf → collect results → destroy cluster.
# After both runs, generates a comparison report via cmd/perf-compare.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"

BASELINE_IMAGE=""
PR_IMAGE=""
INSTALL_DIR="$HOME/before-installer-testing/vsphere-ipi"
INSTALL_CONFIG_TEMPLATE="install-config-backup-dev.yaml"
RESULTS_DIR="$TEST_REPO/results/perf"
WORKER_COUNT=64

usage() {
    cat <<EOF
Usage: $0 --baseline-image <image> --pr-image <image> [options]

Options:
  --baseline-image <image>  Required. Stock OCP release image (baseline).
  --pr-image <image>        Required. OCP release image with MAO PR cherry-picked.
  --install-dir <path>      openshift-install working directory
                            (default: ~/before-installer-testing/vsphere-ipi).
  --install-config <file>   Install config template filename in install-dir
                            (default: install-config-backup-dev.yaml).
  --results-dir <path>      Where to store results (default: <repo>/results/perf).
  --worker-count <n>        Number of machines to provision in benchmark (default: 64).
  -h, --help                Show this help.
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --baseline-image)  BASELINE_IMAGE="$2"; shift 2 ;;
        --pr-image)        PR_IMAGE="$2"; shift 2 ;;
        --install-dir)     INSTALL_DIR="$2"; shift 2 ;;
        --install-config)  INSTALL_CONFIG_TEMPLATE="$2"; shift 2 ;;
        --results-dir)     RESULTS_DIR="$2"; shift 2 ;;
        --worker-count)    WORKER_COUNT="$2"; shift 2 ;;
        -h|--help)         usage ;;
        *)                 echo "Unknown option: $1"; usage ;;
    esac
done

if [[ -z "$BASELINE_IMAGE" ]]; then
    echo "Error: --baseline-image is required"
    usage
fi
if [[ -z "$PR_IMAGE" ]]; then
    echo "Error: --pr-image is required"
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

log "=== Perf Benchmark: A/B comparison ==="
log "Baseline image: $BASELINE_IMAGE"
log "PR image:       $PR_IMAGE"
log "Worker count:   $WORKER_COUNT"
log "Install dir:    $INSTALL_DIR"
log "Results dir:    $RESULTS_DIR"

OVERALL_START=$(date +%s)
OVERALL_RC=0

run_benchmark() {
    local variant="$1"
    local release_image="$2"
    local variant_dir="$RESULTS_DIR/$variant"
    mkdir -p "$variant_dir"

    # Resume support: skip if this variant already has perf-results.json
    if [[ -f "$variant_dir/perf-results.json" ]]; then
        log "=== $variant: already has results, skipping ==="
        return 0
    fi

    log "=== $variant: starting ==="
    local run_start
    run_start=$(date +%s)
    local run_rc=0

    # --- Clean install directory ---
    log "$variant: cleaning install directory"
    cd "$INSTALL_DIR"
    find . -maxdepth 1 \
        ! -name '.' \
        ! -name 'install-config-backup*' \
        ! -name '*.example' \
        ! -name '*.sh' \
        -exec rm -rf {} + 2>/dev/null || true

    # --- Create cluster ---
    log "$variant: creating cluster with $release_image"
    cp "$INSTALL_CONFIG_TEMPLATE" install-config.yaml
    export OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="$release_image"

    if "$INSTALLER_BIN" create cluster --log-level debug 2>&1 | tee "$variant_dir/install.log"; then
        log "$variant: cluster created successfully"
    else
        log "$variant: cluster creation FAILED"
        echo "INSTALL_FAILED" > "$variant_dir/status"
        "$INSTALLER_BIN" destroy cluster --log-level debug 2>&1 | tee "$variant_dir/destroy.log" || true
        return 1
    fi

    export KUBECONFIG="$INSTALL_DIR/auth/kubeconfig"

    # --- Run perf benchmark ---
    log "$variant: running perf benchmark (worker-count=$WORKER_COUNT)"
    cd "$TEST_REPO"
    git pull --ff-only 2>/dev/null || true

    local test_rc=0
    PERF_WORKER_COUNT="$WORKER_COUNT" PERF_RESULTS_DIR="$variant_dir" \
        make test-perf 2>&1 | tee "$variant_dir/test-perf.log" || test_rc=$?

    # --- Collect artifacts ---
    if ls reports/perf.xml >/dev/null 2>&1; then
        cp reports/perf.xml "$variant_dir/"
    fi
    rm -f reports/perf.xml

    echo "$test_rc" > "$variant_dir/test-exit-code"
    if [[ $test_rc -eq 0 ]]; then
        echo "PASS" > "$variant_dir/status"
    else
        echo "FAIL" > "$variant_dir/status"
        run_rc=1
    fi

    # --- Collect must-gather ---
    log "$variant: collecting must-gather"
    if command -v oc >/dev/null 2>&1; then
        local mg_dir="$variant_dir/must-gather"
        rm -rf "$mg_dir"
        if oc adm must-gather --dest-dir "$mg_dir" 2>&1 | tee "$variant_dir/must-gather.log"; then
            log "$variant: must-gather saved"
        else
            log "$variant: WARNING - must-gather failed"
        fi
    fi

    # --- Destroy cluster ---
    log "$variant: destroying cluster"
    cd "$INSTALL_DIR"
    "$INSTALLER_BIN" destroy cluster --log-level debug 2>&1 | tee "$variant_dir/destroy.log" || true

    local run_end
    run_end=$(date +%s)
    local run_duration=$(( run_end - run_start ))
    log "$variant: finished in $(( run_duration / 3600 ))h$(( (run_duration % 3600) / 60 ))m (test rc=$test_rc)"
    echo "$run_duration" > "$variant_dir/duration-seconds"

    return $run_rc
}

# --- Run baseline ---
run_benchmark "baseline" "$BASELINE_IMAGE" || OVERALL_RC=1

# --- Run PR ---
run_benchmark "pr" "$PR_IMAGE" || OVERALL_RC=1

# --- Generate comparison ---
BASELINE_JSON="$RESULTS_DIR/baseline/perf-results.json"
PR_JSON="$RESULTS_DIR/pr/perf-results.json"

if [[ -f "$BASELINE_JSON" ]] && [[ -f "$PR_JSON" ]]; then
    log "Generating comparison report..."
    cd "$TEST_REPO"
    go run ./cmd/perf-compare "$BASELINE_JSON" "$PR_JSON" --output "$RESULTS_DIR/comparison.txt" \
        2>&1 | tee "$RESULTS_DIR/compare.log"
    log "Comparison written to $RESULTS_DIR/comparison.txt"
else
    log "WARNING: cannot generate comparison — missing result files"
    [[ ! -f "$BASELINE_JSON" ]] && log "  missing: $BASELINE_JSON"
    [[ ! -f "$PR_JSON" ]] && log "  missing: $PR_JSON"
fi

OVERALL_END=$(date +%s)
OVERALL_DURATION=$(( OVERALL_END - OVERALL_START ))

log "=== Perf benchmark complete ==="
log "Total wall time: $(( OVERALL_DURATION / 3600 ))h$(( (OVERALL_DURATION % 3600) / 60 ))m"
log "Results in $RESULTS_DIR/"

exit $OVERALL_RC
