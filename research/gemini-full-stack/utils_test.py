import numpy as np
import pandas as pd

from utils import *

def test_get_aggregate_volume():
    bids = np.array([[[0.98, 1], [0.99, 2], [1.00, 1]],
                     [[0.97, 1], [0.98, 1], [0.99, 5]]])
    assert (get_aggregate_volume(bids, agg_sz = 0, is_bid = True) == np.array([1, 5])).all()
    assert (get_aggregate_volume(bids, agg_sz = 0.01, is_bid = True) == np.array([3, 6])).all()
    
    asks = np.array([[[0.98, 1], [0.99, 2], [1.01, 1]],
                     [[0.97, 3], [0.98, 1], [0.99, 5]]])
    assert (get_aggregate_volume(asks, agg_sz = 0, is_bid = False) == np.array([1, 3])).all()
    assert (get_aggregate_volume(asks, agg_sz = 0.02, is_bid = False) == np.array([3, 9])).all()
    
def test_get_bof():
    df = pd.DataFrame({'bid': [1, 1, 0.99, 4, 4],
                       'bid-volume': [5, 4, 6, 3, 7]})
    bof = get_bof(df)
    assert np.allclose(bof.values, np.array([np.nan, -1.0, 0.0, 3.0, 4.0]), equal_nan=True)
    
def test_get_aof():
    df = pd.DataFrame({'ask': [1, 1, 0.99, 4, 4],
                       'ask-volume': [5, 4, 6, 3, 7]})
    aof = get_aof(df)
    assert np.allclose(aof.values, np.array([np.nan, -1.0, 6.0, 0, 4.0]), equal_nan=True)
    
def test_get_oi():
    df = pd.DataFrame({'bid': [1, 1, 0.99, 4, 4],
                       'bid-volume': [5, 4, 6, 3, 7],
                       'ask': [1, 1, 0.99, 4, 4],
                       'ask-volume': [5, 4, 6, 3, 7]})
    oi = get_oi(df)
    assert np.allclose(oi.values, np.array([np.nan, 0, -6.0, 3.0, 0]), equal_nan=True)
    
def test_construct_df():
    bids = np.array([[[0.98, 1]],
                     [[0.98, 1]],
                     [[0.97, 1]],
                     [[1.00, 1]],
                     [[1.00, 2]]])
    asks = np.array([[[0.99, 2]],
                     [[0.99, 3]],
                     [[0.98, 1]],
                     [[1.01, 7]],
                     [[1.01, 2]]])
    times = np.arange(5)
    df = construct_df(bids=bids, asks=asks, times=times, agg_sz=0, roll_mean_window_size=3)
    assert (df['mid'].values == np.array([0.985, 0.985, 0.975, 1.005, 1.005])).all()
    assert np.allclose(df['dmid'].values, np.array([np.nan, 0, -0.01, 0.03, 0]), equal_nan=True)
    assert (df['oir'] == np.array([-1.0 / 3, -0.5, 0, -0.75, 0])).all()
    assert np.allclose(df['roll-mid'], np.array([np.nan, np.nan,
                                                 (0.985 + 0.985 + 0.975) / 3,
                                                 (0.985 + 0.975 + 1.005) / 3,
                                                 (0.975 + 1.005 + 1.005) / 3]), equal_nan=True)
    
def test_get_average_future_mid_change():
    df = pd.DataFrame({'mid': [0, 0, 3, 6, 9, 0],
                       'roll-mid': [np.nan, np.nan, 1, 3, 6, 5]})
    y = get_average_future_mid_change(df, future_window_size=3)
    assert (y.values[:3] == np.array([3, 6, 2])).all()
    assert np.allclose(y.values[4:6], np.array([np.nan, np.nan]), equal_nan=True)
    
def test_get_feature_matrix():
    df = pd.DataFrame({'f1': [1, 2, 3, 4, 5, 6],
                       'f2': [2, 4, 6, 8, 10, 12]})
    fm1 = get_feature_matrix(df, feature_history=2, feature_names=['f1'], add_constant=False)
    assert np.allclose(fm1,
                       np.array(list(zip([1, 2, 3, 4, 5, 6],
                                         [np.nan, 1, 2, 3, 4, 5]))),
                       equal_nan=True)
    fm2 = get_feature_matrix(df, feature_history=2, feature_names=['f1', 'f2'], add_constant=True)
    print(fm2)
    assert np.allclose(fm2,
                       np.array([[1, 1, 2, np.nan, np.nan],
                                 [1, 2, 4, 1, 2],
                                 [1, 3, 6, 2, 4],
                                 [1, 4, 8, 3, 6],
                                 [1, 5, 10, 4, 8],
                                 [1, 6, 12, 5, 10]]),
                       equal_nan=True)

def test_create_portfolio():
    bull_times = pd.Series([1, 1, 1, 0, 0, 0, 1, 0, 0, 0, 0])
    bear_times = pd.Series([0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0])
    p0 = create_portfolio(bull_times=bull_times, bear_times=bear_times, initial_position=0)
    p1 = create_portfolio(bull_times=bull_times, bear_times=bear_times, initial_position=1)
    assert (p0.values == np.array([0, 1, 1, 1, 0, 0, 0, 1, 1, 1, 0])).all()
    assert (p1.values == np.array([1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 0])).all()
    
def test_compute_taker_fees():
    df = pd.DataFrame({'bid': [1, 1, 10, 10],
                       'ask': [2, 2, 20, 20]})
    portfolio = pd.Series([0, 1, 0, 1])
    tf = compute_taker_fees(df, portfolio, taker_fee=2)
    assert np.allclose(np.array([tf.values[0]]), np.array([np.nan]), equal_nan=True)
    assert (tf.values[1:] == np.array([4, 2, 40])).all()
    