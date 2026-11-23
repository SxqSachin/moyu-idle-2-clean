package catalog

import (
	"fmt"
	"mysecondwebgame/server/internal/marketpolicy"
)

func (d *Data) MarketProfile(id string) *marketpolicy.Profile {
	for i := range d.MarketProfiles {
		if d.MarketProfiles[i].ID == id && !d.MarketProfiles[i].Disabled {
			return &d.MarketProfiles[i]
		}
	}
	return nil
}
func (d *Data) ValidateMarketPolicies() error {
	seen := map[string]bool{}
	for _, p := range d.MarketProfiles {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("market profile %s: %w", p.ID, err)
		}
		if seen[p.ID] {
			return fmt.Errorf("duplicate market profile %s", p.ID)
		}
		seen[p.ID] = true
	}
	for _, item := range d.Items {
		if item.MarketPolicy == nil {
			continue
		}
		if err := item.MarketPolicy.Validate(); err != nil {
			return fmt.Errorf("item %s: %w", item.ID, err)
		}
		if item.Stackable || item.HasType("currency") || !seen[item.MarketPolicy.FixedPrice.ProfileID] {
			return fmt.Errorf("invalid market policy reference or instance type: %s", item.ID)
		}
	}
	return nil
}

func (i *Item) UsesInstanceMarket() bool {
	return i != nil && i.MarketPolicy != nil && i.MarketPolicy.Validate() == nil
}
func (i *Item) AllowsAuction() bool {
	if i == nil {
		return false
	}
	if i.MarketPolicy != nil {
		return i.MarketPolicy.Validate() == nil && i.MarketPolicy.Auction
	}
	return i.MarketListable
}
