package biz

import "testing"

func TestProviderCapabilityMatrixAdvertisesOnlyImplementedTrading(t *testing.T) {
	matrix := new(Catalog).GetProviderCapabilities()
	if len(matrix.Providers) < 7 || len(matrix.DataSources) < 3 {
		t.Fatalf("capability matrix is incomplete: %#v", matrix)
	}
	for _, item := range matrix.Providers {
		if (item.Exchange == "binance" || item.Exchange == "okx") && !item.Trading {
			t.Fatalf("implemented testnet-first trading must be advertised: %#v", item)
		}
		if item.Exchange != "binance" && item.Exchange != "okx" && item.Trading {
			t.Fatalf("unimplemented trading must not be advertised: %#v", item)
		}
		if (item.Exchange == "bitget" || item.Exchange == "gate" || item.Exchange == "kucoin" || item.Exchange == "hyperliquid") &&
			item.MigrationStatus == "implemented" {
			t.Fatalf("evaluated provider must not be reported as implemented: %#v", item)
		}
	}
}
