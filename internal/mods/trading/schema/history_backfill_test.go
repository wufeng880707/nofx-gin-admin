package schema

import (
	"testing"
	"time"
)

func TestHistoryBackfillFormNormalizesSymbols(t *testing.T) {
	endAt := time.Now().UTC().Add(-time.Minute)
	form := &HistoryBackfillForm{
		StartAt: endAt.Add(-24 * time.Hour),
		EndAt:   &endAt,
		Symbols: []string{" btcusdt, ETHUSDT ", "BTCUSDT"},
	}
	if err := form.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(form.Symbols) != 2 || form.Symbols[0] != "BTCUSDT" || form.Symbols[1] != "ETHUSDT" {
		t.Fatalf("unexpected symbols: %#v", form.Symbols)
	}
}

func TestHistoryBackfillFormRejectsMoreThanNinetyDays(t *testing.T) {
	endAt := time.Now().UTC().Add(-time.Minute)
	form := &HistoryBackfillForm{StartAt: endAt.Add(-91 * 24 * time.Hour), EndAt: &endAt}
	if err := form.Validate(); err == nil {
		t.Fatal("expected oversized history range to be rejected")
	}
}

func TestHistorySymbolsFormNormalizesAndDeduplicates(t *testing.T) {
	form := &HistorySymbolsForm{Symbols: []string{" btcusdt, ETHUSDT ", "BTCUSDT", "solusdt"}}
	if err := form.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	if len(form.Symbols) != len(want) {
		t.Fatalf("unexpected symbols: %#v", form.Symbols)
	}
	for index := range want {
		if form.Symbols[index] != want[index] {
			t.Fatalf("unexpected symbols: %#v", form.Symbols)
		}
	}
}
