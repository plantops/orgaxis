# OrgAxis — Temporal Organization & Workforce Engine

**People · Structure · Authority · Time**

API-first HR organizational core supporting date-effective reporting relationships, blank/occupied org charts, positions, assignments, and workflow routing.

## Design rules

- Person identity is independent from employment, position, and reporting relationship.
- Blank org structure has its own timeline, independent of people.
- A position reports to another position; people occupy positions for date-effective intervals.
- Calendar date queries use half-open intervals `[valid_from, valid_to)`.
- Reporting hierarchy is not the same as workflow approval authority.
- Sensitive HR domains (payroll, health, immigration, discipline) are separate modules.
- Tenant isolation and authorization are prerequisites for any production SaaS deployment.

## First milestone

Seed 1,000 **synthetic** people, two blank-org changes and 80 employee movements; support `as_of` queries for direct manager, reporting chain, vacant positions, and route resolution.

Implementation is experimental. Do not import real employee records or expose demo credentials to a public network.
