package models

// PoolConfig p
type PoolConfig struct {
	BaseModel
	ID               string `json:"id"`
	BaseMint         string `json:"baseMint"`
	QuoteMint        string `json:"quoteMint"`
	BaseDecimals     int    `json:"baseDecimals"`
	QuoteDecimals    int    `json:"quoteDecimals"`
	OpenOrders       string `json:"openOrders"`
	TargetOrders     string `json:"targetOrders"`
	BaseVault        string `json:"baseVault"`
	QuoteVault       string `json:"quoteVault"`
	MarketID         string `json:"marketId"`
	MarketBaseVault  string `json:"marketBaseVault"`
	MarketQuoteVault string `json:"marketQuoteVault"`
	MarketBids       string `json:"marketBids"`
	MarketAsks       string `json:"marketAsks"`
	MarketEventQueue string `json:"marketEventQueue"`
}

// Create poolConfig
func (poolConfig *PoolConfig) Create() error {

	if dbc := GetDB().Create(poolConfig); dbc.Error != nil {
		return dbc.Error
	}

	return nil
}

// GetPoolConfig new
func GetPoolConfig(fromToken string, toToken string) PoolConfig {

	var poolConfig PoolConfig
	GetDB().Table("pool_configs").Where("base_mint = ? AND quote_mint = ?", fromToken, toToken).First(&poolConfig)

	return poolConfig
}
