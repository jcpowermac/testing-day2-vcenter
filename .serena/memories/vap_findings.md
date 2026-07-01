# VAP Findings (2026-06-29)

## Machine VAP blocks all Infrastructure updates when labels don't match FDs
The `vsphere-failure-domain-in-use-by-machine` VAP checks Machine labels (`machine.openshift.io/region`, `machine.openshift.io/zone`) against the proposed Infrastructure spec's failure domains. If ANY Machine has region/zone labels without a corresponding FD in the spec, the VAP denies the update.

On the test cluster, all Machines have labels `mx-central/mx-central-1a` (from vCenter tags) but no Infrastructure FD defines those values. This blocks ALL Infrastructure updates — even identity patches or adding a new vCenter.

Key question: The VAP appears to check "do all Machine labels have matching FDs in the new spec?" rather than "were any existing FDs removed that Machines reference?" The labels come from vCenter tags, so if tags are out of sync with Infrastructure FD region/zone definitions, all day-2 operations are blocked.

## CPMS VAP false positive on unrelated FD removal
The `vsphere-failure-domain-in-use-by-cpms` VAP reported that `us-east-1` would be removed when the test only removed `fd-vcenter2`. CPMS references `us-east-1` by name. Needs investigation — may be related to how JSON merge patch replaces the entire failureDomains array.

## Affected tests (3 failures)
1. "should allow adding a second vCenter via dry-run" — Machine VAP
2. "should allow patching unrelated Infrastructure fields via dry-run (ratcheting)" — Machine VAP
3. "should allow removing an unreferenced failure domain via dry-run" — CPMS VAP