# Verification retries

| Task | Check | Initial result | Resolution |
| --- | --- | --- | --- |
| T01 | `go -C backend mod tidy` | The workspace sandbox blocked Go module proxy DNS. | Approved module download produced `go.sum`. Go compilation, unit tests and Docker builds succeeded. |
| T05 | `validate-spec-coverage.mjs ecommerce-base .` | The script requires a named work-item argument. | Re-ran with `--work-item ecommerce-base .`. The report recognized the changed Git root and all four spec files. |
| T05 | Python HTTP smoke suite | The workspace sandbox denied loopback sockets. | Re-ran against the approved isolated Compose stack outside the sandbox. All five groups passed. |
| T05 | Independent integrated review | A category based on an inherited object property could crash rendering. Stale quote guidance sent the buyer back to the old quote. | Restricted theme lookup to own properties, clarified price recovery and passed independent re-review. Chrome verified the category regression against Docker. |
| T06 | Independent runtime skill review | Implicit Compose discovery could pick up a shared-volume override. | Pinned the repository Compose file in startup and cleanup. A fresh run passed all smoke groups and verified cleanup of its own labeled resources. |
