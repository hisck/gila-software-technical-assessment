#!/bin/sh
set -eu

repository=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
cd "$repository"

doctor() {
    docker compose -f "$repository/compose.yaml" config --quiet
    docker compose version
    docker version --format 'Docker Engine {{.Server.Version}}'
    python3 --version
    test -r scripts/smoke.py
    test -r "Code Challenge E-Commerce.csv"
}

case "${1:-run}" in
    doctor)
        doctor
        exit
        ;;
    run)
        ;;
    *)
        printf 'Usage: %s [doctor|run]\n' "$0" >&2
        exit 2
        ;;
esac

evidence=$(mktemp -d "${TMPDIR:-/tmp}/fieldwork-verify.XXXXXX")
verify_project=fieldwork-verify-$(printf '%s' "${evidence##*.}" | tr '[:upper:]' '[:lower:]')-$$
export SHOP_PORT="${SHOP_PORT:-18081}"
printf 'Evidence directory %s\n' "$evidence"

compose() {
    docker compose -f "$repository/compose.yaml" -p "$verify_project" "$@"
}

project_resources() {
    label="label=com.docker.compose.project=$verify_project"
    docker ps -aq --filter "$label" &&
        docker volume ls -q --filter "$label" &&
        docker network ls -q --filter "$label"
}

cleanup() {
    status=$?
    trap - 0 HUP INT TERM
    compose ps -a >"$evidence/containers.txt" 2>&1 || true
    compose logs --no-color >"$evidence/services.log" 2>&1 || true
    if compose down --volumes >"$evidence/cleanup.log" 2>&1 &&
        remaining=$(project_resources) && test -z "$remaining"; then
        printf 'PASS verification project resources removed\n' | tee -a "$evidence/cleanup.log"
    else
        printf 'Cleanup requires attention for %s\n' "$verify_project" >&2
        cat "$evidence/cleanup.log" >&2
        status=1
    fi
    exit "$status"
}

if ! doctor >"$evidence/doctor.txt" 2>&1; then
    cat "$evidence/doctor.txt" >&2
    exit 1
fi
remaining=$(project_resources)
if test -n "$remaining"; then
    printf 'Verification project name already in use: %s\n' "$verify_project" >&2
    exit 1
fi

trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

if ! compose up --build -d --wait --wait-timeout 90 >"$evidence/compose-up.log" 2>&1; then
    tail -n 50 "$evidence/compose-up.log" >&2
    exit 1
fi
if ! python3 scripts/smoke.py --base-url "http://127.0.0.1:$SHOP_PORT" --sample >"$evidence/smoke.log" 2>&1; then
    cat "$evidence/smoke.log" >&2
    exit 1
fi
cat "$evidence/smoke.log"
