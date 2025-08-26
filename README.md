# Simple Bank Reconciliation (Go)

## 📌 Overview

This service reconciles **system transactions** (internal data) against **bank statements** (external data).  
It identifies:

- ✅ **Matched transactions** (same date + sign, nearest amount)
- ❌ **Unmatched transactions** (system present but missing in bank, or vice versa)
- ⚖️ **Discrepancies** (sum of absolute amount differences between matched pairs)

The reconciliation process helps detect errors, discrepancies, and missing transactions across multiple bank accounts.

---

## 🗂 Data Model

**System Transaction**
- `trxID` – unique ID (string)
- `amount` – decimal (positive)
- `type` – enum: `DEBIT` or `CREDIT`
- `transactionTime` – datetime (RFC3339)

**Bank Statement**
- `unique_identifier` – unique ID from the bank (string)
- `amount` – decimal (negative for debits, positive for credits)
- `date` – date only (YYYY-MM-DD)
- `bank` – bank name (string, optional; inferred from filename if missing)

---

## 📂 Project Structure
```
go-reconcile/                   # project root (Go module)
│
├── go.mod                      # module name + Go version
├── README.md                   # documentation, usage, design notes
│
├── reconcile/                  # core library package (business logic)
│   ├── models.go               # data models (Transaction, BankStatement),
│   │                           # CSV parsing, Money type
│   ├── reconciler.go           # reconciliation algorithm (match, discrepancies, grouping)
│   └── reconciler_test.go      # unit tests for reconciler
│
├── cmd/                        # CLI applications
│   └── reconcile/              # "reconcile" CLI app
│       └── main.go             # command-line entrypoint, JSON output
│
└── samples/                    # example input CSVs
    ├── system_transactions.csv # system-side transactions (DEBIT/CREDIT, timestamped)
    ├── alpha_bank.csv          # sample bank statement (Bank A)
    └── beta_bank.csv           # sample bank statement (Bank B)
```
---
## ▶️ Usage
### Run CLI
From the project root:

```bash
    go run ./cmd/reconcile \
  --system samples/system_transactions.csv \
  --bank "samples/bank_a_transaction.csv,samples/bank_b_transaction.csv" \
  --start 2025-08-01 \
  --end 2025-08-02 \
  --output-json summary.json
 
```

example sample result:

```
{
  "matched_count": 3,
  "total_bank_transactions": 4,
  "total_discrepancy": "0.02",
  "total_processed": 7,
  "total_system_transactions": 3,
  "unmatched_bank_by_name": {
    "BankA": [
      {
        "unique_identifier": "b1",
        "amount": "75.00",
        "date": "2025-08-02",
        "bank": "BankA"
      }
    ]
  },
  "unmatched_system": [],
  "unmatched_total": 1
}
```
---

## ⚙️ Assumptions

- Discrepancies only occur in amount.
- Matching is done by transaction date (not timestamp) and sign:
  - DEBIT → negative
  - CREDIT → positive
- If multiple candidates exist, the nearest amount is chosen.
- CSV headers must exactly match expected column names:
  - System: trxID,amount,type,transactionTime
  - Bank: unique_identifier,amount,date[,bank]
---
## 🚀 Next Steps (if needed)

- Add amount tolerance (e.g. ±0.50 allowed difference).
- Stream CSVs instead of fully loading (for very large files).
- Add parallel reconciliation by bucket for huge datasets.
- Expose as an HTTP API service using the same library package.

## 🧭 Diagrams

```
+------------------------+        +-----------------------+        +--------------------+
| System CSV             |        | Bank CSV(s)           |        | Start/End Date     |
| trxID, amount, type,   |        | unique_identifier,    |        | (YYYY-MM-DD)       |
| transactionTime (RFC3339)       | amount, date[, bank]  |        +---------+----------+
+-----------+------------+        +-----------+-----------+                  |
            |                                 |                              |
            v                                 v                              v
   Parse & Validate                   Parse & Validate                Filter transactions
  (ParseSystemCSV)                   (ParseBankCSV)                   into [start, end]
            \______________________________  _______________________________/
                                           \/
                                  Bucket by (date, sign)
                                        |
                                        v
                           Match within bucket by nearest amount
                           (sum |Δ| as discrepancy over matches)
                                        |
                                        v
                         Aggregate summary (matched, unmatched, discrepancy)
                                        |
                                        v
                            JSON to stdout (+ optional file)
```