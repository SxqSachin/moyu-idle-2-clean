-- Partial index for ListOnlinePresenceUsers (WHERE status = 'online').
-- Before: full table scan every 60 s for presence reconcile.
CREATE INDEX IF NOT EXISTS idx_player_presence_online
  ON player_presence(user_id) WHERE status = 'online';

-- Covering index for the active-only path of ListMarketListings.
-- Matches ORDER BY price ASC, id ASC with status = 'active' filter.
CREATE INDEX IF NOT EXISTS idx_market_listings_item_active
  ON market_listings(item_id, price ASC, id ASC) WHERE status = 'active';

-- Index for ListMyMarketListings and cleanupBannedUserListingsTx (filter by seller_id).
CREATE INDEX IF NOT EXISTS idx_market_listings_seller
  ON market_listings(seller_id, id DESC);
