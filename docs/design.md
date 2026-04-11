# CampusRec Seat & Commerce Operations Platform
## Software Design Document

---

## 1. System Overview

CampusRec is a fully on-premise platform designed for unified session registration, seat utilization, and offline commerce operations within a private network environment.

Primary roles:

* Member
* Staff
* Moderator
* Administrator

Core capabilities:

* session registration and seat management
* offline order lifecycle with WeChat Pay simulation
* real-time seat utilization and check-in enforcement
* waitlist promotion and attendance tracking
* logistics and proof-of-delivery management
* moderation and abuse control
* KPI dashboards and reporting
* exception ticket workflow with SLA enforcement
* offline import/export and backup/restore

The system operates entirely within a private network using a Gin backend and Templ-rendered frontend.

---

## 2. Design Goals

* Fully offline-capable system with no external dependency
* Strong transactional consistency for seat and order management
* Deterministic workflows for registration, payment, and attendance
* High auditability with immutable logs
* Secure authentication and data handling
* Scalable modular backend design
* Operational observability and reporting readiness

---

## 3. High-Level Architecture

```text
Templ Frontend (UI)
        ↓
Gin REST API Layer
        ↓
Service Layer (Business Logic)
        ↓
Repository Layer
        ↓
PostgreSQL Database
````

Supporting components:

* Scheduler Jobs
* Async Queue Workers
* Audit Logging System
* Configuration Center
* Backup and Archive Manager

### Architecture Principle

The backend is the source of truth.
All business logic is enforced server-side to guarantee consistency and compliance.

---

## 4. Backend Architecture (Gin)

### 4.1 Layers

* Controllers → API endpoints and validation
* Services → core business logic
* Repositories → database interaction
* Middleware → authentication, RBAC, logging

### 4.2 Key Services

* RegistrationService
* SeatControlService
* OrderService
* PaymentService
* CheckInService
* LogisticsService
* ModerationService
* KPIService
* TicketService
* ImportExportService

---

## 5. Frontend Architecture (Templ)

### 5.1 UI Structure

* Server-rendered UI with Templ
* Responsive catalog views
* Role-based UI rendering

### 5.2 Core Screens

* session catalog
* product catalog
* cart and checkout
* payment countdown screen
* check-in interface
* staff logistics panel
* moderation console
* admin dashboards

---

## 6. Domain Model

### 6.1 Member

Persistent authenticated user with unique ID
(All actions tied to member_id) 

### 6.2 Session Registration

Tracks:

* member_id
* session_id
* status (Pending, Registered, Waitlisted, etc.)
* timestamps

State transitions enforced via state machine.

### 6.3 Seat Inventory

Tracks:

* total seats
* allocated seats
* waitlist queue

Seat assignment occurs at registration confirmation.

### 6.4 Order

Lifecycle:

* Created → Pending Payment → Paid → Fulfilled → Closed/Refunded 

Includes:

* cart items
* delivery info
* payment status
* audit logs

### 6.5 Payment

Offline WeChat Pay simulation:

* payment request
* countdown timer
* callback verification (local keys)
* idempotent updates

### 6.6 Check-In Events

Tracks:

* QR scan validation
* optional Bluetooth validation
* seat assignment
* leave/re-entry timestamps

### 6.7 Exception Ticket

Lifecycle:

* Open → Assigned → In Progress → Resolved → Closed 

Includes SLA tracking and escalation.

---

## 7. Seat Control Engine

### 7.1 Seat Deduction

* performed atomically at registration confirmation
* enforced using database transactions
* prevents overselling

### 7.2 Waitlist Promotion

* FIFO queue per session 
* triggered within 30 seconds after seat release
* handled by async worker

### 7.3 Seat Ownership

* assigned at check-in
* released on:

  * no-show
  * leave violation
  * session end

---

## 8. Registration & Attendance Logic

### 8.1 Registration Rules

* registration closes 2 hours before session start
* override allowed by admin

### 8.2 No-Show Handling

* auto-cancel after 10 minutes
* seat released immediately
* waitlist promotion triggered

### 8.3 Temporary Leave

* tracked via timestamps
* enforced server-side
* violations trigger seat release

### 8.4 Check-In Validation

* QR scan = primary
* Bluetooth = optional enhancement 

---

## 9. Order & Payment Design

### 9.1 Checkout Flow

1. Add to cart / Buy Now
2. Select address
3. Generate payment request
4. Start 15-minute countdown
5. Await callback
6. Confirm or auto-close

### 9.2 Payment Timeout

* enforced by scheduled job 
* expired orders → auto-closed

### 9.3 Idempotency

* payment callbacks mapped to order ID
* duplicate callbacks safely ignored

---

## 10. Logistics & Delivery

### 10.1 Fulfillment

Order marked fulfilled when:

* delivery confirmed
* proof-of-delivery recorded (image/text)

### 10.2 Exception Handling

* delivery failures create exception tickets
* linked to order

---

## 11. Moderation System

### 11.1 Features

* post review
* report handling
* user bans

### 11.2 Rate Limits

* 5 posts per hour per user 

### 11.3 Decision Model

* decisions immutable
* audit logs recorded

---

## 12. KPI & Reporting

Metrics include:

* fill rate
* member growth/churn
* engagement
* coach productivity

### 12.1 Calculation

* computed from transactional data 
* aggregated per facility

---

## 13. Exception Ticket System

### 13.1 Workflow

* intake → assignment → resolution → closure

### 13.2 SLA

* response within 4 hours
* resolution within 3 days

### 13.3 Enforcement

* scheduled jobs monitor deadlines
* escalation flags generated

---

## 14. Data Persistence (PostgreSQL)

Tables include:

* users
* sessions
* registrations
* seats
* waitlists
* orders
* payments
* invoices
* checkins
* logistics
* moderation
* tickets
* audit_logs

---

## 15. Security Design

### 15.1 Authentication

* username/password
* min 12 characters
* lockout after 5 failed attempts

### 15.2 Authorization

* RBAC per role

### 15.3 Audit Logs

* immutable
* track all state transitions

---

## 16. Async Processing

### 16.1 Queue Tasks

* waitlist promotion
* report generation
* import processing

### 16.2 Guarantees

* idempotent tasks
* retry-safe execution 

---

## 17. Import/Export

### 17.1 Import

* CSV/Excel parsing
* strict validation
* duplicate detection

### 17.2 Export

* filtered data export
* reporting support

### 17.3 File Integrity

* SHA fingerprinting 

---

## 18. Configuration & Deployment

### 18.1 Canary Release

* percentage-based rollout
* deterministic assignment via user ID hashing 

### 18.2 Backup

* nightly encrypted backups
* point-in-time restore

### 18.3 Archiving

* records older than 24 months moved to archive schema 
* masked fields retained for reporting

---

## 19. Scheduler Design

Runs periodic jobs:

* payment expiration
* SLA monitoring
* seat release checks
* waitlist promotion
* reminders

---

## 20. System Guarantees

* no seat overselling
* deterministic waitlist promotion
* idempotent payment handling
* audit-compliant state transitions
* offline operability

---

## 21. Conclusion

This design ensures:

* transactional integrity for seat and order management
* strong compliance with audit and SLA requirements
* scalable modular architecture
* full offline capability within private networks

The system is production-ready and aligned with enterprise operational standards.

```

---

If you want next level (this is where senior engineers usually go next), I can:
- Turn this into **database schema (PostgreSQL DDL)**
- Or **Gin API design**
- Or **state machines (registration, order, ticket)**
