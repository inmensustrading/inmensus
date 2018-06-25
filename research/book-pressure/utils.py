def get_aggregate_volume(data, is_bid = None, agg_sz = None):
    """data should be one side of an order book"""
    depth = data.shape[1]
    prices = data[:, :, 0]
    vols = data[:, :, 1]
    best_prices = prices[:, 0 - is_bid]
    if is_bid:
        in_agg_rng = prices > (best_prices.repeat(depth).reshape(-1, depth) - agg_sz)
    else: # asks
        in_agg_rng = prices < (best_prices.repeat(depth).reshape(-1, depth) + agg_sz)
    agg_vols = (vols * in_agg_rng).sum(axis=1)
    return agg_vols

def get_bof(df):
    """bof := bid order flow"""
    bids = df['bid']
    sr_bids = bids.shift(1)
    bid_eq = (bids == sr_bids)
    bid_g = (bids > sr_bids)

    bid_vol = df['bid-volume']
    sr_bid_vol = bid_vol.shift(1)

    bof = bid_eq * (bid_vol - sr_bid_vol) + bid_g * bid_vol
    return bof

def get_aof(df):
    """aof := ask order flow"""
    asks = df['ask']
    sr_asks = asks.shift(1)
    ask_eq = (asks == sr_asks)
    ask_l = (asks < sr_asks)

    ask_vol = df['ask-volume']
    sr_ask_vol = ask_vol.shift(1)

    aof = ask_eq * (ask_vol - sr_ask_vol) + ask_l * ask_vol
    return aof

def get_oi(df):
    """oi := order imbalance,
    per http://eprints.maths.ox.ac.uk/1895/1/Darryl%20Shen%20(for%20archive).pdf"""
    oi = get_bof(df) - get_aof(df)
    return oi
