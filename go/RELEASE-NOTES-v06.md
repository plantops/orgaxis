# OrgAxis 0.6 — HR workflow & RBAC demonstration

- Compared with source repository `ngtrthanh/chinfon-kaizen`: database roles, scoped visibility, audit, anti-self-review, staged approval and Kaizen financial/nonfinancial reward tiers.
- Legacy insecure shared-key API gated behind local administrator session; scoped `/v2` for employee workflows.
- Six seeded users (random one-time credentials at first launch), session cookies, CSRF, login rate limiting, server-side RBAC.
- Organization explorer: 1,000 individual synthetic employee markers at bird-eye; department/team drilldown; effective date query.
- Leave requests: employee-owned, Monday–Saturday demo workday calculation, role-based approval thresholds, no self-approval.
- Bonus and discipline: direct manager opens, HR and director progression, evidence narrative, persistent approval history.
- 2026 payroll: 12,000 synthetic monthly pay records across 12 months; aggregate/year and self/authorized payslip.
- Non-destructive migration, consistent pre-v0.6 SQLite backup, Linux Go tests + HTTP authorization tests.
- Limitations: no legal payroll/tax/BHXH calculation, no MFA/SSO, no Windows native integration acceptance or production security certification.
