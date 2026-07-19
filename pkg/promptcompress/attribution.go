package promptcompress

// This package implements a Go-native prompt compression layer pinned to
// OmniRoute's MIT-licensed RTK/Caveman compression pipeline at the commit below.
//
// Sources studied for compatible behavior:
// - OmniRoute: https://github.com/diegosouzapw/OmniRoute
// - RTK - Rust Token Killer: https://github.com/rtk-ai/rtk
// - Caveman: https://github.com/JuliusBrussee/caveman
//
// The implementation here is not a runtime dependency on those projects. The
// rule packs and RTK filter catalog under omniroute/ are vendored from the
// pinned OmniRoute commit so updates to new-api can replay this custom layer.

const (
	OmniRouteParityCommit = "630baa6"
	Attribution           = "OmniRoute RTK/Caveman compression parity layer (MIT), pinned to diegosouzapw/OmniRoute@630baa6."
	RuleVersion           = "omniroute-parity-630baa6-go-v1"
)
