# Core

QA/QE Ginkgo v2 e2e test suite for the OpenShift vSphere Multi-vCenter Day 2 feature.
Tests run against a live OpenShift cluster via `KUBECONFIG`.

- Feature gate: `VSphereMultiVCenterDay2`
- Readonly tests use server-side dry-run for xValidation (CEL) checks; no write attempts
- Mutating tests backup/restore cluster state; VAP denial tests use real patches (denied = no mutation)
- Multi-vCenter tests skip on single-vCenter clusters (`requireMultiVCenter()` guard)
- See `mem:tech_stack` for language/tooling details
- See `mem:conventions` for code style
- See `mem:testing` for test execution and remote workflow
- See `mem:cluster_state` for current test cluster details
- See `mem:vap_findings` for known VAP issues found during QA