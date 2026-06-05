package promptcompress

// This package implements a Go-native prompt compression layer inspired by
// OmniRoute's MIT-licensed RTK/Caveman compression pipeline.
//
// Sources studied for compatible behavior:
// - OmniRoute: https://github.com/diegosouzapw/OmniRoute
// - RTK - Rust Token Killer: https://github.com/rtk-ai/rtk
// - Caveman: https://github.com/JuliusBrussee/caveman
//
// The implementation here is not a runtime dependency on those projects. It
// preserves the pipeline semantics needed by new-api: protect structured data,
// compress noisy command output with RTK-style filters, then condense prose with
// Caveman-style rules.

const Attribution = "Inspired by OmniRoute RTK/Caveman compression (MIT), RTK, and Caveman."
