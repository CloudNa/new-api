package promptcompress

import "embed"

// omniRouteFS contains the RTK filter catalog and Caveman language packs
// vendored from diegosouzapw/OmniRoute at OmniRouteParityCommit.
//
//go:embed omniroute/caveman_rules/*/*.json omniroute/rtk_filters/*.json
var omniRouteFS embed.FS
