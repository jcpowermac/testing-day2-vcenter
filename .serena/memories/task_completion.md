# Task Completion Checklist

Run before considering a coding task done:

1. `make build` — compiles cleanly
2. `make vet` — no warnings
3. Sync to remote and run `make test-readonly` against real cluster
4. Review failures — distinguish test bugs from product findings