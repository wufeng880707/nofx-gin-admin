package schema

import "testing"

func TestExchangeRejectsPlaintextCredentials(t *testing.T) {
	exchange := &Exchange{APIKeyCiphertext: "plain-api-key"}
	if err := exchange.BeforeSave(nil); err == nil {
		t.Fatal("expected plaintext credential to be rejected")
	}
}

func TestExchangeAcceptsEncryptedOrEmptyCredentials(t *testing.T) {
	exchange := &Exchange{
		APIKeyCiphertext:    "ENC:v1:encrypted-api-key",
		SecretKeyCiphertext: "",
	}
	if err := exchange.BeforeSave(nil); err != nil {
		t.Fatalf("expected encrypted credentials to be accepted: %v", err)
	}
}

func TestAIModelRejectsPlaintextCredential(t *testing.T) {
	model := &AIModel{APIKeyCiphertext: "plain-model-key"}
	if err := model.BeforeSave(nil); err == nil {
		t.Fatal("expected plaintext model credential to be rejected")
	}
}

func TestTradingTableNames(t *testing.T) {
	tests := map[string]string{
		(&Exchange{}).TableName(): "trading_exchanges",
		(&AIModel{}).TableName():  "trading_ai_models",
		(&Strategy{}).TableName(): "trading_strategies",
		(&Trader{}).TableName():   "trading_traders",
	}
	for got, want := range tests {
		if got != want {
			t.Fatalf("unexpected table name: got %q, want %q", got, want)
		}
	}
}

func TestBinanceUnifiedAccountRejectsTestnet(t *testing.T) {
	form := &ExchangeForm{
		ExchangeType: " BINANCE ",
		AccountType:  " UNIFIED ",
		Testnet:      true,
	}
	if err := form.Validate(); err == nil {
		t.Fatal("expected Binance Portfolio Margin testnet to be rejected")
	}
	if !form.UnifiedAccount {
		t.Fatal("expected unified_account to follow account_type")
	}

	form.Testnet = false
	if err := form.Validate(); err != nil {
		t.Fatalf("expected Binance Portfolio Margin production config to be valid: %v", err)
	}
}
