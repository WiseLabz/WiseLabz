# Security Policy

WiseLabz handles credentials for the services in your home lab — Proxmox tokens, Docker
socket paths, pfSense API keys, and more. We take that responsibility seriously. This
document describes how to report a vulnerability and what you can expect from us.

## What counts as a security vulnerability

For WiseLabz, a security vulnerability is any defect that could allow:

- Unauthorized access to or disclosure of stored service credentials
- Unauthenticated access to the WiseLabz API or dashboard
- Injection of untrusted configuration data that leads to code execution
- Exposure of sensitive data through generated documentation or the web UI
- Privilege escalation within the WiseLabz process or container
- Bypassing the configuration environment-variable isolation model

When in doubt, report it privately. We'd rather triage a non-issue than leave something
real unaddressed.

## Reporting a vulnerability

**Do not open a public issue for a security vulnerability.**

Use GitHub Security Advisories to report privately:

1. Go to the [Security Advisories][advisories] tab on the WiseLabz repository.
2. Click **New draft security advisory**.
3. Fill in the details, a clear description, steps to reproduce, and any relevant
   environment details help us triage faster.
4. Submit. Only WiseLabz maintainers will see your report.

We practice responsible disclosure. Once a fix is ready and released, we will publish a
security advisory with credit to the reporter (unless you prefer to remain anonymous).

## What we commit to

| Commitment                                     | Timeline                          |
|------------------------------------------------|-----------------------------------|
| Acknowledge your report                        | Within 48 hours                   |
| Provide a status update and initial assessment | Within 7 days                     |
| Ship a fix or a mitigation advisory            | As appropriate to severity        |
| Request a CVE                                  | If the vulnerability warrants one |

These are targets, not guarantees, we're a small team. If you haven't heard back within
the stated windows, ping us.

## Supported versions

Only the latest release receives security patches. There is no LTS train for older major
versions at this stage. If you run WiseLabz from `main` (build from source), upgrade to
the latest tagged release before reporting, you may be on a version where the issue is
already fixed.

Check the [releases page][releases] for the current version.

[advisories]: https://github.com/WiseLabz/WiseLabz/security/advisories
[releases]: https://github.com/WiseLabz/WiseLabz/releases
