package content

// Balance holds the tunable coefficients of the formulas in TECH_DESIGN §7. All
// money is in cents. Changing a value changes game results, so the content version
// changes with it.
type Balance struct {
	Demand      DemandBalance      `json:"demand"`
	Pricing     PricingBalance     `json:"pricing"`
	Service     ServiceBalance     `json:"service"`
	Reputation  ReputationBalance  `json:"reputation"`
	Flights     FlightBalance      `json:"flights"`
	Aging       AgingBalance       `json:"aging"`
	Fuel        FuelBalance        `json:"fuel"`
	Countries   CountryBalance     `json:"countries"`
	Leases      LeaseBalance       `json:"leases"`
	Finance     FinanceBalance     `json:"finance"`
	Competition CompetitionBalance `json:"competition"`
}

// DemandBalance: D_OD = d0 · popFactor_A · popFactor_B / distFactor · wealthAvg · mods.
type DemandBalance struct {
	D0                 float64 `json:"d0"`
	BusinessShareMin   float64 `json:"businessShareMin"` // share of D going business, by business profile
	BusinessShareMax   float64 `json:"businessShareMax"`
	ConnectFactor      float64 `json:"connectFactor"`      // connecting demand through a hub
	ConnectingDiscount float64 `json:"connectingDiscount"` // fare multiplier for connections
	MediumAirportShare float64 `json:"mediumAirportShare"` // airportSizeFactor for medium airports
}

// PricingBalance: ref_eco = (base + perKm·km) · wealthAvg; ref_biz = bizMultiplier · ref_eco.
type PricingBalance struct {
	RefBaseCents   int64   `json:"refBaseCents"`
	RefPerKmCents  float64 `json:"refPerKmCents"`
	BizMultiplier  float64 `json:"bizMultiplier"`
	ElasticityEco  float64 `json:"elasticityEco"`
	ElasticityBiz  float64 `json:"elasticityBiz"`
	PriceFactorMin float64 `json:"priceFactorMin"`
	PriceFactorMax float64 `json:"priceFactorMax"`
}

// ServiceBalance: 0.7 + 0.3·condition + 0.05·planeLevel + 0.03·airportLevel.
type ServiceBalance struct {
	Base               float64 `json:"base"`
	ConditionWeight    float64 `json:"conditionWeight"`
	PlaneLevelWeight   float64 `json:"planeLevelWeight"`
	AirportLevelWeight float64 `json:"airportLevelWeight"`
	BusinessMultiplier float64 `json:"businessMultiplier"` // business weighs the non-constant part more
}

// ReputationBalance: factor = 0.5 + reputation/100; reputation 0..100.
type ReputationBalance struct {
	Start         float64 `json:"start"`
	DelayPenalty  float64 `json:"delayPenalty"`
	CancelPenalty float64 `json:"cancelPenalty"`
	OnTimeTarget  float64 `json:"onTimeTarget"` // on-time rate above which reputation grows
	GainPerPoint  float64 `json:"gainPerPoint"` // per on-time point above target, per day
	DecayPerDay   float64 `json:"decayPerDay"`  // fraction of the distance to Start recovered daily
}

// FlightBalance: turnaround, delay/cancel split, delayed revenue.
type FlightBalance struct {
	TurnaroundTicks      int     `json:"turnaroundTicks"`
	DelayShareOfFailures float64 `json:"delayShareOfFailures"`
	DelayRevenueFactor   float64 `json:"delayRevenueFactor"`
	ConditionReliability float64 `json:"conditionReliability"` // weight of condition in success chance
	CancelRebookShare    float64 `json:"cancelRebookShare"`
}

// AgingBalance: condition decay, overhaul, depreciation.
type AgingBalance struct {
	ConditionDecayPerHour float64 `json:"conditionDecayPerHour"`
	OverhaulCostShare     float64 `json:"overhaulCostShare"`
	OverhaulDays          int     `json:"overhaulDays"`
	DailyDepreciation     float64 `json:"dailyDepreciation"` // multiplied daily: 0.88^(1/365)
	ReconfigureTicks      int     `json:"reconfigureTicks"`
}

// FuelBalance: p' = p + θ(μ − p) + σ·μ·Z + shock, clamped to [minFactor·μ, maxFactor·μ].
type FuelBalance struct {
	MeanCentsPerUnit int64   `json:"meanCentsPerUnit"`
	Theta            float64 `json:"theta"`
	Sigma            float64 `json:"sigma"`
	ShockChance      float64 `json:"shockChance"`
	ShockMin         float64 `json:"shockMin"`
	ShockMax         float64 `json:"shockMax"`
	ShockDays        int     `json:"shockDays"`
	MinFactor        float64 `json:"minFactor"`
	MaxFactor        float64 `json:"maxFactor"`
}

// CountryBalance: price = k · priceBase; daily upkeep share; start cash.
type CountryBalance struct {
	PriceK            float64 `json:"priceK"`
	UpkeepShare       float64 `json:"upkeepShare"`
	StartCashCents    int64   `json:"startCashCents"`
	StartCashMinCents int64   `json:"startCashMinCents"`
	StartCashMaxCents int64   `json:"startCashMaxCents"`
	StartCashExponent float64 `json:"startCashExponent"`
	OwnerFeeShare     float64 `json:"ownerFeeShare"`
	ActiveServiceDays int     `json:"activeServiceDays"`
}

// LeaseBalance: landing rights.
type LeaseBalance struct {
	UpfrontShareMin     float64 `json:"upfrontShareMin"`
	UpfrontShareMax     float64 `json:"upfrontShareMax"`
	YearlyLeaseShare    float64 `json:"yearlyLeaseShare"`
	LeasedFeeMultiplier float64 `json:"leasedFeeMultiplier"`
}

// FinanceBalance: loans, overdraft, credit tiers, liquidation.
type FinanceBalance struct {
	BaseRate        float64      `json:"baseRate"`
	CreditTiers     []CreditTier `json:"creditTiers"` // ordered by MinScore descending
	OverdraftRate   float64      `json:"overdraftRate"`
	LiquidationDays int          `json:"liquidationDays"`
	Restructurings  int          `json:"restructurings"`
	MaxLoanWeeks    int          `json:"maxLoanWeeks"`
}

// CreditTier: the spread above BaseRate for a credit score at or above MinScore.
// A tier without loans (NoLoans) is the lowest.
type CreditTier struct {
	MinScore int     `json:"minScore"`
	Spread   float64 `json:"spread"`
	NoLoans  bool    `json:"noLoans,omitempty"`
}

// CompetitionBalance: background carriers.
type CompetitionBalance struct {
	PriceAdjust   float64 `json:"priceAdjust"`
	UndercutShare float64 `json:"undercutShare"` // player share that triggers undercutting
	UndercutDays  int     `json:"undercutDays"`
	UndercutMin   float64 `json:"undercutMin"`
	UndercutMax   float64 `json:"undercutMax"`
}
