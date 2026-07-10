package params

import (
	"math/big"
	"testing"
)

// Chiado runs Spurious Dragon (EIP-155/158/160/161/170) from genesis, like
// Gnosis mainnet. The canonical chain enforces the EIP-170 max code size:
// Chiado block 4,129,734 contains a contract creation that fails there with
// an oversized deployment (24,587 bytes), so a nil EIP158Block makes geth
// accept the creation and diverge on gasUsed.
func TestGnosisChainsSpuriousDragonAtGenesis(t *testing.T) {
	for name, config := range map[string]*ChainConfig{
		"gnosis": GnosisChainConfig,
		"chiado": ChiadoChainConfig,
	} {
		if !config.IsEIP158(big.NewInt(0)) {
			t.Errorf("%s: EIP-158 not active at genesis", name)
		}
		if !config.IsEIP155(big.NewInt(0)) {
			t.Errorf("%s: EIP-155 not active at genesis", name)
		}
	}
}
