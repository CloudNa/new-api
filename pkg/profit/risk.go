package profit

const (
	RiskReasonGrossMarginBelowMinimum    = "gross_margin_below_minimum"
	RiskReasonExpectedMarginBelowMinimum = "expected_margin_below_minimum"
	RiskReasonLossMakingRequest          = "loss_making_request"
)

type RiskDecision struct {
	Mode                 string   `json:"mode"`
	Alert                bool     `json:"alert"`
	Reasons              []string `json:"reasons,omitempty"`
	MinGrossMarginUSD    float64  `json:"min_gross_margin_usd,omitempty"`
	MinGrossMarginPct    float64  `json:"min_gross_margin_pct,omitempty"`
	MinExpectedMarginUSD float64  `json:"min_expected_margin_usd,omitempty"`
	MinExpectedMarginPct float64  `json:"min_expected_margin_pct,omitempty"`
	ObserveOnly          bool     `json:"observe_only"`
	LiveEnforced         bool     `json:"live_enforced"`
}

func BuildRiskDecision(settings Settings, estimate CostEstimate) *RiskDecision {
	settings = settings.Normalize()
	if (settings.RiskEnforcement != RiskModeAlert && settings.RiskEnforcement != ModeEnforce) || !estimate.CostKnown {
		return nil
	}

	observeOnly := settings.ObserveOnly || settings.RiskEnforcement == RiskModeAlert
	decision := &RiskDecision{
		Mode:                 settings.RiskEnforcement,
		MinGrossMarginUSD:    settings.RiskMinGrossUSD,
		MinGrossMarginPct:    settings.RiskMinGrossPct,
		MinExpectedMarginUSD: settings.RiskMinExpectUSD,
		MinExpectedMarginPct: settings.RiskMinExpectPct,
		ObserveOnly:          observeOnly,
		LiveEnforced:         false,
	}

	if belowMinimum(estimate.GrossMarginUSD, settings.RiskMinGrossUSD) ||
		belowMinimum(estimate.GrossMarginPct, settings.RiskMinGrossPct) {
		decision.addReason(RiskReasonGrossMarginBelowMinimum)
	}
	if belowMinimum(estimate.ExpectedMarginUSD, settings.RiskMinExpectUSD) ||
		belowMinimum(estimate.ExpectedMarginPct, settings.RiskMinExpectPct) {
		decision.addReason(RiskReasonExpectedMarginBelowMinimum)
	}
	if belowZero(estimate.GrossMarginUSD) || belowZero(estimate.ExpectedMarginUSD) {
		decision.addReason(RiskReasonLossMakingRequest)
	}
	decision.Alert = len(decision.Reasons) > 0
	decision.LiveEnforced = decision.Alert && !decision.ObserveOnly && settings.RiskEnforcement == ModeEnforce
	return decision
}

func belowMinimum(value *float64, minimum float64) bool {
	return value != nil && *value < minimum
}

func belowZero(value *float64) bool {
	return value != nil && *value < 0
}

func (d *RiskDecision) addReason(reason string) {
	for _, existing := range d.Reasons {
		if existing == reason {
			return
		}
	}
	d.Reasons = append(d.Reasons, reason)
}
