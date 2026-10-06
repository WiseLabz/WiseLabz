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
After reviewing the preview, confirm the runbook’s name and authenticate when
prompted. This approval is scoped to that runbook and covers the entire run.
A runbook with no steps cannot start.

Only one running, waiting, or failed run can exist per runbook. If Start reports
that a run is already active, follow **Open active run** to inspect it. Resume or
cancel that run before starting another.

## Follow and control a run

The run detail updates as steps change. The run can be **Running**, **Waiting for
confirmation**, **Failed**, **Succeeded**, **Cancelled**, or **Expired**. Steps can
be **Pending**, **Running**, **Waiting**, **Succeeded**, **Failed**, **Skipped**,
or **Unknown**. Detail shows available timestamps, reasons, errors, and the users
who started, resumed, confirmed, or cancelled the run.

- **Confirm step** continues a waiting manual step. It needs operator access to
  every connector in the run, but no additional authentication approval.
- **Resume run** requires fresh approval for the original runbook and retries
  from the first step that did not succeed. Completed steps are not repeated.
  If that step is **Unknown**, its real outcome is not known, usually because the
  backend restarted during execution. Inspect the connector before approving:
  resuming repeats that operation and may repeat a change that already happened.
- **Cancel run** asks for confirmation and prevents further steps. It does not
  undo completed operations. Unfinished steps become skipped.

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
without activity and remove finished runs after 90 days. Administrators can
adjust both under retention settings; a history period of 0 keeps finished runs
indefinitely. Expired runs cannot be confirmed or resumed. Notification rules
can alert you when a run fails or waits for manual confirmation; success and
cancellation do not send run notifications.
