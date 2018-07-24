import os
import psutil
import pandas as pd

def toRelPath(origPath):
	"""Converts path to path relative to current script

	origPath:	path to convert
	"""
	try:
		if not hasattr(toRelPath, "__location__"):
			toRelPath.__location__ = os.path.realpath(os.path.join(os.getcwd(), os.path.dirname(__file__)))
		return os.path.join(toRelPath.__location__, origPath)
	except NameError:
		return origPath
	
def getMBUsage():
	process = psutil.Process(os.getpid())
	return process.memory_info().rss / 1e6

def setCUDAVisible(devices):
	"""
	0: 1080Ti, 1: 940MX
	"""
	os.environ["CUDA_VISIBLE_DEVICES"] = devices

def portfolioToValue(portfolio, mid, fees):
	midShift = mid.shift(-1)
	rMid = midShift / mid
	rValue = portfolio * rMid + (1 - portfolio)
	trueValue = rValue.cumprod() * mid[0]
	
	dPortfolio = portfolio.shift(-1) - portfolio
	#percentage fees incurred at each timestep
	dRelValueFees = 1 - dPortfolio.abs() * fees
	#value with fees relative to value without fees
	relValueFees = dRelValueFees.cumprod()
	return relValueFees * trueValue

def computeMACD(series, fast, slow, signal):
	emaFast = series.ewm(span = fast).mean()
	emaSlow = series.ewm(span = slow).mean()
	maDiff = emaFast - emaSlow
	maSignal = maDiff.ewm(span = signal).mean()
	return maDiff - maSignal, maDiff, maSignal, emaFast, emaSlow

def computeOBV(series, volume):
	dMid = series - series.shift(1)
	dMid[dMid > 0] = 1
	dMid[dMid < 0] = -1
	vMod = volume * dMid
	return vMod.cumsum()