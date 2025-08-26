package reconcile

import (
	"sort"
	timepkg "time"
)

// ReconciliationResult summarizes the output.
type ReconciliationResult struct {
	TotalSystemTransactions int
	TotalBankTransactions   int
	TotalProcessed          int

	MatchedCount int

	UnmatchedSystem     []Transaction
	UnmatchedBankByName map[string][]BankStatement

	// Sum of absolute differences across matched pairs (in cents).
	TotalDiscrepancyCents Money
}

// Reconcile matches system vs bank rows within [start,end] (inclusive),
// using transaction DATE and SIGN (+/-) as the match key.
// Among candidates in the same bucket (date+sign), the nearest amount is chosen.
// Any leftovers are reported as unmatched (system) or unmatched-by-bank (bank).
func Reconcile(system []Transaction, bank []BankStatement, start, end timepkg.Time) ReconciliationResult {
	// Filter by inclusive date window.
	sysIn := make([]Transaction, 0, len(system))
	for _, t := range system {
		td := t.DateOnly()
		if !td.Before(start) && !td.After(end) {
			sysIn = append(sysIn, t)
		}
	}

	bankIn := make([]BankStatement, 0, len(bank))
	for _, b := range bank {
		if !b.Date.Before(start) && !b.Date.After(end) {
			bankIn = append(bankIn, b)
		}
	}

	// Buckets keyed by (date, sign).
	type key struct {
		dateUnix int64
		sign     int // +1 for >=0, -1 for <0
	}

	buckets := make(map[key][]BankStatement)
	for _, b := range bankIn {
		sign := 1
		if b.AmountCents < 0 {
			sign = -1
		}
		k := key{dateUnix: b.Date.Unix(), sign: sign}
		buckets[k] = append(buckets[k], b)
	}

	// Deterministic system iteration (by transaction time).
	sort.SliceStable(sysIn, func(i, j int) bool {
		return sysIn[i].TransactionTime.Before(sysIn[j].TransactionTime)
	})

	matched := 0
	var discrepancy Money
	var unmatchedSys []Transaction

	for _, tx := range sysIn {
		sign := 1
		if tx.SignedAmount() < 0 {
			sign = -1
		}
		k := key{dateUnix: tx.DateOnly().Unix(), sign: sign}

		candidates := buckets[k]
		if len(candidates) == 0 {
			unmatchedSys = append(unmatchedSys, tx)
			continue
		}

		// Pick nearest amount (absolute delta).
		target := tx.SignedAmount()
		bestIdx := -1
		var bestDiff Money
		for i, c := range candidates {
			diff := target - c.AmountCents
			if diff < 0 {
				diff = -diff
			}
			if bestIdx == -1 || diff < bestDiff {
				bestIdx = i
				bestDiff = diff
			}
		}

		// Remove chosen candidate and account for discrepancy.
		candidates = append(candidates[:bestIdx], candidates[bestIdx+1:]...)
		buckets[k] = candidates
		discrepancy += bestDiff
		matched++
	}

	// Remaining bank statements in buckets are unmatched, group by bank.
	unmatchedBank := make(map[string][]BankStatement)
	for _, rem := range buckets {
		for _, b := range rem {
			unmatchedBank[b.Bank] = append(unmatchedBank[b.Bank], b)
		}
	}

	return ReconciliationResult{
		TotalSystemTransactions: len(sysIn),
		TotalBankTransactions:   len(bankIn),
		TotalProcessed:          len(sysIn) + len(bankIn),
		MatchedCount:            matched,
		UnmatchedSystem:         unmatchedSys,
		UnmatchedBankByName:     unmatchedBank,
		TotalDiscrepancyCents:   discrepancy,
	}
}
