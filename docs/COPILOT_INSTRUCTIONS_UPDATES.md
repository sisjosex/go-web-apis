# Copilot Instructions - Updates & Improvements

## Overview

The `.github/copilot-instructions.md` file has been updated to reflect best practices and patterns learned from the OTP (One-Time Password) multi-channel authentication implementation. These updates ensure consistency and clarity for future AI-assisted development.

## Changes Summary

### 1. **New Section: Pointer Types in DTOs** (Optional Fields)
**Location:** After "### DTO Binding Tags"

**Rationale:** The OTP system revealed a common pattern mistake where optional UUID fields failed validation because they were non-pointer types that received zero values instead of `nil`.

**Content:**
- Explanation of why pointers are needed for optional fields
- Proper DTO pattern with `*uuid.UUID` and `binding:"omitempty"`
- Validator implementation using `reflect` package
- Handling both pointer and direct type validation

**Why:** This prevents `omitempty` validation from silently failing when JSON unmarshals optional fields into zero values.

---

### 2. **New Section: Repository Scan Patterns**
**Location:** After "### Stored Procedures Drive Logic"

**Rationale:** The OTP implementation revealed a subtle but critical issue where stored procedures returning multiple columns must be scanned completely in Go repositories.

**Content:**
- Correct pattern: Scanning all columns including those to be ignored
- Common error: Skipping columns causes `sql: expected N columns, got M` errors
- Solution: Count RETURN QUERY columns in PostgreSQL and match exactly

**Why:** PostgreSQL drivers require scanning ALL columns returned by a stored procedure, even if the application doesn't use them.

---

### 3. **Comprehensive Section: OTP (One-Time Password) System**
**Location:** Before "## Build & Deployment Strategy"

**Rationale:** The OTP implementation is a complete, production-ready system that demonstrates multiple architecture patterns used in the project.

**Subsections:**

#### a. Architecture Overview
- Visual representation of the multi-channel OTP flow
- Strategy Pattern with pluggable providers
- Database integration

#### b. Key Components
1. OTP Provider Interface definition
2. Database schema with sp_request_otp and sp_verify_otp
3. **Critical: Type Casting Rules** - CAST requirements for PostgreSQL
4. **Critical: Row Aliases** - Prevent ambiguous column references
5. Multilingual support with context-based language detection

#### c. Configuration Pattern
- AuthConfig struct with OTP-specific variables
- Environment variables structure

#### d. API Endpoints
- Channel-specific request endpoints (email, whatsapp, sms)
- Universal verify endpoint

#### e. Error Codes Reference
- Complete error code documentation
- PostgreSQL exception codes
- Error code to HTTP status mapping

#### f. Common Issues & Solutions (Troubleshooting Guide)
- Type mismatch errors and fixes
- Ambiguous column references
- ON CONFLICT constraints
- Repository scan mismatches

---

### 4. **New Section: Provider/Strategy Pattern for Pluggable Features**
**Location:** Before "## Build & Deployment Strategy"

**Rationale:** The OTP system showcases the Strategy Pattern which is reusable for many pluggable features (notifications, payment providers, etc.).

**Content:**
- When to use the Strategy Pattern
- Step-by-step example: OTP Providers implementation
- 4-step workflow: Interface → Implementations → Registration → Runtime Selection
- Benefits and use cases
- Context propagation pattern for configuration

**Why:** This pattern enables flexible, configuration-driven, easily-testable feature implementations.

---

## Impact on Future Development

### For OTP/Authentication
- Developers now have a complete reference for OTP implementation patterns
- Troubleshooting guide helps resolve PostgreSQL type issues quickly
- Configuration and API endpoint documentation is in one place

### For Multi-Channel Systems
- The Provider/Strategy pattern section provides a blueprint for:
  - Notification systems (email, SMS, push)
  - Payment providers (Stripe, PayPal, etc.)
  - Storage backends (S3, GCS, local)
  - Auth providers (Google, GitHub, Facebook)

### For PostgreSQL Development
- Type casting rules prevent hours of debugging
- Row alias pattern documentation prevents ambiguous column errors
- Repository scan pattern documentation prevents silent failures

### For DTOs & Validation
- Pointer types documentation clarifies optional field handling
- Prevents incorrect validation logic in future DTOs

---

## File Statistics

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Total Lines | 661 | 978 | +317 lines (+48%) |
| Sections | 13 | 18 | +5 major sections |
| Code Examples | ~20 | ~35 | +15 examples |
| Troubleshooting Entries | 0 | 4 | +4 (new section) |

---

## Backward Compatibility

✅ **All previous content preserved** - No existing sections were removed or significantly altered
✅ **Additive only** - New content is added in logical places with clear structure
✅ **Consistent formatting** - Follows existing markdown style and conventions

---

## Recommended Next Steps

1. **Use OTP section as template** - When adding new multi-channel systems, reference the OTP implementation patterns
2. **Extend Provider/Strategy section** - As new pluggable features are added (notifications, payments), document them alongside OTP
3. **Keep troubleshooting guide updated** - As new issues are encountered and solved, add them to the respective troubleshooting sections
4. **Review quarterly** - Every quarter, review the instructions for patterns that should be documented

---

## Key Learnings Documented

1. **PostgreSQL Type System** - TEXT vs VARCHAR, explicit casting required in RETURN QUERY
2. **Ambiguous References** - Always use table aliases in SELECT queries
3. **Pointer Semantics** - Optional fields must use pointer types to maintain nil vs zero-value distinction
4. **Strategy Pattern** - Pluggable implementations via interface-based provider registry
5. **Context Propagation** - Passing runtime configuration through context.Context
6. **Multi-language Support** - Dynamic template translation via context and language middleware
7. **Repository Patterns** - Must scan all SP columns, not just the ones used
8. **Error Code Mapping** - Extracting PostgreSQL error codes and mapping to HTTP status codes

---

## Document Location

📄 **File:** `.github/copilot-instructions.md`

Use this document as the authoritative source for project architecture, patterns, and best practices when working with GitHub Copilot or other AI-assisted development tools.
