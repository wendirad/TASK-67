# Questions

## 1. Member Identity and Account Model
**Question:** The prompt describes Regular Members performing registration, checkout, and check-in actions, but it does not specify how members are uniquely identified and authenticated across sessions and transactions.
**Assumption:** Members are persistent authenticated users with unique accounts stored in the system and all actions are tied to a stable member ID.
**Solution:** Implemented a user identity model with persistent member accounts, ensuring all registrations, orders, check-ins, and moderation actions are linked to a unique user ID for consistency and traceability.

## 2. Session Seat Ownership and Release Logic
**Question:** The prompt defines seat utilization, temporary leave rules, and automatic release, but it does not define how seat ownership is tracked during active sessions.
**Assumption:** Each seat is explicitly assigned to a registered member upon check-in and remains reserved unless released by rule violations or session completion.
**Solution:** Implemented seat assignment tracking at check-in time with state transitions that enforce break limits and automatically release seats when violation thresholds are exceeded.

## 3. Waitlist Promotion Ownership and Timing Guarantee
**Question:** The prompt requires automatic promotion of waitlisted users within 30 seconds after seat release, but it does not define how fairness or ordering is enforced.
**Assumption:** Waitlist operates as a strict FIFO queue per session.
**Solution:** Implemented a queue-based waitlist system where seat release triggers a transactional promotion job that assigns the next eligible member in FIFO order within the defined time window.

## 4. Registration State Transitions
**Question:** The prompt lists multiple registration states but does not define valid transitions between them.
**Assumption:** Registration follows a controlled lifecycle with predefined transitions (e.g., Pending → Approved/Rejected → Registered → Completed/Canceled).
**Solution:** Implemented a state machine for registrations with strict transition validation to ensure consistency across UI and backend operations.

## 5. Seat Deduction Atomicity Scope
**Question:** The prompt requires atomic seat deduction but does not define whether deduction occurs at registration time, payment time, or check-in.
**Assumption:** Seat deduction occurs at successful registration confirmation to guarantee availability.
**Solution:** Implemented seat reservation within a database transaction at registration confirmation, ensuring consistency and preventing overselling.

## 6. Order Lifecycle State Model
**Question:** The prompt describes order creation, payment countdown, auto-close, delivery, and refund sync, but it does not define the full lifecycle states.
**Assumption:** Orders follow a structured lifecycle such as Created → Pending Payment → Paid → Fulfilled → Closed/Refunded.
**Solution:** Implemented a state-driven order lifecycle with immutable audit logging for each transition.

## 7. Payment Timeout Enforcement
**Question:** The prompt specifies a 15-minute payment countdown but does not define the enforcement mechanism.
**Assumption:** Timeout is enforced by backend scheduled jobs rather than relying on frontend timers.
**Solution:** Implemented a scheduled job that scans pending orders and auto-closes expired ones based on server-side timestamps.

## 8. WeChat Pay Callback Handling
**Question:** The prompt requires signature verification but does not define how callbacks are reconciled with existing orders.
**Assumption:** Each callback maps to a unique order reference and must be idempotent.
**Solution:** Implemented callback verification with signature validation and idempotent order updates to prevent duplicate processing.

## 9. Check-In Verification Priority
**Question:** The prompt allows QR scan and optional Bluetooth beacon but does not define precedence when both are available.
**Assumption:** QR scan is the primary method, with Bluetooth as a secondary validation layer when enabled.
**Solution:** Implemented QR-based check-in as the default flow, with optional beacon verification enhancing validation but not replacing it.

## 10. No-Show and Late Arrival Handling
**Question:** The prompt defines no-show after 10 minutes but does not specify whether late arrivals can reclaim seats.
**Assumption:** After no-show threshold, the seat is permanently released and cannot be reclaimed.
**Solution:** Implemented automatic status transition to no-show with immediate seat release and waitlist promotion.

## 11. Temporary Leave Enforcement Logic
**Question:** The prompt defines break rules but does not specify how time tracking is enforced.
**Assumption:** Leave duration is tracked server-side based on timestamp events.
**Solution:** Implemented leave tracking using check-out/check-in timestamps and enforced rule validation on re-entry.

## 12. Exception Ticket Trigger Conditions
**Question:** The prompt mentions prolonged unverified occupancy but does not define thresholds.
**Assumption:** Occupancy without validation beyond a predefined inactivity threshold triggers an exception.
**Solution:** Implemented monitoring logic that raises exception tickets when expected verification events are missing within allowed intervals.

## 13. Shipping Order Completion Criteria
**Question:** The prompt defines proof-of-delivery but does not define when an order is considered fulfilled.
**Assumption:** Order is fulfilled once delivery confirmation is recorded with valid proof.
**Solution:** Implemented fulfillment status transition triggered by delivery confirmation with stored evidence.

## 14. Moderation Decision Finality
**Question:** The prompt defines moderation actions but does not specify whether decisions are reversible.
**Assumption:** Moderation actions are final but auditable.
**Solution:** Implemented immutable moderation decision records with audit logs rather than editable states.

## 15. Posting Frequency Enforcement Scope
**Question:** The prompt defines posting limits but does not specify enforcement scope.
**Assumption:** Limits are enforced per user account within a rolling time window.
**Solution:** Implemented rate-limiting logic using timestamp tracking to enforce posting frequency constraints.

## 16. KPI Metric Calculation Scope
**Question:** The prompt defines KPIs but does not specify calculation granularity.
**Assumption:** Metrics are computed per facility and aggregated for reporting.
**Solution:** Implemented metric aggregation jobs that compute KPIs based on stored transactional data.

## 17. Exception Ticket Workflow Model
**Question:** The prompt defines SLA-driven ticket workflows but does not specify lifecycle states.
**Assumption:** Tickets follow a lifecycle such as Open → Assigned → In Progress → Resolved → Closed.
**Solution:** Implemented a workflow engine with SLA tracking and escalation triggers.

## 18. SLA Time Calculation Rules
**Question:** The prompt defines SLA targets but does not specify business hour handling.
**Assumption:** SLA uses configured business hours and excludes non-working periods.
**Solution:** Implemented SLA timers using business calendar rules and scheduled escalation checks.

## 19. Import/Export Duplicate Detection Rules
**Question:** The prompt requires duplicate detection but does not define matching logic.
**Assumption:** Duplicate detection is based on key business identifiers depending on entity type.
**Solution:** Implemented validation rules with configurable uniqueness constraints during file import.

## 20. File Fingerprinting Scope
**Question:** The prompt requires file fingerprinting but does not define scope.
**Assumption:** Fingerprints are applied to uploaded/imported files for integrity and deduplication.
**Solution:** Implemented SHA-based fingerprinting for all processed files with storage-level validation.

## 21. Async Queue Processing Guarantees
**Question:** The prompt requires async task queues but does not define execution guarantees.
**Assumption:** Tasks must be idempotent and retry-safe.
**Solution:** Implemented queue workers with retry logic and idempotency checks to ensure consistency.

## 22. Canary Release Behavior
**Question:** The prompt defines canary release by percentage but does not define assignment method.
**Assumption:** Users are deterministically assigned to cohorts based on stable identifiers.
**Solution:** Implemented percentage-based rollout using consistent hashing on user IDs.

## 23. Backup and Restore Scope
**Question:** The prompt requires encrypted backups and point-in-time restore but does not define coverage.
**Assumption:** Full database and critical file storage are included in backups.
**Solution:** Implemented scheduled encrypted backups with restore checkpoints and recovery procedures.

## 24. Data Archiving Boundaries
**Question:** The prompt defines archiving rules but does not specify what data remains accessible.
**Assumption:** Archived data is moved to a separate schema but remains queryable for reporting.
**Solution:** Implemented archival pipelines that migrate old records while preserving masked reference fields for analytics.

## 25. Offline System Boundary
**Question:** The prompt states no external network dependency but includes WeChat Pay logic.
**Assumption:** All payment processing and verification occur within the internal network using locally stored keys and simulated callbacks.
**Solution:** Implemented fully local payment verification flows without external API calls, ensuring system operability in a closed network.
