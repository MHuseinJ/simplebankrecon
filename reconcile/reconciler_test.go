package reconcile

import (
	"testing"
	"time"
)

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBasicMatchAndDiscrepancy(t *testing.T) {
	sys := []Transaction{
		{TrxID: "TX1", AmountCents: 10000, Type: DEBIT, TransactionTime: mustTime("2025-08-01T10:00:00Z")},
		{TrxID: "TX2", AmountCents: 5000, Type: CREDIT, TransactionTime: mustTime("2025-08-01T11:00:00Z")},
	}
	bank := []BankStatement{
		{UniqueIdentifier: "B1", AmountCents: -10000, Date: mustDate("2025-08-01"), Bank: "Alpha"},
		{UniqueIdentifier: "B2", AmountCents: 4998, Date: mustDate("2025-08-01"), Bank: "Alpha"},
	}

	res := Reconcile(sys, bank, mustDate("2025-08-01"), mustDate("2025-08-01"))
	if res.MatchedCount != 2 {
		t.Fatalf("want 2 matches, got %d", res.MatchedCount)
	}
	if res.TotalDiscrepancyCents != 2 { // 0.02
		t.Fatalf("want discrepancy 2 cents, got %d", res.TotalDiscrepancyCents)
	}
	if len(res.UnmatchedSystem) != 0 {
		t.Fatalf("unexpected unmatched system")
	}
	for _, v := range res.UnmatchedBankByName {
		if len(v) != 0 {
			t.Fatalf("unexpected unmatched bank entries")
		}
	}
}

func TestUnmatchedAndGrouping(t *testing.T) {
	sys := []Transaction{
		{TrxID: "TX1", AmountCents: 10000, Type: DEBIT, TransactionTime: mustTime("2025-08-02T09:00:00Z")},
	}
	bank := []BankStatement{
		{UniqueIdentifier: "B3", AmountCents: -10000, Date: mustDate("2025-08-02"), Bank: "Alpha"},
		{UniqueIdentifier: "B4", AmountCents: 7500, Date: mustDate("2025-08-02"), Bank: "Beta"},
		{UniqueIdentifier: "B5", AmountCents: -2500, Date: mustDate("2025-08-02"), Bank: "Alpha"},
	}

	res := Reconcile(sys, bank, mustDate("2025-08-02"), mustDate("2025-08-02"))
	if res.MatchedCount != 1 {
		t.Fatalf("want 1 match, got %d", res.MatchedCount)
	}
	if len(res.UnmatchedBankByName) != 2 {
		t.Fatalf("want 2 banks, got %d", len(res.UnmatchedBankByName))
	}
	if len(res.UnmatchedBankByName["Beta"]) != 1 {
		t.Fatalf("want 1 unmatched in Beta")
	}
	if len(res.UnmatchedBankByName["Alpha"]) != 1 {
		t.Fatalf("want 1 unmatched in Alpha")
	}
}

func TestTimeframeAndNearest(t *testing.T) {
	sys := []Transaction{
		{TrxID: "TX1", AmountCents: 1000, Type: CREDIT, TransactionTime: mustTime("2025-07-31T23:59:59Z")}, // out of range
		{TrxID: "TX2", AmountCents: 1000, Type: CREDIT, TransactionTime: mustTime("2025-08-03T00:00:00Z")},
		{TrxID: "TX3", AmountCents: 1000, Type: CREDIT, TransactionTime: mustTime("2025-08-03T12:00:00Z")},
	}
	bank := []BankStatement{
		{UniqueIdentifier: "BA", AmountCents: 1000, Date: mustDate("2025-08-03"), Bank: "Alpha"},
		{UniqueIdentifier: "BB", AmountCents: 1000, Date: mustDate("2025-08-03"), Bank: "Alpha"},
		{UniqueIdentifier: "BC", AmountCents: 1000, Date: mustDate("2025-08-04"), Bank: "Alpha"}, // out of range
	}

	res := Reconcile(sys, bank, mustDate("2025-08-01"), mustDate("2025-08-03"))
	if res.MatchedCount != 2 {
		t.Fatalf("want 2 matches, got %d", res.MatchedCount)
	}
	if len(res.UnmatchedSystem) != 0 {
		t.Fatalf("unexpected unmatched system")
	}
	for _, v := range res.UnmatchedBankByName {
		if len(v) != 0 {
			t.Fatalf("unexpected unmatched bank entries")
		}
	}
}

func TestSignLogic(t *testing.T) {
	sys := []Transaction{
		{TrxID: "D", AmountCents: 1234, Type: DEBIT, TransactionTime: mustTime("2025-08-10T10:00:00Z")},
		{TrxID: "C", AmountCents: 1234, Type: CREDIT, TransactionTime: mustTime("2025-08-10T10:00:00Z")},
	}
	bank := []BankStatement{
		{UniqueIdentifier: "X", AmountCents: -1234, Date: mustDate("2025-08-10"), Bank: "Alpha"},
		{UniqueIdentifier: "Y", AmountCents: 1234, Date: mustDate("2025-08-10"), Bank: "Alpha"},
	}

	res := Reconcile(sys, bank, mustDate("2025-08-10"), mustDate("2025-08-10"))
	if res.MatchedCount != 2 {
		t.Fatalf("want 2 matches, got %d", res.MatchedCount)
	}
	if res.TotalDiscrepancyCents != 0 {
		t.Fatalf("want 0 discrepancy, got %d", res.TotalDiscrepancyCents)
	}
}
