package main

import (
	"github.com/btcsuite/btcd/chaincfg"
)

var BTCMainNetParams = chaincfg.Params{
	Name: "mainnet",

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "bc", // always bc for main net

	// Address encoding magics
	PubKeyHashAddrID: 0x00, // starts with 1
	ScriptHashAddrID: 0x05, // starts with 3
	PrivateKeyID:     0x80, // starts with 5 (uncompressed) or K (compressed)

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // starts with xprv
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // starts with xpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 0,
}

/*
const DSC_MAIN_NETWORK = {
    messagePrefix : '\x19DashCash Signed Message:\n',
    bip32 : {
        public : 0x0488b21e,
        private : 0x0488ade4
    },
    pubKeyHash : 0x1e,
    scriptHash : 0x10,
    wif : 0xcc,
    dustThreshold: 5460
};
*/
var DSCMainNetParams = chaincfg.Params{
	Name: "mainnet",

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "bc", // always bc for main net

	// Address encoding magics
	PubKeyHashAddrID: 0x1e, // starts with D
	ScriptHashAddrID: 0x10, // starts with 7
	PrivateKeyID:     0xCC, // starts with 2 (uncompressed) or K (compressed)

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // starts with xprv
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // starts with xpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 0,
}
