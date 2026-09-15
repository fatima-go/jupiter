# CLI-managed rollout lifecycle

Managed clients open an authenticated DeploymentManagement session before Create.
Heartbeat renews it every 10 seconds; expiry is fixed at 20 seconds after the last
receipt. Detach persists immediate cancellation intent. Watch never renews or
owns a session. Expired credentials cannot resume ownership.

Jupiter persists the session binding, cancellation reason, dispatch intent, and
last/next reconciliation timestamps. It rechecks ownership before Stage/Start.
A closed owner stops further targets. Dispatched but unstarted IDs are sealed by
Juno Cancel; running operations finish and their results are retained. Failed
operations no longer reserve a group, but retry rechecks newer reservations.

ATTENTION automatically retries observation, using the same operation ID. A
missing/rejected operation that was never dispatched is a definite failure;
errors after dispatch remain uncertain until reconciled. Cancellation never
turns into a new deployment. Partial completion is retained per target, with
CANCELLED/FAILED/SUCCEEDED as the final rollout result.

The new workflow requires deployment_cancel on selected Junos. Old unmanaged
rollouts keep their existing lifetime; they can be viewed and explicitly
cancelled. Upgrade an old Juno if a staged legacy operation cannot be sealed.
Retention failures do not prevent lifecycle ticks.

For coordinated source builds, go.mod resolves fatima-core from ../fatima-core.
Replace this with a released module version when publishing standalone sources.

## Sequential target interval

`deployment.v2.target.interval.seconds` defaults to 5; 0 disables the gap. Set a non-negative integer in Jupiter configuration before startup. Invalid values prevent the deployment service from starting.

The second target starts immediately after first-target approval. After confirmed success of each subsequent target, Jupiter persists `next_target_start_at` together with the successful result before waiting. No gap follows the last target. Restart/reconciliation preserves the deadline; worker lease recovery may delay execution further. Cancellation and management expiry clear the deadline and prevent subsequent dispatch. Separate rollouts wait independently. This interval does not perform an application health check.
