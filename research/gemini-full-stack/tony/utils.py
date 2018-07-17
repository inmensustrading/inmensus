import numpy as np
import pandas as pd
import statsmodels.api as sm


def get_aggregate_volume(data, is_bid = None, agg_sz = None):
    # type: (nparr(times, depth, 2), bool, float) -> nparr(times)
    """data should be one side of an order book. agg_sz=0 means no aggregation,
    only top level reported"""
    depth = data.shape[1]
    prices = data[:, :, 0]
    vols = data[:, :, 1]
    best_prices = prices[:, 0 - is_bid]
    agg_sz += 1e-6 # handle floating point errors
    if is_bid:
        in_agg_rng = prices >= (best_prices.repeat(depth).reshape(-1, depth) - agg_sz)
    else: # asks
        in_agg_rng = prices <= (best_prices.repeat(depth).reshape(-1, depth) + agg_sz)
    agg_vols = (vols * in_agg_rng).sum(axis=1)
    return agg_vols

def get_bof(df):
    # type: (pd.DataFrame) -> Pandas.Series
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
    # DataFrame -> Pandas.Series
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
    # type: (pd.DataFrame) -> Pandas.Series
    """oi := order imbalance,
    per http://eprints.maths.ox.ac.uk/1895/1/Darryl%20Shen%20(for%20archive).pdf"""
    oi = get_bof(df) - get_aof(df)
    return oi

def construct_df(bids=None, asks=None, times=None, agg_sz=1, roll_mean_window_size=20):
    # same types as above for bids, asks. times should be np.array of type np.datetime64.
    """agg_sz is in USD
    roll_mean_window_size is integer size for window"""

    agg_bid_vol = get_aggregate_volume(bids, is_bid=True, agg_sz=agg_sz)
    agg_ask_vol = get_aggregate_volume(asks, is_bid=False, agg_sz=agg_sz)
    agg_oir = (agg_bid_vol - agg_ask_vol) / (agg_bid_vol + agg_ask_vol)

    df = pd.DataFrame(data={'bid': bids[:, -1, 0],
                            'ask': asks[:, 0, 0],
                            'bid-volume': agg_bid_vol,
                            'ask-volume': agg_ask_vol,
                            'mid': (bids[:, -1, 0] + asks[:, 0, 0]) / 2,
                            'oir': agg_oir,
                            'time': times})

    df['dmid'] = df['mid'] - df['mid'].shift(1)
    df['oi'] = get_oi(df)

    df['roll-mid'] = df['mid'].rolling(roll_mean_window_size).mean()
    
    return df

def get_average_future_mid_change(df, future_window_size=None):
    # type: (pd.DataFrame, int) -> pd.Series
    ret = df['roll-mid'].shift(-future_window_size) - df['mid']
    ret = ret.round(decimals=8) # make things close to zero (floating point error) actually zero
    return ret

def get_feature_matrix(df, feature_history=None, feature_names=None, add_constant=None):
    # type: (pd.DataFrame, int, List[str], bool) -> 2D np.array
    """No constant is appended. Caller is in charge of that."""
    if add_constant is None:
        raise Exception("must specify add_constant")
    if feature_names is None:
        raise Exception("must specify feature_names")
    X = np.zeros((df.shape[0], feature_history * len(feature_names)))
    for i in range(feature_history):
        for j, feature_name in enumerate(feature_names):
            X[:, i * len(feature_names) + j] = df[feature_name].shift(i).values
    if add_constant:
        return sm.add_constant(X)
    return X

def create_portfolio(bull_times=None, bear_times=None, initial_position=None):
    # type: (pd.Series, pd.Series, float) -> pd.Series
    """If:
    bull_times - bear_times == [     1, 1, 1, -1, 0, 0, 1, 0, 0, -1, 0]
    then returns:    [initial_position, 1, 1,  1, 0, 0, 0, 1, 1,  1, 0]"""
    portfolio = bull_times - bear_times
    portfolio = portfolio.shift(1)
    portfolio[portfolio == 0] = np.nan
    portfolio[0] = initial_position
    portfolio = portfolio.ffill()
    portfolio *= portfolio > 0 # don't support shorting
    return portfolio

def compute_taker_fees(df, portfolio, taker_fee=None):
    # type: (pd.DataFrame, pd.Series, float) -> pd.Series
    """taker_fee is fraction"""
    buy_times = (portfolio < portfolio.shift(-1))
    sell_times = (portfolio > portfolio.shift(-1))
    bid_prices = pd.Series(df['bid'].values) # re-zero index
    ask_prices = pd.Series(df['ask'].values) # re-zero index
    taker_fees = taker_fee * (buy_times * ask_prices + sell_times * bid_prices)
    taker_fees = taker_fees.shift(1) # the fee is accrued between timesteps and is counted towards the next timestep
    return taker_fees

def get_sharpe_ratio(portfolio_returns, returns):
    # type: (pd.Series, pd.Series) -> float
    """See formula in https://en.wikipedia.org/wiki/Sharpe_ratio"""
    gain = portfolio_returns - returns
    sr = gain.mean() / gain.std()
    return sr