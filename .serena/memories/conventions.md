# Conventions

- Standard Go conventions (effective Go, Go code review comments)
- `gofmt` formatting
- Ginkgo v2 style: `Describe`/`Context`/`It` with Labels for filtering
- Test IDs from test plan: N-INF-*, N-SEQ-*, N-CFG-* in test names
- `expectPatchRejected` (dry-run) accepts both xValidation and VAP errors via `SatisfyAny`
- `expectFailureDomainRemovalDenied` uses real patches (not dry-run) for VAP denial tests
- `expectPatchAllowedDryRun` fails clearly if VAP interferes — don't mask product issues
- `requireMultiVCenter()` guards tests meaningless on single-vCenter clusters
- ClusterOperator name is `config-operator` (NOT `cluster-config-operator`)
- Source ConfigMap key is `config`; managed/CCM ConfigMap key is `cloud.conf`
- Never write to CCO-managed secrets (`openshift-machine-api/vsphere-cloud-credentials`) — update root secrets and let CCO reconcile
- `CreateTestNamespace` waits for SCC `sa.scc.uid-range` annotation before returning — required for pod creation on OpenShift
- CSI test AfterAll force-deletes orphaned Machines if MachineSet drain times out, preventing VAP from blocking restore
- `pkg/vsphere` tag helpers (`AttachTagToDatastore`, `DetachTagFromDatastore`, `IsDatastoreTagged`) all accept a tag *name*, not ID, and resolve internally — keep new tag helpers consistent with this
- `git commit` requires GPG signing and pinentry pops up a GUI passphrase prompt on the user's screen (DISPLAY/DBUS are set correctly in the shell). Foreground `git commit` calls time out waiting on this prompt. Run the commit with `block_until_ms: 0` (backgrounded) and tell the user to enter their passphrase in the popup, then poll/await the shell — do not add `--no-gpg-sign` to work around it