# Active/Standby HA test suite

Every test that covers the cross-cluster Active/Standby HA controller
(issues #293-299), organized by what it verifies and how to run it. This
complements `docs/ha-runbook.md` (operator procedures for a live pair) with
the developer-facing view: what's tested, and what specific past bug each
regression test exists to catch.

The worker side of HA (worker-operator issues #467-469: endpoint
reconnection, connection-health conditions, robustness tests) has its own
test suite in the `worker-operator` repo and is out of scope for this
document.

## Layout

HA test coverage is layered. Each layer catches a different class of bug and
runs at a different cost:

| Layer | Location | Build tag | What it needs |
|---|---|---|---|
| Unit | `pkg/ha/*_test.go` | none | nothing (fake clients throughout) |
| Reconciler gate | `controllers/controller/leader_gate_test.go` | none | nothing (fake client) |
| End-to-end | `test/e2e/*_test.go` | `e2e` | `docker`, `kind`, `kubectl` on `PATH`; real, disposable Kind clusters |

Run everything except the e2e suite with:

```
go test -race ./pkg/ha/... ./controllers/controller/...
```

`-race` matters here: several of the most important tests in this suite
(`TestPromote_IsOnceOnly`, `TestPromote_ConcurrentAttemptsRunTheSequenceOnce`,
`TestKick_RespectsContextCancellation`) exist specifically to catch
concurrency bugs that only show up under `-race` or under repeated/shuffled
runs, not on a single ordinary pass.

`controllers/controller`'s own `TestAPIs` needs the envtest `etcd` binary and
fails on any machine without it. That failure predates and is unrelated to
`leader_gate_test.go`, which uses a fake client and needs nothing extra.

Run the end-to-end suite with:

```
make test-e2e-ha
```

which is `go test -tags e2e -v -timeout 30m ./test/e2e/...`. It creates two
disposable Kind clusters prefixed `e2e-ha-`, never touching any cluster
already on your machine, and deletes them when the test finishes (pass or
fail, via `t.Cleanup`). It does build a fresh controller image via `docker
build` and needs enough RAM to run two more Kind clusters alongside whatever
you already have up; stop any other local Kind clusters first if the machine
is memory-constrained.

---

## 1. Unit tests: `pkg/ha`

182 tests across 16 files, all against fake clients (`sigs.k8s.io/
controller-runtime/pkg/client/fake`). Tests marked **Regression** exist
because a specific bug reached a real cluster before the test was written;
they pin the fix so it can't silently regress.

### `mode_test.go`: parsing `--ha-mode` (`pkg/ha/mode.go`)

- `TestParseHAMode` - `"active"`/`"ACTIVE"` to Active, `" standby "` to
  Standby (trims whitespace), `"standalone"`/`""`/`"garbage"` all to
  Standalone. Lenient: fails open to standalone.
- `TestHAModeIsValid` - Active/Standby/Standalone are valid; an arbitrary
  string is not.
- `TestParseHAModeStrict_RejectsTypos` - `"stanby"`, `"activ"`, `"primary"`,
  `"true"`, `"STANDBYY"` are all rejected with an error.
- `TestParseHAModeStrict_AcceptsKnownModesAndEmpty` - empty/whitespace to
  Standalone, valid mode strings (any case, with surrounding whitespace) to
  the right mode, no error.
- **Regression**: the strict parser exists because standalone mode is
  unconditionally leader. A mistyped `--ha-mode` that fails open to
  standalone (the lenient parser's behavior) silently creates a second,
  unfenced writer against a shared cluster.

### `lease_test.go`: the HA Lease primitives (`pkg/ha/lease.go`)

Also defines shared test fixtures used package-wide: `testScheme`,
`fakeClient`, `failingWriteClient`, `testLog`, `newLease`.

- `TestIsLeaseStale` - fresh lease (renewed 2s ago) not stale; stale lease
  (60s ago) stale; missing `renewTime` stale; **nil lease stale**.
- `TestAcquireOrRenewLease_CreatesThenRenews` - the same holder renewing
  never moves `renewTime` backwards and never bumps `LeaseTransitions`.
- `TestAcquireOrRenewLease_TakeoverBumpsTransitions` - a new holder
  acquiring an existing lease increments `LeaseTransitions`.
- `TestAcquireOrRenewLease_SubSecondDurationClampsToOne` - **Regression**: a
  lease duration under 1s is clamped up to 1s instead of truncating to 0.
- `TestGetLease_NotFoundReturnsError` - a missing lease surfaces as an error,
  not a silently-zeroed value.

### `leader_elector_test.go`: `ClusterLeaderElector` (`pkg/ha/leader_elector.go`)

Construction, Active-side renewal, Standby-side watch, and detection under a
read failure (the "Active's API server is gone" path #297 needs).

- `TestNewClusterLeaderElector_StandaloneIsAlwaysLeader`,
  `_DefaultsToStandalone` - standalone (or an empty/unset mode) is
  unconditionally leader; the no-HA-regression guarantee.
- `TestNewClusterLeaderElector_LeaseNamespacePrefersDownwardAPIEnvVar`,
  `_LeaseNamespaceFallsBackWhenEnvVarUnset` -
  `KUBESLICE_CONTROLLER_MANAGER_NAMESPACE` wins when set; falls back to
  `DefaultLeaseNamespace` when unset.
- `TestActive_BecomesLeaderAfterRenew` - Active is not leader until its first
  successful renew.
- `TestActive_LosesLeadershipAfterRenewDeadline` - a renew failure past
  `RenewDeadline` drops leadership (this is the fencing mechanism: a dead
  Active stops being able to write on its own, no external actor required).
- `TestStandby_NeverLeaderEvenWhenLeaseStale` - **Regression (pin for
  #294)**: a Standby that detects staleness must not promote itself.
  Promotion is #297's job; #294 must never do it as a side effect.
- `TestCheckRemoteLeaseOnce_PropagatesGetError` - a failed remote read
  surfaces as an error, never gets read as "fresh".
- `TestRenewOnce_KeepsLeadershipWithinRenewDeadline` - a single failed renew
  still errors, but keeps leadership if still inside the deadline
  (tolerates a transient blip).
- `TestSetLeader_LogsOnlyOnTransition` - `LeadershipAcquired`/`Lost` log
  exactly once per real transition, not once per call.
- `TestStartLeaseRenewal_NoopWhenNotActive`,
  `TestWatchRemoteLease_NoopWhenNotStandby` - each loop is a no-op outside
  its own mode.
- `TestStartLeaseRenewal_ReturnsNilOnContextCancellation`,
  `TestWatchRemoteLease_ReturnsNilOnContextCancellation` - graceful shutdown
  returns `nil`, not `ctx.Err()`, and doesn't log as an error.
- `TestWatchRemoteLease_RequiresRemoteClientInStandbyMode` - a Standby
  configured with no remote client fails fast instead of nil-panicking
  later.
- `TestNeverArmed_NeverBecomesCandidate` - **Regression, the most
  dangerous mistake this file guards against**: an elector that has never
  successfully read the Active's lease must never become a promotion
  candidate, no matter how many failed reads pile up. `isLeaseStale(nil,
  ...)` returns true by design; folding the nil check into it instead of a
  separate "have I ever armed" gate would mean a broken kubeconfig causes
  instant false promotion on tick 1.
- `TestUnreadableLease_RetainsCacheAndGoesStale` - the core #297 detection
  insight: "Active's pod died" and "Active's API server died" look
  identical from here (a frozen `renewTime`), so a read failure retains the
  last cached lease and lets it age on its own rather than treating a read
  error as a different case.
- `TestReadFailure_DoesNotRefreshLastGoodRead` - a failed read must not
  advance the freshness marker used elsewhere.
- `TestSuccessfulRead_ReplacesCachedLease` - a live read on recovery clears
  any latched stale verdict immediately.
- `TestCheckRemoteLeaseOnce_BoundsTheRead` - **Regression, found live**: a
  hanging read against a dead API server (no TCP reset) is bounded by
  `PromotionDialTimeout`; unbounded, it would block the watch loop for
  minutes.
- `TestCheckRemoteLeaseOnce_TimedOutReadStillAgesTheCache` - a timed-out read
  is treated exactly like any other failed read.

### `resume_test.go`: restart-time resume decision (`ResumeAsActive`)

Decides whether a hub that restarts already configured `--ha-mode=active`
(e.g. after being promoted) should actually come back up as Active, or defer
to Standby to avoid a dual writer.

- `TestResumeAsActive_ResumesWhenThisHubHoldsALiveLease` - restarting inside
  the lease duration while still holding it: resume as Active.
- `TestResumeAsActive_DefersWhenItsOwnLeaseIsStale` - its own lease is stale
  (another hub may have already taken over): defer to Standby.
- `TestResumeAsActive_DefersWhenAnotherHubHoldsTheLease` - lease held by a
  different identity: defer.
- `TestResumeAsActive_NoLeaseMeansNeverPromoted` - `NotFound` reads as "stay
  Standby", not as an error.
- `TestResumeAsActive_UnreadableLeaseIsAnErrorNotADemotion` - a transient
  read failure must not silently demote a genuine Active; it surfaces as an
  error and the caller keeps the configured mode.
- `TestResumeAsActive_OnlyAppliesToStandbyMode` - Active/Standalone modes are
  never overridden by this check, even with a live matching lease.
- `TestResumeAsActive_ResolvesTheSameLeaseTargetAsTheElector` -
  **Regression**: this check must resolve the exact same lease
  name/namespace/identity/padding that `applyDefaults` gives the elector, or
  it silently checks the wrong lease and always fails open to "defer".

### `mirror_test.go`: single-object mirror semantics (`pkg/ha/mirror.go`)

`mirrorCreateOrUpdate`/`mirrorDelete`, the engine underneath both the CRD
mirror and the credential mirror.

- `TestMirrorCreateOrUpdate_CreatesWithLabelAndAnnotation` - a fresh mirror
  gets the `synced-from: active` label plus a source-resourceVersion
  annotation.
- `TestMirrorCreateOrUpdate_UpdatesExistingSyncedObject` - an update
  propagates spec changes and refreshes the source-RV annotation.
- `TestMirrorCreateOrUpdate_ConflictGuardSkipsUnlabeledExisting` - an
  existing object without the sync label is never overwritten, so a
  hand-created or hand-applied object on the Standby is safe.
- `TestMirrorCreateOrUpdate_StripOwnerRefsOnlyWhenConfigured` -
  `StripOwnerRefs: true` strips `ownerReferences` (a cross-cluster UID
  reference is meaningless); `false` leaves them alone.
- `TestMirrorCreateOrUpdate_StripsDeletionTimestampFromTerminatingSource`
  (subtests for the update path and the create path) - **Regression, found
  live**: mirroring a Terminating Active-side object must never carry
  `deletionTimestamp`/`deletionGracePeriodSeconds` onto the Standby; a real
  API server rejects that as an immutable-field violation.
- `TestMirrorCreateOrUpdate_MirrorsStatusExplicitly` - the status
  subresource is written with an explicit `Status().Update()`, since a
  plain `Update()` never touches `.status` once the subresource is
  registered.
- `TestMirrorCreateOrUpdate_SkipsStatusMirrorWhenSourceIsTerminating` -
  **Regression**: a Terminating source's status write is skipped, since a
  real API server rejects `status.Phase` once `deletionTimestamp` is set.
- `TestMirrorCreateOrUpdate_SkipsStatusMirrorWhenSkipStatusSet` -
  `SkipStatus: true` suppresses the status write entirely, for types (only
  Namespace today) whose status the API server itself owns.
- `TestCRDMirrorSet_NamespaceSkipsStatusAndNothingElseDoes` - pins the
  mirror-set table: Namespace is the only entry with `SkipStatus` set.
- `TestMirrorDelete_IdempotentOnNotFound`,
  `TestMirrorDelete_DeletesSyncedObject`,
  `TestMirrorDelete_ConflictGuardSkipsUnlabeledExisting` - delete is
  idempotent, deletes labeled mirrors, and never deletes an unlabeled
  object.

### `remote_syncer_test.go`: `RemoteSyncer` (`pkg/ha/remote_syncer.go`)

The workqueue-driven reconcile loop and informer registration/retry. Also
defines stub infrastructure (`stubRemote`, `buildSyncer`, `stubInformer`,
`stubCache`) reused by `prune_test.go`, `credential_set_test.go`, and
`metrics_test.go`.

- `TestRemoteSyncer_ReconcileKey_FoundMirrorsCreate`,
  `_NotFoundMirrorsDelete` - present on the remote: create locally with the
  sync label; absent: delete the local mirror.
- `TestRemoteSyncer_ProcessOnce_RetriesOnErrorAndRedelivers` - a failed sync
  is re-queued via `AddRateLimited`, never dropped. This is issue #295's own
  acceptance criterion.
- `TestRemoteSyncer_HandlersFor_UnwrapsDeletedFinalStateUnknown` - an
  informer delete-tombstone still enqueues the correct key.
- `TestRemoteSyncer_HandlersFor_IgnoresUnexpectedType` - a non-unstructured
  object handed to the handler is ignored, not enqueued.
- `TestRemoteSyncer_Start_NoopWhenNotStandby` - a no-op in Active/Standalone
  mode.
- `TestNewRemoteSyncer_StandbyRequiresRemoteConfig` - constructing a Standby
  syncer with no remote config errors immediately.
- `TestNamespaceMirrorSelector_MatchesOnlyProjectNamespaces` -
  **Regression**: the Namespace informer's label selector matches only
  `util.LabelsKubeSliceController`-stamped namespaces, not `kube-system` or
  other unrelated namespaces. This is the fix for the original unscoped
  Namespace watch, which risked cascade-deleting incidentally-mirrored
  system namespaces.
- `TestRegisterInformers_RetriesUntilSuccess` - informer setup keeps
  retrying past transient failures until it succeeds.
- `TestRegisterInformersOnce_SkipsAlreadyRegisteredHandlersOnRetry` -
  **Regression**: a retry after partial success must not re-register a
  resource whose handler already succeeded.
  `AddEventHandlerWithResyncPeriod` is not idempotent; a naive retry would
  silently double that resource's event and resync load on every retry.
- `TestRegisterInformers_StopsRetryingOnContextCancel` - the retry loop
  aborts cleanly on cancellation.

### `prune_test.go`: drift backstop (`pkg/ha/prune.go`)

The periodic forward-orphan and reverse "missing locally" diff pass.

- `TestPruneOnce_EnqueuesOnlyOrphanedMirrors` - a local labeled mirror absent
  from the Active's remote list is enqueued; one still present is not.
- `TestPruneOnce_ThenWorkerDeletesOrphan` - end to end: the enqueued orphan
  is actually deleted once the worker re-reads it and gets `NotFound`.
- `TestPruneOnce_LeavesUnlabeledObjectsAlone` - objects without the sync
  label are never pruned, even if absent from the Active.
- `TestPruneOnce_SkipsKindWhenRemoteListFails` - a failed remote list for one
  kind must not be read as "everything on Active was deleted".
- `TestPruneOnce_ReverseDiffEnqueuesActiveObjectsMissingLocally` -
  anti-entropy: an Active object with no local mirror (deleted directly on
  the Standby, a cold-start race, or stuck deep in backoff) gets
  re-enqueued.
- `TestPruneOnce_ReverseDiffCannotOverrideConflictGuard` - a hand-created,
  unlabeled object present on both sides keeps getting re-enqueued by the
  reverse diff, but the create-only conflict guard still refuses to adopt
  it.
- `TestRunPrune_DoesNotPruneBeforeCacheSync` - **Regression**: pruning
  against an unsynced cache would read every mirrored object as deleted; the
  loop must wait for `waitForCacheSync` first.
- `TestRunPrune_TicksAndStopsOnContextCancel` - the periodic loop ticks on
  `pruneInterval` and stops cleanly on cancellation.

### `credential_set_test.go`: credential mirroring (`pkg/ha/credential_set.go`)

`CredentialMirrorSet`/`FullMirrorSet`: Secret/ServiceAccount/Role/RoleBinding
mirroring, service-account-token sanitization, and namespace-boundary
enforcement.

- `TestCredentialMirrorSet_ShapeAndDefenses` - every row is exactly
  Secret/ServiceAccount/Role/RoleBinding, all with `StripOwnerRefs` and
  `RequireMirroredNamespace` set.
- `TestIsServiceAccountTokenSecret` - correctly distinguishes an SA-token
  Secret from an Opaque Secret and from an untyped one.
- `TestSanitizeSecret_ReducesTokenSecretToItsShell` - an SA-token Secret's
  `.data` and UID annotation are stripped on mirror (its account-name
  annotation survives, so the Standby's own token controller can mint a
  fresh token); an Opaque secret such as a gateway cert passes through
  unchanged.
- `TestSanitizeCachedSecret_StripsTokenBytesOnTheWayIntoTheCache` -
  defense in depth: token bytes are stripped before the informer cache ever
  holds them (both typed and unstructured paths), the original object is
  never mutated in place, and the UID annotation is deliberately kept in
  the cache view since prune diffs against it.
- `TestFullMirrorSet_CombinesBothSetsWithoutCollisions` - the CRD set and
  the credential set concatenate with no duplicate GVKs.
- `TestReconcileKey_MirrorsOpaqueSecretWithData` - a gateway-cert-style
  Opaque Secret's data mirrors through unchanged.
- `TestReconcileKey_MirrorsServiceAccountTokenSecretAsShell` - an
  Active-signed SA-token Secret mirrors as an empty shell: type and
  account-name annotation only, no data, no UID annotation.
- `TestReconcileKey_NeverOverwritesAMintedTokenSecret` - **Regression, the
  reason the shell design exists**: once the Standby's own token controller
  populates the shell, a later resync of the Active's copy must not
  overwrite it. Doing so would silently invalidate live worker credentials
  on a timer.
- `TestReconcileKey_StillUpdatesNonTokenSecretsOnResync` - the create-only
  guard is per-object, not per-Secret-type: a rotated gateway cert still
  converges on resync.
- `TestPruneOnce_LeavesAMintedTokenSecretAlone` - the prune pass's
  create-only guard also protects a minted token shell from the
  reverse-diff path.
- `TestReconcileKey_SkipsCredentialsInUnmirroredNamespaces` - table of 5
  cases (the controller's own webhook Secret, a `kube-system` bootstrap
  token, a `kube-system` ServiceAccount, a `default`-namespace Role and
  RoleBinding): none mirror, because their namespace isn't in the mirrored
  namespace view. **Regression** for the real security defect found live:
  name-prefix-based namespace scoping would have leaked the controller's
  own webhook TLS secret and image-pull secret.
- `TestReconcileKey_NamespaceCheckErrorIsRetryable` - a transient
  namespace-lookup failure surfaces as a retryable error, never a silent
  skip.
- `TestMirrorCacheByObject_ScopesCredentialInformers` - Secret is
  deliberately cluster-wide (can't be scoped by label or type) with a
  Transform that strips token bytes on ingress; ServiceAccount/Role/
  RoleBinding are label-scoped.

### `active_publisher_test.go`: `status.activeController` publisher (`pkg/ha/active_publisher.go`)

- `TestPublishOnce_WritesWhileTheWriteFenceIsShut` - **Regression**:
  `PublishOnce` (called directly by promotion step 7) must write even while
  `IsLeader()` reports false, since promotion holds the fence shut across
  its entire sequence.
- `TestPublishOnce_PeriodicLoopStillSkipsWhenNotLeader` - the *other*
  entry point, the periodic loop, stays gated on leadership; a Standby
  never advertises itself.
- `TestPublishOnce_WritesActiveControllerToEveryCluster` - one pass writes
  the correct endpoint, identity, and `LastUpdated` to every Cluster CR.
- `TestPublishOnce_SecondPassWritesNothing` - a converged pass performs zero
  writes (comparison ignores `LastUpdated`, or every tick would write).
- `TestPublishOnce_RepublishesWhenIdentityChanges` - a newly promoted hub's
  identity correctly overwrites the previous holder's declaration.
- `TestPublishOnce_RefusesShippedPlaceholderEndpoint`,
  `_RefusesEmptyEndpoint` - refusing to publish an unconfigured endpoint is
  silent success, not an error; it must never block reconciling.
- `TestPlaceholderMatchesServiceDefault` - **Regression pin**:
  `PlaceholderControllerEndpoint` must stay identical to
  `service.ControllerEndpoint`'s shipped default, since `main.go` overwrites
  the latter at startup, so the publisher can't read it live.
- `TestNewActivePublisher_ReadsAndEncodesCABundle`,
  `_PublishesWithoutUnreadableCABundle` - the CA bundle is base64-encoded
  when readable; an unreadable CA path never blocks publication.
- `TestPublishOnce_ReturnsErrorWhenListFails` - a failed Cluster list
  surfaces for retry, not swallowed.
- `TestPublishOnce_ContinuesAfterOneClusterFails` - one Cluster's write
  failing doesn't stop the others from publishing.
- `TestPublishOnce_ReportsWhenTheFieldIsSilentlyPruned` - **Regression, for
  the real defect found on chart-installed CRDs**: a CRD without
  `status.activeController` in its schema silently drops the write; the
  publisher detects this via read-after-write and reports an error naming
  the field and the likely cause.
- `TestPublishOnce_PruningReportsOncePerPassNotPerCluster` - the pruned-field
  error is reported once per pass, not once per Cluster CR.
- `TestPublishOnce_AlreadyConvergedCountsAsVerified` - a hub restarting onto
  already-correct Cluster CRs counts as verified without a fresh write and
  read-back.
- `TestPublishOnce_VerifiesOnceThenStopsReadingBack` - the read-back
  verification runs once per process lifetime, not once per publish pass.
- `TestStart_PublishesPromptlyWhenLeadershipArrivesLate` - **Regression,
  found live**: if leadership arrives after `Start`'s first pass, the loop
  must poll on the short `LeadershipPollInterval` rather than wait a full
  publish `Interval`. A fresh Active took 31s to advertise itself before
  this fix.
- `TestStart_ReturnsNilOnContextCancel` - `Start` publishes immediately on
  entry (doesn't wait for the first tick) and returns cleanly on
  cancellation.

### `promotion_guards_test.go`: pre-promotion guards (`pkg/ha/promotion_guards.go`)

`selfHealthy`, `activeStillAlive`, `guardsAllowPromotion`, and
`PromotionDialTimeout`.

- `TestSelfHealthy_NotFoundCountsAsHealthy` - **the most important test in
  this file**: no local HA Lease yet (the very first failover ever) reads
  as healthy, because the API server answered. Treating `NotFound` as
  unhealthy would block every real first failover.
- `TestSelfHealthy_ExistingLeaseIsHealthy`,
  `_UnreachableIsUnhealthy` - a readable own-cluster API server is healthy
  regardless of what the lease says; a transport error against it is
  unhealthy ("it might be me, not them").
- `TestActiveStillAlive_FreshLeaseAborts` - the final-dial guard: if the
  Active renewed between polls, abort. This is the polling-race case the
  guard exists for.
- `TestActiveStillAlive_RefreshesCacheOnAbort` - an abort refreshes the
  cached lease view rather than leaving a stale verdict latched.
- `TestActiveStillAlive_UnreachableProceeds`,
  `_StaleLeaseProceeds` - an unreachable Active, or a confirmed-still-stale
  read, doesn't block promotion; unreachable is not evidence of life.
- `TestGuardsAllowPromotion_BothPass`,
  `_SelfUnhealthyBlocks`,
  `_LiveActiveBlocks` - the combined guard: own API reachable and Active
  unreachable proceeds; self unreachable blocks ("it's me" case); a
  demonstrably-alive Active blocks.
- `TestGuardsAbort_DoesNotDisarm` - **Regression**: a guard refusal must not
  clear `lastSeenLease`. Disarming here would mean an already-gone Active
  can never re-arm the elector, permanently disabling future failover.
- `TestPromotionDialTimeout_IsApplied` - **Regression**: an unbounded guard
  read would hang for the OS TCP timeout (minutes); `PromotionDialTimeout`
  bounds it.

### `promotion_test.go`: the promotion sequence (`pkg/ha/promotion.go`)

`promote()` end to end, driven through a `promotionRecorder` that captures
hook call order and fence state at each step.

- `TestPromote_HappyPath` - the full sequence completes: fence opens, mode
  flips to Active, lease acquired under the local identity.
- `TestPromote_StopsMirrorBeforeOpeningTheFence` - **ordering regression**:
  the mirror must stop first, and the fence must stay shut through
  `stopMirror`/`publish`; otherwise a still-running mirror and the new
  Active dual-write the same objects.
- `TestPromote_FenceOpensOnlyAfterPublish` - pins the full order
  (`stopMirror`, `publish`, `kick`, `event`); the fence is open by the time
  `kick`/`event` run, since a kick before the fence opens would drop
  re-enqueued requests.
- `TestPromote_FenceStaysShutThroughout` - `IsLeader()` reports `false` for
  the entire sequence, even with a stale internal flag set; only `true`
  after completion.
- `TestPromote_GuardRefusalLeavesHubUnchanged` - a guard refusal runs zero
  hooks and leaves mode, fence, and lease untouched; the promoting latch is
  released so a later retry can succeed.
- `TestPromote_MirrorStopFailureAborts` - a failed mirror stop aborts before
  any further step (dual-writer risk), latch released.
- `TestPromote_AbortAfterMirrorStopIsLoud`,
  `_AbortBeforeMirrorStopIsQuiet` - an abort *after* the mirror was stopped
  logs at error level (a permanent, one-way state change worth flagging); an
  abort *before* (a guard refusal) logs nothing, since the mirror was never
  touched.
- `TestPromote_PublishFailureStillPromotes` - publishing
  `status.activeController` is a best-effort step, not a precondition;
  promotion completes even if it fails.
- `TestPromote_PublishRespectsGracePeriod` - a hung publish is bounded by
  `PromotionGracePeriod`, and doesn't hold the fence shut indefinitely.
- `TestPromote_KickFailureStillPromotes` - a kick failure doesn't undo an
  otherwise-successful promotion.
- `TestPromote_IsOnceOnly` - **Regression, found via `-race`**: a second
  `promote()` call on an already-active hub is a no-op that reruns zero
  hooks. Re-running would race the renewal loop's live Lease writes.
- `TestPromote_NilHooksAreSkipped` - an elector with no hooks configured
  still completes lease acquisition and the mode flip.
- `TestWatchRemoteLease_PromotesAndReturns` - end to end: the watch loop
  itself promotes and returns once it does, since it's no longer watching a
  gone Active.
- `TestWatchRemoteLease_NeverArmedNeverPromotes` - an unarmed standby never
  promotes across many ticks.
- `TestPromote_ConcurrentAttemptsRunTheSequenceOnce` - real-concurrency
  test, two goroutines racing `promote()`; exactly one runs the sequence,
  verified under `-race`.
- `TestPromote_LeaseAcquisitionFailureAborts` - a lease-acquire write
  failure aborts (dual-writer risk without a lease), mode/fence unchanged.
- `TestPromote_EventFailureStillPromotes` - a failed `EmitPromotedEvent`
  doesn't undo the promotion.
- `TestPromote_AttachesTheAcquiredLeaseToTheEvent` - the event hook
  receives the just-acquired lease object (correct name/holder), which is
  what places the K8s Event in the right namespace.
- `TestPromote_BoundsTheMirrorStop` - **Regression**: an unbounded wait on a
  mirror that never confirms stopping would hang the *entire* watch loop
  forever, worse than the dual-writer state it guards against. Bounded by
  the grace period; aborts loudly and releases the latch.
- `TestPromote_RefusesWithoutARemoteClient` - calling `promote()` with no
  remote client configured errors cleanly instead of crashing inside the
  final dial.
- `TestPromote_BoundsThePostFenceHooks` (subtests: `kick`, `event`) - a hung
  post-fence hook is bounded by the grace period and doesn't undo an
  already-completed takeover, since the hub is already Active and writing
  by that point.

### `promotion_event_test.go`: the promotion K8s Event (`pkg/ha/promotion_event.go`)

- `TestPromotedToActiveEvent_LandsInTheControllersOwnNamespace` - the Event
  attaches to the Lease (not a fixed `kubeslice-system`, which doesn't exist
  on a hub per ADR #293), lands in the Lease's namespace, type `Normal`.
- `TestPromotedToActiveEvent_FollowsTheLeaseNamespace` - the namespace is
  derived from wherever the Lease actually lives, never hardcoded.
- `TestPromotedToActiveEvent_RequiresGeneratedMapEntry` - guards the `make
  generate-events` step: an unregistered event name errors loudly instead
  of being silently dropped.
- `TestPromotedToActiveEmitter_NilRecorderDisablesEmission`,
  `_NilLeaseIsNotFatal` - a nil recorder yields a nil (skipped) hook; a nil
  lease at emit time isn't reported as a failure, since promotion already
  succeeded by then.

### `kicker_test.go`: post-promotion re-enqueue (`pkg/ha/kicker.go`)

`ReconcileKicker` re-enqueues every pre-existing object into a per-type
`source.Channel` right after promotion, so the newly-Active hub's
reconcilers process state that predates its own startup, not just new
events.

- `TestKick_DeliversOneEventPerObject` - every existing object of a kicked
  type gets exactly one event.
- `TestKick_ChannelsAreDisjointPerType` - **the regression it exists to
  avoid**: per-type channels stay disjoint; Cluster events never land on
  the Project channel. Sharing one channel across all 9 sources would
  misroute most objects in a way that's invisible with only a couple of
  test objects.
- `TestKick_EmptyClusterIsNotAnError` - nothing to kick is a valid, silent
  outcome.
- `TestKick_ContinuesAfterOneTypeFails` - one type's `List` failure doesn't
  stop the other types from being kicked; the failing type is named in the
  returned error.
- `TestKick_DoesNotBlockOnAFullChannel` - **Regression**: kick must not
  block when a channel fills, because `main.go` calls it before `mgr.Start`,
  so nothing is draining the channel yet. Excess events are dropped
  (a degradation, not a failure).
- `TestKick_RespectsContextCancellation` - **found via `-shuffle`**: an
  already-cancelled context must deterministically abort the kick on every
  run. A `select` with both `ctx.Done()` and the channel send ready is not
  deterministic; an earlier version passed or failed at random.
- `TestSource_UnknownTypeIsNil` - an unregistered GVK's `Source()` returns
  `nil`, so the caller simply registers no watch for it.
- `TestNilKicker_IsSafe` - a nil `*ReconcileKicker` behaves as "no kick"
  rather than panicking (the standalone-mode path).
- `TestReconciledGVKs_CoversEveryReconciledTypeAndNothingElse` - exactly 9
  GVKs, Namespace excluded (mirrored but not reconciler-owned), every
  kicked type actually present in `CRDMirrorSet`, and the returned slice is
  a defensive copy.

### `events_test.go`: mirror-failure event (`pkg/ha/events.go`)

- `TestProcessOnce_EmitsHAMirrorSyncFailedOncePerFailureEpisode` - the first
  failure emits once (`Count=1`); same-episode retries don't re-emit; a
  successful sync doesn't emit at all; recovery followed by a *new* failure
  emits again as a new episode (`Count=2`).
- `TestProcessOnce_NoRecorderMeansNoEventButStillRetries` - a nil event
  recorder doesn't panic and doesn't break the retry contract.
- `TestHAMirrorSyncFailedEvent_RegisteredInGeneratedMap` - guards `make
  generate-events`: the real generated map accepts the event; an empty map
  errors loudly.

### `lifecycle_events_test.go`: startup and transition events (`pkg/ha/lifecycle_events.go`)

`BecameActive`, `BecameStandby`, `LeadershipLost`, `PromotionAborted`, plus
`abortPromotion`.

- `TestHALifecycleEvents_RegisteredInGeneratedMap` - all 4 event names are
  present in the generated map with #298's exact reasons and severities
  (`Warning` for `LeadershipLost`/`PromotionAborted`, `Normal` for
  `BecameActive`).
- `TestEmitStartupModeEvent_RecordsTheModeThisHubStartedIn` - Standby mode
  emits `BecameStandby` only; Active mode emits `BecameActive` only.
- `TestEmitStartupModeEvent_StandaloneIsSilent` - standalone (the pre-HA
  default) emits nothing at all: the no-regression guarantee.
- `TestEmitLifecycleEvent_AttachesToTheLeaseInTheControllerNamespace` -
  events attach to the Lease, placing them in the controller's own
  namespace rather than the nonexistent worker namespace
  `kubeslice-system`.
- `TestEmitLifecycleEvent_WorksBeforeTheLeaseExists` - `BecameStandby` must
  record even though a fresh Standby has no local Lease yet; it references
  the Lease by name, it doesn't require a live read of it.
- `TestEmitLifecycleEvent_NilRecorderIsANoop` - a nil recorder doesn't
  panic.
- `TestEmitLifecycleEvent_RecorderFailureIsSwallowed` - a failed event write
  doesn't fail the caller; an observability gap must never become an
  outage.
- `TestAbortPromotion_CountsTheReasonAndEmitsOneEvent` - a guard refusal
  increments `ha_promotions_aborted_total` and emits exactly one `Warning`
  `PromotionAborted` event.
- `TestGuardRefusal_EmitsPromotionAborted` - the same guarantee, exercised
  end to end through the real `promote()` guard path.
- `TestRenewOnce_EmitsLeadershipLostOnlyPastTheRenewDeadline` - a renew
  failure inside the deadline is silent; only past the deadline does it
  emit exactly one `Warning` `LeadershipLost`.
- `TestStartLeaseRenewal_ShutdownDoesNotEmitLeadershipLost` - graceful
  shutdown must not be reported as lost leadership, or every rolling
  restart would look like an incident.

### `metrics_test.go`: the 18 HA metrics (`pkg/ha/metrics.go`)

Registration, HTTP exposition, and every call site that updates them.

- `TestHAMetrics_RegisteredOnControllerRuntimeRegistry` - **Regression for
  a real defect**: all 18 metrics live on `ctrlmetrics.Registry`. They were
  previously on the default Prometheus registry, which controller-runtime's
  own `/metrics` server never serves.
- `TestHAMetrics_ServedOverHTTPWithHelpAndType` - a real
  `promhttp.HandlerFor` exposition includes HELP/TYPE for every metric.
- `TestHAMetrics_HaveHelpAndType` - the same guarantee, checked via
  `Gather()` instead of a live HTTP round trip.
- `TestHAMetrics_LintClean` - Prometheus' own naming linter passes for all
  18 collectors (`_total` suffix on counters, unit/name agreement, etc).
- `TestHASyncMetrics_RecordAndLabel` - `ha_sync_lag_seconds` and
  `ha_sync_errors_total` record and label correctly.
- `TestHASyncLagSeconds_UsesTheSpecifiedBuckets` - exact bucket boundaries
  `[0.1, 0.5, 1, 2, 5, 10, 30]`; the previous default buckets wasted more
  than half their resolution below 100ms and stopped entirely at 10s.
- `TestHAPromotionHistograms_ReachPastTenSeconds` - the top bucket is 60s: a
  promotion can span up to 4 grace periods plus 2 guard dials, so a 10s
  ceiling would collapse the slowest, most interesting promotions into
  `+Inf`.
- `TestHALeaderStatus_TracksLeadershipTransitions` - the gauge is published
  at construction (1 for standalone, 0 for standby) and flips correctly on
  every `setLeader` call.
- `TestRoleScopedGauges_AbsentOnTheWrongRole` - **Regression for a real
  defect found on a live pair**: role-scoped gauges (`ha_armed`,
  `ha_remote_lease_age_seconds`, `ha_lease_last_renew_time_seconds`,
  `ha_last_promotion_timestamp_seconds`, `ha_prune_last_run_timestamp_seconds`)
  must be entirely absent (zero series) on the wrong role, not
  present-and-zero. A plain Gauge always collects 0 even when unset, which
  made "unarmed" and "no backstop running" alerts fire on every Active.
- `TestPromote_DropsStandbyScopedSeries` - after promotion,
  `ha_remote_lease_age_seconds`'s series disappears (there's no remote left
  to age), and `ha_last_promotion_timestamp_seconds` starts publishing.
- `TestHAArmed_ZeroUntilTheActiveLeaseIsRead` - `ha_armed` starts at 0,
  stays 0 through failed reads, and flips to 1 only after one successful
  remote read.
- `TestHARemoteLeaseReads_CountedByResult` - `ha_remote_lease_reads_total`
  is labeled by `ok`/`error` result.
- `TestHARemoteLeaseAge_ClimbsWhileReadsFail` - the age gauge keeps
  climbing against the wall clock during an outage, rather than freezing at
  the last good value; it's a leading indicator, not a lagging one.
- `TestRemoteLeaseAge` - table: nil lease has no age; missing `renewTime`
  has no age; a past `renewTime` gives the correct duration; a **future**
  `renewTime` (clock skew) clamps to 0 rather than going negative, which
  would make an alert flap.
- `TestPromote_RecordsDurationTimestampAndSteps` - a successful promotion
  increments `ha_failover_total`, stamps
  `ha_last_promotion_timestamp_seconds`, records `outcome="promoted"` in
  the duration histogram, and every one of its 5 named steps records its
  own step-duration sample.
- `TestPromote_AbortIsLabelledAbortedAndCounted` - a guard-refused
  promotion records `outcome="aborted"` (not `"promoted"`) plus the
  specific abort-reason counter.
- `TestPromote_ConcurrentRejectionIsNotTimed` - a near-instant
  latch-rejected concurrent attempt is counted but not added to the
  duration histogram; adding it would drag the aborted-quantiles toward
  zero and hide genuinely slow aborts.
- `TestHALeaseRenewMetrics` - a successful renew stamps
  `ha_lease_last_renew_time_seconds`; a failed renew increments
  `ha_lease_renew_errors_total`.
- `TestHAPruneMetrics_CountResurrectionsAndStampTheRun` - a reverse-diff
  resurrection increments `ha_prune_resurrected_total`; every completed
  pass stamps `ha_prune_last_run_timestamp_seconds`.
- `TestHAPruneLastRun_StampedEvenWhenAKindWasSkipped` - a partially
  degraded prune pass (one kind's list failed) still stamps the "last run"
  timestamp. This is a deliberate choice: a degraded pass shouldn't look
  indistinguishable from one that never ran.
- `TestHASyncQueueDepth_RisesOnEnqueueAndFallsOnDrain` -
  `ha_sync_queue_depth` tracks backlog directly, a distinct signal from lag
  (which only reflects completed items).

---

## 2. Reconciler write fencing: `controllers/controller`

`leader_gate_test.go` tests the `IsLeader()` gate wired into the top of
every reconciler's `Reconcile()` (the #294 write-fencing requirement),
against a real reconciler type rather than a synthetic stand-in.

- `TestReconcile_StandbySkipsAndLogs` - a `SliceConfigReconciler` built with
  a Standby elector and a deliberately nil `SliceConfigService` (a gate that
  leaked through would panic on it) returns `ctrl.Result{}, nil` on 3
  consecutive calls, logging "standby mode, skipping reconcile" exactly
  once per call. Proves `IsLeader()` is evaluated on every invocation, not
  cached once at startup.

Run with `go test ./controllers/controller/...`. This package's unrelated
`TestAPIs` needs the envtest `etcd` binary; that failure is pre-existing on
any machine without it and has nothing to do with HA.

---

## 3. End-to-end: `test/e2e` (issue #299)

One top-level test, `TestActiveStandbyHA`, builds a single Active+Standby
hub fixture (`newHubFixture`: two real, disposable Kind clusters running
this branch's controller image) shared across 4 ordered subtests. Each
scenario asserts on real logs, real Lease objects, and real Cluster CRs on
real clusters, not fakes.

### `BaselineSync`

Normal-operation CRD mirror, the controller-side counterpart of the
external suite's `20-baseline.sh`. Verifies: the Active holds its own
lease and the Standby holds none; the Active publishes
`status.activeController` naming itself; the mirror copies the Cluster CR
to the Standby with the `synced-from: active` label, and the mirrored copy
also names the Active. It then mutates an annotation on the Active's live
object and requires it to propagate to the Standby's mirror, a positive
control proving the mirror is live-syncing rather than a one-time initial
copy. Finally checks the Standby's logs contain "watching active hub
lease".

### `TransientBlipDoesNotPromote`

Designed fresh for this suite, no equivalent in the external one. Revokes
the Standby's remote-read RBAC on the Active for half the detection budget,
then restores it. Verifies the Standby's logs show it actually tried and
failed during the outage window (a positive control: "could not read
active hub lease; retaining last known view"), and that after restoration
and one retry period, the logs never show "PROMOTED to active" or
"promotion sequence starting". The Active still holds the lease; the
Standby never acquired one.

### `FailoverPromotion`

Ported from the external suite's `40-failover.sh`. Kills the Active's
manager Deployment and waits up to `promotionCeiling() + 10s` for "PROMOTED
to active" in the Standby's logs. Verifies: the elapsed time is within
budget; the logs never contain "promotion aborted"; the full ordered
sequence appears (`STALE` &rarr; `promotion sequence starting` &rarr;
`state mirror stopped and confirmed exited` &rarr; `acquired lease on this
hub` &rarr; `published activeController for the new Active` &rarr;
`re-enqueued all reconciled types after promotion` &rarr; `PROMOTED to
active`); the Standby now holds the HA lease; the Standby's Cluster CR
names itself as the active controller; and a `PromotedToActive` K8s Event
exists, checked by its real `.reason` field against `pkg/ha/promotion_event.go`
rather than a label-based grep.

### `ReconciliationResumesOnThePromotedHub`

The controller-focused half of worker reconnection (the worker side is
worker-operator issues #467/#468/#469, already covered in that repo's own
suite). Creates a brand-new Cluster CR on the now-promoted hub and waits
for `status.SecretName` to be populated, proof the reconciler is genuinely
processing new objects and not just holding the lease, then re-confirms
`status.activeController` still names the promoted hub.

### Harness

No `func Test...` of its own; all `t.Helper()`-marked support:

| File | Provides |
|---|---|
| `kind_test.go` | Disposable cluster lifecycle (`kindCreateCluster`, prefix `e2e-ha-`, guaranteed teardown via `t.Cleanup`), kubeconfig/REST config, `waitFor` (generic poll-until-true) |
| `deploy_test.go` | CRD/RBAC apply, the manager Deployment, the standby-reader ClusterRole binding |
| `client_test.go` | Scheme registration, controller-runtime client construction |
| `cr_helpers_test.go` | The Project + Cluster CR fixture the scenarios mirror/promote around |
| `assertions_test.go` | Log/lease/event assertions (`managerLogs`, `leaseHolder`, `assertLogSequence`, `eventReasonExists`, ...) |
| `ha_credentials_test.go` | Provisions the Standby's cross-cluster credential end to end via a real ServiceAccount, token, and ClusterRoleBinding on the Active |

Run with `make test-e2e-ha`. See the Layout section above for prerequisites.

---

## 4. Coverage map

| Issue | What it covers | Primary tests |
|---|---|---|
| #293 (ADR) | Design decisions encoded as invariants elsewhere in this table | n/a, see `docs/ha-runbook.md` |
| #294 (leader election, fencing) | `mode_test.go`, `lease_test.go`, `leader_elector_test.go`, `leader_gate_test.go` | |
| #295 (state mirror) | `mirror_test.go`, `remote_syncer_test.go`, `prune_test.go`, `credential_set_test.go`, `events_test.go` | |
| #297 (failover/promotion) | `resume_test.go`, `promotion_guards_test.go`, `promotion_test.go`, `promotion_event_test.go`, `kicker_test.go`, `active_publisher_test.go` (the `activeController` half) | |
| #298 (observability, runbook) | `metrics_test.go`, `lifecycle_events_test.go`, plus the event-registration checks inside `events_test.go`/`promotion_event_test.go` | |
| #299 (e2e) | `test/e2e/*_test.go` | |

---

## 5. Totals

| Layer | Files | Test functions | Lines |
|---|---|---|---|
| `pkg/ha` unit | 16 | 182 | 4807 |
| `controllers/controller` gate | 1 | 1 | 62 |
| `test/e2e` | 11 (1 top-level test, 4 subtests, 6 harness files) | 1 | 1264 |
| **Total** | **28** | **184 `func Test...` entries (+ 4 named e2e subtests)** | **6133** |
