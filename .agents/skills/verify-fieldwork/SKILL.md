---
name: verify-fieldwork
description: Verify the Fieldwork Market storefront, catalog, CSV import, search and simulated checkout against a real isolated Docker Compose stack.
---

# Verify Fieldwork Market

## Launch

From the project root, run `sh .agents/skills/verify-fieldwork/scripts/verify.sh`. The driver builds and starts a new Compose project, waits for container health, imports the original CSV through the API, then checks the HTTP journeys. The PostgreSQL entry point creates service databases and Go applies schemas at startup. Use `SHOP_PORT=18082` if the verifier's loopback port `18081` is occupied.

For a browser session, keep the stack available until the journey is complete.

```sh
verify_project="fieldwork-browser-$(date +%s)-$$"
SHOP_PORT=18082 docker compose -f compose.yaml -p "${verify_project:?Verify project is required}" up --build -d --wait
```

Open `http://localhost:18082`. The first page is empty until you import the sample. The project and port belong to this shell session.

## Doctor

Run `sh .agents/skills/verify-fieldwork/scripts/verify.sh doctor` from the repository root. It checks Compose syntax, Docker engine access, Python and the fixture. For a browser session, also check that `docker compose -f compose.yaml -p "${verify_project:?Verify project is required}" ps` shows eight healthy containers, and that the web port belongs to this project. The local app opens directly without sign-in.

## Drive

Use the [feature map](features/README.md) for exact selectors and observable pass states. From a new stack, import the fixture through **Manage catalog**. Then search for **Wireless Mouse** and start a pending order, choose a result in the simulator tab, and check the Orders screen. The automated script drives all three API surfaces through the production Nginx gateway.

## Evidence

The driver prints the path to a directory containing `doctor.txt`, `compose-up.log`, `smoke.log`, `containers.txt`, `services.log`, and `cleanup.log`. A passing `smoke.log` has six `PASS` lines covering Next.js routes and script policy, literal fixture names, CRUD, pending orders, payment replay, compensation, and deletion. For a browser run, save the UI state and an order receipt or CSV report to a directory outside the Compose project. Confirm the stored order, returned stock or visible search result where the feature map calls for it.

## Cleanup

The driver removes the containers, network and volumes for the uniquely named project it launched. It verifies that no resources with that Compose project label remain and keeps the logs. For the manual browser instance, run `docker compose -f compose.yaml -p "${verify_project:?Verify project is required}" down --volumes` in the same shell, then verify `docker compose -f compose.yaml -p "${verify_project:?Verify project is required}" ps -a` shows no containers and your evidence is still present.

## Helpers

- `scripts/verify.sh` controls isolation, readiness, evidence and cleanup.
- `../../../scripts/smoke.py` from this skill's directory checks the API flows. Run it with Python 3.
