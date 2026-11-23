package rng_test

// goldenUint64 holds the first three draws of For(42, "ledger", 1), generated on
// amd64. CI checks them on amd64, arm64 and wasm.
var goldenUint64 = []uint64{378661343234297846, 18403809693981013411, 13041757324340770324}
