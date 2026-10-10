# Runbook runs

A run executes a frozen copy of a runbook’s steps in order on the server. You can
close the browser while it runs; edits to the runbook do not alter existing runs.
Executing a single lifecycle step still works independently and does not create
a run history entry.

## Start a run

Open the runbook attached to a change, alert, or finding and choose **Start run**.
Review the ordered preview: lifecycle steps show the target, affected entities,
dependencies, and expected downtime. Sync steps wait for connector synchronization;
health steps wait for an online health check; manual steps wait for a person.

Any blocked step disables Start. You need operator access to every connector in
the run, and steps on connectors you cannot view appear as **Restricted step**.
For a run with no connectors, only an instance admin or someone with operator
access to at least one connector can start it.
After reviewing the preview, confirm the runbook’s name and authenticate when
prompted. This approval is scoped to that runbook and covers the entire run.
A runbook with no steps cannot start.

Only one awaiting approval, running, waiting, or failed run can exist per
runbook. If Start reports that a run is already active, follow **Open active
run** to inspect it. Resume or cancel that run before starting another.

## Require a second approver

An instance admin can turn on **Require a second approver** for a runbook in
**Settings → Runbooks**. Starting such a runbook is a request: the button reads
**Request approval**, and the run is created as **Awaiting approval** with its
steps frozen but nothing executed. The request needs another enabled operator
who has operator access to every connector in the run (or, for a run with no
connectors, an instance admin or someone with operator access to at least one
connector). When there is none, the preview says so and the button is disabled.
Eligible operators are notified through the `runbook.run_approval_requested`
notification rule.

- **Approve** is available to an eligible operator other than the person who
  requested the run, and requires that operator’s own fresh authentication
  approval for the run when step-up is on. The run then executes as the
  original requester, so their access to each connector is checked again at
  every step.
- **Reject** is available to the same operators, needs no additional
  authentication approval, and never executes anything. The run becomes
  **Rejected** and its steps are skipped.
- The requester can withdraw the request with **Cancel request**, which uses the
  regular cancel rules and skips the pending steps.
- A request nobody answers expires as **Expired** after the approval retention
  setting, 24 hours by default, and its steps are skipped. Administrators adjust
  it under retention settings as **Runbook approval requests (hours)**.

Resume and Confirm step are unavailable while a run awaits approval, and a
backend restart leaves the request waiting. Executing a single step directly is
refused for runbooks that require a second approver; use a run instead.

## Follow and control a run

The run detail updates as steps change. The run can be **Awaiting approval**, **Running**,
**Waiting for manual confirmation**, **Failed**, **Succeeded**, **Cancelled**,
**Rejected**, or **Expired**. Steps can
be **Pending**, **Running**, **Waiting**, **Succeeded**, **Failed**, **Skipped**,
or **Unknown**. Detail shows available timestamps, reasons, errors, and the users
who started, approved, rejected, resumed, confirmed, or cancelled the run.

- **Confirm step** continues a waiting manual step. It needs operator access to
  every connector in the run, but no additional authentication approval. For a
  run with no connectors, only an instance admin or someone with operator access
  to at least one connector can confirm it.
- **Resume run** requires fresh approval for the original runbook and retries
  from the first step that did not succeed. Completed steps are not repeated.
  When the run has no connectors, the caller must be an instance admin or have
  operator access to at least one connector.
  If that step is **Unknown**, its real outcome is not known, usually because the
  backend restarted during execution. Inspect the connector before approving:
  resuming repeats that operation and may repeat a change that already happened.
- **Cancel run** asks for confirmation and prevents further steps. On a run
  awaiting approval it is shown as **Cancel request** and withdraws the request. It does not
  undo completed operations. Unfinished steps become skipped. Any user with
  operator access to every connector in the run that still exists can cancel it;
  connectors deleted since the run started do not count. If no connectors
  remain, the caller must be an instance admin or have operator access to at
  least one connector. Cancelling needs no additional authentication approval.

A backend restart never automatically continues an interrupted run. A running
run becomes failed and its active step becomes unknown; a waiting manual step
remains waiting. A deleted runbook’s saved run remains readable, but cannot be
resumed. The detail explains this restriction.

## History and retention

Choose **Run history** in the runbook panel or beside a runbook in
**Settings → Runbooks**. History lists newest runs first and supports pagination.
Open an entry to inspect its frozen steps, state, and activity. Run detail links
can also be opened directly, even after the runbook has been deleted.

Connector permissions apply to history too: hidden steps appear as neutral
restricted rows. Live events refresh detail through the API so the same
redaction remains in effect.

The default retention settings expire waiting or failed runs after 24 hours
without activity, expire unanswered approval requests after 24 hours, and
remove finished runs after 90 days. Administrators can
adjust both under retention settings; a history period of 0 keeps finished runs
indefinitely. Expired runs cannot be confirmed, resumed, or approved. Notification rules
can alert you when a run fails or waits for manual confirmation, and tell
eligible approvers about a new approval request; success and cancellation do not
send run notifications.
