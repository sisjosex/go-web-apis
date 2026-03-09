# Purchase Orders (Purchasing) Module - Complete Guide

## 📋 What is a Purchase Order?

A **Purchase Order (PO)** is a formal request to buy goods from suppliers. It's the opposite flow of Sales Orders:

```
SALES ORDER                          PURCHASE ORDER
┌─────────────────────────┐         ┌─────────────────────────┐
│ YOU SELL TO CUSTOMER    │         │ YOU BUY FROM SUPPLIER   │
│                         │         │                         │
│ Dinero ENTRA ➡️          │         │ ➡️ Dinero SALE          │
│ Stock SALE              │         │ Stock ENTRA             │
│ Accounts Receivable     │         │ Accounts Payable        │
└─────────────────────────┘         └─────────────────────────┘
```

---

## 🏗️ Module Architecture

### **Database Layer**
```
purchasing schema:
├── suppliers              (✓ Where you buy from)
├── supplier_products      (✓ What cost per supplier)
├── purchase_orders        (✓ Your POs)
├── purchase_order_items   (✓ Items in each PO)
├── purchase_order_receipts (✓ When goods arrive)
└── purchase_order_invoices (✓ Supplier invoices for payment)
```

### **Go Layer**
```
Repository → Service → Controller → Routes
   ├── CRUD suppliers
   ├── CRUD purchase orders
   ├── Add items to POs
   ├── Receive goods
   ├── Manage supplier invoices
   └── Reports (Accounts Payable)
```

---

## 🔄 Complete Purchase Order Workflow (Real Example)

### **Scenario: Pulpería needs to buy Jugo "Naturales"**

#### **STEP 1: Create a Supplier**
```bash
POST /api/v1/purchasing/suppliers
{
  "name": "Jugos Don Pepe",
  "contact_person": "Carlos Soto",
  "email": "pedidos@jugodonpepe.com",
  "phone": "+506 8888 9999",
  "address": "San José, Barrio Escalante",
  "payment_terms": 30  # Days credit
}

Response:
{
  "supplier_id": "uuid-1",
  "name": "Jugos Don Pepe",
  "message": "Supplier created successfully"
}
```

#### **STEP 2: Create a Purchase Order (DRAFT)**
```bash
POST /api/v1/purchasing/purchase-orders
{
  "supplier_id": "uuid-1",
  "expected_delivery_date": "2026-03-15",
  "notes": "Reabastecimiento semanal"
}

Response:
{
  "po_id": "uuid-po-123",
  "po_number": "PO-2026-00001",
  "supplier_id": "uuid-1",
  "status": "draft",
  "total_amount": 0,
  "message": "Purchase order created successfully"
}
```

#### **STEP 3: Add Items to the PO**
```bash
POST /api/v1/purchasing/purchase-orders/uuid-po-123/items
{
  "product_id": "uuid-jugo-naturales",
  "quantity": 50,              # 50 cajas
  "unit_cost": 9000           # ¢9,000 por caja
}

Response:
{
  "item_id": "uuid-item-1",
  "po_id": "uuid-po-123",
  "product_id": "uuid-jugo-naturales",
  "quantity": 50,
  "unit_cost": 9000,
  "line_total": 450000,       # 50 × 9,000
  "message": "Item added to purchase order"
}

Status: Still DRAFT
Total: ¢450,000 (calculated automatically)
```

#### **STEP 4: Approve the PO**
```bash
PATCH /api/v1/purchasing/purchase-orders/uuid-po-123/approve
{}

Response:
{
  "po_id": "uuid-po-123",
  "po_number": "PO-2026-00001",
  "status": "approved",         # Now it's official!
  "total_amount": 450000,
  "message": "Purchase order approved"
}
```

#### **STEP 5: Goods Arrive - RECEIVE**
```bash
PATCH /api/v1/purchasing/purchase-orders/uuid-po-123/receive
{
  "received_by": "user-uuid",   # Who received it
  "notes": "Todas las cajas llegaron en buen estado"
}

Response:
{
  "receipt_id": "uuid-receipt-1",
  "receipt_number": "RCP-20260306-12345",
  "message": "Purchase order received"
}

✅ Automatic: 
   - PO status → "received"
   - All items marked as received
   - Stock quantity updated in inventory
```

#### **STEP 6: Supplier Invoice Arrives - RECORD INVOICE**
```bash
POST /api/v1/purchasing/purchase-orders/uuid-po-123/invoices
{
  "invoice_number": "FAC-002345",      # On supplier invoice
  "invoice_date": "2026-03-10",
  "invoice_amount": 450000,            # They want ¢450,000
  "tax_amount": 0,                     # No additional tax
  "due_date": "2026-04-09"             # Pay within 30 days
}

Response:
{
  "id": "uuid-invoice-1",
  "purchase_order_id": "uuid-po-123",
  "invoice_number": "FAC-002345",
  "invoice_date": "2026-03-10",
  "invoice_amount": 450000,
  "due_date": "2026-04-09",
  "status": "received",                # Received from supplier
  "message": "Invoice recorded"
}

Status: "received" → "validated" → "partially_paid" → "paid"
```

#### **STEP 7: Pay Supplier (Accounts Payable)**
```bash
# Check what you owe
GET /api/v1/purchasing/pending-payments

Response:
{
  "pending_payments": [
    {
      "id": "uuid-po-123",
      "po_number": "PO-2026-00001",
      "supplier_id": "uuid-1",
      "total_amount": 450000,
      "paid_amount": 0,
      "invoices": [
        {
          "invoice_number": "FAC-002345",
          "due_date": "2026-04-09",    # Pay by this date!
          "invoice_amount": 450000,
          "status": "received"
        }
      ]
    }
  ],
  "total_count": 1
}

# Then: Make payment via bank transfer
# Update status to "paid" when payment sent
```

---

## 📊 Complete API Endpoints

### **Suppliers**
```
POST   /api/v1/purchasing/suppliers                          Create supplier
GET    /api/v1/purchasing/suppliers                          List suppliers
GET    /api/v1/purchasing/suppliers/{id}                     Get supplier details
PATCH  /api/v1/purchasing/suppliers/{id}                     Update supplier (ready to implement)
DELETE /api/v1/purchasing/suppliers/{id}                     Soft delete supplier (ready to implement)
```

### **Purchase Orders**
```
POST   /api/v1/purchasing/purchase-orders                    Create PO
GET    /api/v1/purchasing/purchase-orders                    List POs (filter by status)
GET    /api/v1/purchasing/purchase-orders/{id}               Get PO details + items

PATCH  /api/v1/purchasing/purchase-orders/{id}/approve       draft → approved
PATCH  /api/v1/purchasing/purchase-orders/{id}/receive       approved → received (+ inventory update)

POST   /api/v1/purchasing/purchase-orders/{id}/items         Add item to PO
POST   /api/v1/purchasing/purchase-orders/{id}/invoices      Record supplier invoice

GET    /api/v1/purchasing/pending-payments                   Accounts Payable report
```

---

## 💡 How PO Data Flows to Other Modules

### **Stock Costing (Inventory)**
```
Purchase Order (cost ¢9,000/unit)
         ↓
Receive Goods (inventory.products stock increases)
         ↓
Sales Order (sell @ ¢12,000/unit)
         ↓
Margen = ¢12,000 - ¢9,000 = ¢3,000/unit profit
```

### **Financial Reports**
```
Accounts Payable:
├── Outstanding Invoices (due dates)
├── Payment Terms (30, 60 days)
├── Aging Report (30+ days overdue)
└── Cash Flow Forecast

Cost Analysis:
├── Cost per unit by supplier
├── Price trends over time
├── Supplier performance
└── Margin analysis by product
```

---

## ⚙️ Database Details

### **Purchase Order Statuses**
```
draft        → Editing, no items yet
  ↓
approved     → Sent to supplier (official)
  ↓
received     → Goods arrived, all items received
  ↓
invoiced     → Supplier invoice recorded
  ↓
paid         → Payment sent to supplier
  ↓
cancelled    → Cancelled before payment
```

### **Invoice Statuses**
```
received         → Invoice from supplier (waiting validation)
validated        → Checked against PO
partially_paid   → Partial payment sent
paid             → Full payment sent
disputed         → Difference with PO
```

---

## 🔧 Configuration (.env)

```env
# Enabled modules must include purchasing
ENABLED_MODULES=core,auth,users,tenancy,tracking,inventory,sales,purchasing

# Purchasing specific (optional)
PURCHASING_ENABLE_AUTO_RECEIVE=false    # Auto-receive goods? (false = manual)
PURCHASING_DEFAULT_PAGE_SIZE=20         # Default pagination
```

---

## 📁 Module Files Structure

```
modules/purchasing/
├── config/config.go                    → Module configuration
├── models/
│   ├── purchasing_models.go            → Domain entities
│   └── purchasing_dtos.go              → API request/response DTOs
├── interfaces/purchasing_interfaces.go → DI contracts
├── repositories/purchasing_repository.go → DB operations
├── services/purchasing_service.go      → Business logic
├── controllers/purchasing_controller.go → HTTP handlers
├── routes/purchasing_routes.go         → Route registration
├── errors/errors.go                    → Error constants (16 types)
├── lang/
│   ├── en.json                         → English translations
│   └── es.json                         → Spanish translations
├── migrations/                          → 8 database migration files
└── tests/purchasing_test.go            → Integration tests
```

---

## ✅ What's Implemented

### Database
- ✅ 8 migrations (schema, suppliers, products, POs, items, receipts, invoices, SPs)
- ✅ 3 stored procedures (create PO, add item, approve PO, receive goods)

### Go Layer
- ✅ Repository: Full CRUD + 10 operations
- ✅ Service: Delegation pattern
- ✅ Controller: 10 HTTP handlers + Swagger docs
- ✅ Routes: All endpoints wired
- ✅ Config: Module configuration
- ✅ Errors: 16 error codes + translations (EN/ES)
- ✅ Tests: 5 integration tests + 1 summary test

---

## 🚀 What You Can Build Next

### Immediate (Useful Today)
- **Update Supplier** - PATCH endpoint
- **Delete Supplier** - Soft delete with is_active flag
- **Pagination** - Already partially implemented
- **Supplier Search** - By name, email, contact

### Short Term (Next Phase)
- **Supplier Price History** - Track cost changes over time
- **Reorder Alerts** - "This supplier's price went up"
- **Automatic PO Creation** - Based on inventory reorder levels
- **Invoice Matching** - Auto-validate PO ↔ Invoice ↔ Receipt
- **CpE Integration** - Generate supplier expense receipts for tax

### Medium Term
- **Multi-currency Support** - Buy from suppliers in different currencies
- **Supplier Ratings** - Track on-time delivery, quality
- **Budget Controls** - Budget per supplier / category
- **Purchase Requisitions** - Employee requests → Manager approval → PO
- **RFQ (Request for Quote)** - Get prices from multiple suppliers

---

## 🧪 Testing

All tests are included. Run:
```bash
make test                              # All tests including purchasing
go test -v -tags=integration ./modules/purchasing/tests
```

---

## 📈 Why This Matters for "Mi Pulpería"

**Before Purchase Orders:**
- ❌ Mama never knows what she paid for stock
- ❌ Can't calculate real profit margin
- ❌ No way to track "we owe supplier ¢50,000"
- ❌ Can't make purchasing decisions

**After Purchase Orders:**
- ✅ Cost per unit is recorded
- ✅ Real profit margin per product (¢12,000 - ¢9,000 = margin)
- ✅ Automatic accounts payable (who to pay, how much, when)
- ✅ Data-driven purchasing (detect suppliers, reorder timely)
- ✅ Cash flow forecasting (know payment obligations)

---

## 🎯 Real Example: Making Better Decisions

```
Supplier "Don Pepe" Juice Analysis:

PO History:
├── PO-2026-00001: ¢9,000/caja (30 days credit) ✅ Always on time
├── PO-2026-00002: ¢9,200/caja (30 days credit) ⚠️ Price went up!
└── PO-2026-00003: ? (pending)

Current Stock: 5 cajas @ ¢12,000 = potential ¢15,000 profit
Next Order Needed: 20 cajas (once stock hits 5 units)

Decision:
- Cost is rising (9,000 → 9,200)
- Margin shrinking (3,000 → 2,800 per unit)
- Consider cheaper supplier OR raise retail price

This data comes from Purchase Orders!
```

---

## ⚖️ Legal Compliance (Costa Rica)

Purchase Orders help with:
- 📋 **Auditable Purchase Trail** - When auditor asks: "How much did you spend?"
- 💰 **Tax Deductions** - Expenses documented
- 🔐 **Supplier Verification** - Licensed vendors only
- 📊 **Financial Reports** - Accounts Payable section

All PO data can feed into legal invoicing when combined with Billing module.

