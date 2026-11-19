# OrgAxis KB Intake — Onboarding, Identity and 2FA for Mixed Workforce

Intake date: 2026-11-19. Status: **REVIEWED REFERENCE / DESIGN ONLY**. No runtime implementation or AppGate promotion.

## Authoritative source

User-supplied original knowledge base (preserved without edits):
- `docs/kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md`
- Original file SHA-256: `df100586e863cb5969975b7d01fc373cb6186cc39d2ff4c13afb99ca0624f096`
- Original Git blob SHA-1: `e26bb090ec125d3dece6c431a6b0492a8a50e3e0`

The source is a design proposal, not a tested security standard. This review does not overwrite or silently amend it. All changes below are **proposed engineering dispositions**.

## Context and functional scope from source

The document defines three workforce populations: employees with company email, workers without company email or personal phone enrollment, and employees with limited access/skills in smartphone technology.

Proposals within the source:
1. Hybrid first enrollment, including email route and non-email route with DDMM date-of-birth temporary password.
2. Password + TOTP (RFC 6238), `otpauth://` deep link, manual setup key, QR, trusted device cookie (30 days), drift `skew=1`, and backup codes.
3. Shared workstation/kiosk, QR employee badges, foreman acting on behalf of staff and on-site PIN/signature.
4. PostgreSQL reference entities `users`, `user_backup_codes`, `activity_logs`.
5. Optional self-hosted 4G SMS gateway.

## Engineering disposition — KEEP / MODIFY / REJECT

| Source feature | Disposition | Rationale and OrgAxis target |
|---|---|---|
| Distinguish email and no-email workers | **KEEP** | Account creation and access must not require personal email/phone |
| First login with emailed temporary password | **MODIFY** | Use a short-lived, one-time activation flow and verified delivery; an 8-character password alone is not sufficient evidence of identity |
| Mã NV + `DDMM` first password | **REJECT for authentication** | Date of birth is low-entropy and discoverable; 48–72h and 3 failures do not turn it into a secure enrollment secret. Use a high-entropy one-time activation code distributed through an approved supervised HR/shift-leader identity verification process |
| Pending / Activated / Expired / Locked state and enrollment-only token | **KEEP / HARDEN** | Use separate limited-scope enrollment session; atomic single-use activation, expiry, global retry controls, new factor binding and password setup; zero salary/attendance access before activation |
| Username case/whitespace normalization | **KEEP** | Normalize identifier canonically without altering unrelated secrets |
| TOTP with standard authenticator | **KEEP** | Validate enrollment with a fresh OTP, protect secret at rest, rotate/revoke, reject replay, bind with least-privilege authenticated setup |
| `otpauth://` + manual setup key + QR | **MODIFY** | Display once within controlled enrollment UI, avoid logging URI/secret, use same-device UX fallback; manual copy is sensitive and needs exposure warnings |
| `skew=1` | **CONDITIONAL** | Test clock sync and time-window rules; consume OTP once per accepted time step; audit drift/abuse |
| Trusted device cookie for 30 days | **CONDITIONAL** | Device token lifecycle, secret hashing server-side, revocation, device binding and step-up for high-risk actions; 30-day default requires security sign-off |
| Backup codes | **KEEP** | Independent unpredictable codes, cryptographic storage of verifiers, single-use and secure regeneration, user confirmation and recovery controls |
| Shared kiosk with employee printed static HMAC QR | **REJECT as sole authenticator** | Printable/static QR can be copied or replayed. Badge scanning alone is only identification; use approved time-bound challenge or additional independent proof and session binding as needed for sensitivity |
| Auto-logout kiosk after 10–15 seconds | **MODIFY** | Short-lived task session with explicit exit/clearing; protect against residual state, browser history and shared device cache; usability/security UAT |
| Foreman act-on-behalf | **KEEP with policy gate** | Foreman authenticates as themself, act-on-behalf is explicit, scoped, time-limited, auditable with actor_id, subject_id, reason, authority, consent where applicable; cannot self-approve or impersonate employee |
| PIN 4 digits / touch signature | **REJECT as standalone high-assurance confirmation** | Short PIN needs strong local/device-bound rate limits and independent identity assurance; drawn signature is not equivalent to cryptographic authentication |
| PostgreSQL DDL for users/logs | **ADAPT** | OrgAxis is Go + SQLite, already has `person`, `oa_user`, `oa_audit`; do not duplicate personnel or replace storage. Design additive migrations with current IDs and transaction semantics |
| Self-hosted GSM SMS | **OPTIONAL / DEFER** | Separate procurement, telecom compliance, SIM/operational costs and delivery evidence; claim of 0 VND maintenance is unverified |

External check: OWASP Authentication and Multifactor Authentication Cheat Sheets; NIST SP 800-63B-4 (July 2025). TOTP is not phishing-resistant. Two knowledge factors (password plus PIN) are not MFA. Security acceptance also depends on identity proofing, recovery and application/endpoint security.

## Recommended OrgAxis bounded contexts

**Core HR identity:** Existing `person` is person truth, `oa_user` is local account, `tenant_id` and effective employment scope remain authoritative. Separate credential lifecycle and enrollment from position/role; a promotion or transfer must not silently reset MFA.

**Onboarding / Access:**
- `oa_identity_enrollment`: user/account, enrollment method, status, activation hash, issued_at, expires_at, attempts, verification evidence, operator.
- `oa_auth_factor`: factor ID, account, factor type, encrypted secret reference or public key, enrollment date, last_used/revoked_at.
- `oa_recovery_code`: hashed code verifier, account, used_at, issued_at.
- `oa_trusted_device`: account, token verifier hash, device metadata (minimized), expires_at, revoked_at.
- `oa_delegation`: actor, subject, allowed operations, approved_by, reason, valid_from/to.
- `oa_kiosk_device` and `oa_kiosk_session`: registered device, scope, session timestamps and dedicated interaction audit.

These are **proposed** additive SQLite entities, not existing tables or completed migrations. Restrict access and retention for identity proofs, email and personal metadata.

## Workforce scenarios and scoped access

1. **Office worker with email:** Import only minimum identity/account fields; issue controlled one-time activation; enroll MFA; access rights only after verified completion.
2. **Worker without email/mobile:** Confirm identity through documented HR/authorized supervisor process, issue sealed/random one-time activation, enroll an available suitable factor, or use narrowly scoped assisted kiosk mode; do not infer smartphone possession.
3. **Worker with no smartphone or digital literacy:** Supervisor cannot know employee password/MFA secret. Use shared terminal for bounded workflows with independent confirmation for sensitive requests, explicit actor+subject logging, no impersonation.
4. **Shift leader / Foreman:** Supervisor uses own independently authenticated account; additional delegated authority from organization chart and policy; can prepare a request for employee but not silently approve salary/OT changes or employee consent.
5. **Payroll / HR Admin:** Stronger authentication and step-up for sensitive data export, payroll month close, factor reset, delegation creation and permission grants.

## API / state machine proposals

- States: `PENDING -> IDENTITY_VERIFIED -> ACTIVATION_ISSUED -> FACTOR_SETUP -> ACTIVE`; `EXPIRED`, `LOCKED`, `RECOVERY_PENDING`, `DISABLED` with explicit transitions.
- `POST /v2/identity/enrollments` (scoped HR), `POST /v2/identity/activate` (limited activation token), `POST /v2/identity/factors/totp/verify`, `POST /v2/identity/recovery` (approved identity recovery), `POST /v2/delegations` (independent approver), `POST /v2/kiosk/session`.
- Stable response schemas with generic failure for account enumeration; server-side authorization, CSRF, idempotent write, audit and transaction. Reject any HR/payroll read until authorized activation.
- Keep valid_time / transaction_time, active employment, department and approved role allocation independent from identity enrollment.

## Tests / AppGate

P0 security acceptance:
- Offline guessing, DOB-known attacker, account enumeration, multi-account spraying and lockout DoS; retries counted server side.
- Activation code replay, expiry boundary, stolen link, cross-account binding, reset/reissue and deactivated employee.
- TOTP enrollment takeover, secret exposure in URL/log/HTML/caches, ±1 window, duplicate code reuse, backup code atomic consumption, lost-device recovery.
- Shared kiosk replayed photograph or QR, browser leftover session, rapid user switch, privacy of payslips and timecards, session invalidation.
- Proxy: record actor AND subject, role/position date, approved delegation, self-approval denial, no bypass of payslip/loan/OT confirmation.
- Restore and migration of identity factors; hardware/Windows tests and independent security reviewer.

AppGate `enterprise-v2026.11.2`: identity SEC-01/02/03/04, DAT-01/02/03/06, BIZ-02/03/04, DEL-02, GOV-01/02/04, UX-01/02/03. Scores unchanged. A submitted KB is not a passed test or signed acceptance.

## Delivery slice

P0: preserve this KB; review security hazards; design privacy-minimal employee-ID-based activation; implement a pilot for email and no-email with **random supervised one-time code**, limited session, TOTP/recovery and audited actor/subject.
P1: kiosk device registration and sensitive-action step-up; bounded supervisor act-on-behalf with explicit delegation.
P2: trusted-device lifecycle and optional SMS, only after risk, cost and IT security review.

## References

- User-provided original KB preserved at `docs/kb/ONBOARDING-IDENTITY-2FA-MULTI-WORKFORCE-SOURCE.md`.
- https://pages.nist.gov/800-63-4/sp800-63b.html
- https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html
- https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html
- https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html
