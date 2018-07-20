import pymysql
import numpy as np
import pandas as pd
import math
import functools

def sqlToDataframe(
	metrics = [],
	startTime = None,
	endTime = None,
	timestepSize = 500,
	sqlHost = "inmensus-trading-db-1.ce50oojfsygk.us-east-2.rds.amazonaws.com",
	sqlUser = "GILGAMESH",
	sqlPassword = None,
	sqlDB = "inmensus_trading_db_1",
	eventsTable = "gemini_events_btcusd",
	checkpointsTable = "gemini_checkpoints_btcusd",
	checkpointTimesTable = "gemini_checkpoint_times_btcusd",
	queryLimit = 100000,
	maxPrice = 1000000
):
	"""
	metrics: a combination of the following:
		"time"
		"max-bid"
		"min-ask"
		"mid"
		"bid-volume"
		"ask-volume"
		"last-trade"
		"oir"
	startTime: unix ms time; select -X to query the most recent X ms; DF will start at the first time divisible by timestepSize including or after startTime
	endTime: unix ms time; DF will end at the last time divisible by timestepSize before endTime
	queryLimit: max rows for a single query to the events table; to lessen load on memory and network
	"""
	
	conn = pymysql.connect(
		host = sqlHost,
		user = sqlUser,
		passwd = sqlPassword,
		db = sqlDB
	)
	cur = conn.cursor()
	
	#get row count in each table
	dbTables = [eventsTable, checkpointsTable, checkpointTimesTable]
	tableRows = {}
	for table in dbTables:
		cur.execute("SELECT COUNT(*) FROM " + table + ";")
		tableRows[table] = cur.fetchall()[0][0]
	
	if endTime == None:
		cur.execute("SELECT * FROM " + eventsTable + " LIMIT 1 OFFSET " + str(tableRows[eventsTable] - 1) + ";")
		endTime = cur.fetchall()[0][0]
	if startTime == None:
		cur.execute("SELECT * FROM " + checkpointTimesTable + " LIMIT 1;")
		startTime = cur.fetchall()[0][0]
	#we can query recent event segments too
	elif startTime < 0 and endTime != None:
		startTime += endTime
	print("Perparing to process from times", startTime, "to", str(endTime) + "... ")
	
	#setup metrics
	firstTimestep = math.ceil(startTime / timestepSize) * timestepSize
	timesteps = math.floor((endTime - firstTimestep) / timestepSize) + 1
	npMetrics = np.zeros((timesteps, len(metrics)))

	#metrics implemented as callbacks during event processing
	mcbDict = {
		"time":		computeMetricTime,
		"max-bid":	computeMetricMaxBid, 
		"min-ask":	computeMetricMinAsk, 
		"mid":	computeMetricMid, 
		"bid-volume":	computeMetricBidVolume, 
		"ask-volume":	computeMetricAskVolume, 
		"last-trade":	computeMetricLastTrade, 
		"oir":	computeMetricOIR
	}
	metricCallbacks = []
	for a in range(len(metrics)):
		metricCallbacks.append(
			functools.partial(mcbDict[metrics[a]], npMetrics, a))

	#setup initial checkpoint
	cur.execute("SELECT MAX(time) FROM " + checkpointTimesTable + 
				" WHERE time <= " + str(startTime) + 
				";")
	ckpTime = cur.fetchall()[0][0]
	cur.execute("SELECT * FROM " + checkpointsTable + 
				" WHERE time = " + str(ckpTime) + 
				";")
	checkpoint = cur.fetchall()
	print("Initializing from checkpoint at time", str(ckpTime) + "...")

	#process orderbook from checkpoint
	maxBid = 0
	minAsk = 100 * maxPrice
	curOB = np.zeros((math.ceil(maxPrice) * 100 + 1, 2))
	for event in checkpoint:
		curOB[int(round(event[2] * 100))][event[1]] = event[3]
		if event[3] > 0:
			if event[1] == 0:
				maxBid = max(maxBid, int(round(event[2] * 100)))
			if event[1] == 1:
				minAsk = min(minAsk, int(round(event[2] * 100)))
		
	#progressively query events
	cur.execute("SELECT COUNT(*) FROM " + eventsTable + 
				" WHERE time >= " + str(startTime) + 
				" AND time < " + str(endTime) + 
				" ORDER BY time ASC" + 
				";")
	cEvents = cur.fetchall()[0][0]

	#walk through events
	prevTimestep = firstTimestep
	lastTrade = None
	print("Processing", cEvents, "events in", math.ceil(cEvents // queryLimit), "parts...")
	for a in range(0, cEvents, queryLimit):
		cur.execute("SELECT * FROM " + eventsTable + 
					" WHERE time >= " + str(startTime) + 
					" AND time <= " + str(endTime) + 
					" ORDER BY time ASC" + 
					" LIMIT " + str(queryLimit) + 
					" OFFSET " + str(a) + 
					";")
		events = cur.fetchall()
	
		#walk through events
		for b in range(0, len(events)):
			event = events[b]

			#progress
			if b % 10000 == 0:
				print("{:4.2f}%".format(100 * (a + b) / cEvents), end = "\r")

			#only consider update events
			if event[4] == 0 or event[4] == 1 or event[4] == 5:
				continue
				
			#walk through missed timesteps
			for prevTimestep in range(prevTimestep, event[0], timestepSize):
				ts = (prevTimestep - firstTimestep) // timestepSize
				#compute metrics
				for cb in metricCallbacks:
					cb((ts, lastTrade, curOB, maxBid, minAsk))
				
			#keep track of last trade
			if event[4] == 3:
				lastTrade = event[2]

			#update ob
			pp = int(round(event[2] * 100))
			if event[1] == 1: #ask
				if event[3] > 0:
					minAsk = min(minAsk, pp)
				else:
					for minAsk in range(minAsk + 1, maxPrice + 1):
						if curOB[minAsk][1] > 0:
							break
			else: #bid
				if event[3] > 0:
					maxBid = max(maxBid, pp)
				else:
					for maxBid in range(maxBid, -1, -1):
						if curOB[maxBid][0] > 0:
							break
			curOB[pp][event[1]] = event[3]

	cur.close()
	conn.close()

	print("100.00%")
	
	#convert to dataframe
	df = pd.DataFrame(npMetrics, columns = metrics)
			
	return df

#metric functions
#takes arguments: metric array, the index of the metric, status
#status is a tuple: (time, lastTrade, curOB, maxBid, minAsk)
def computeMetricTime(npMetrics, id, status):
	npMetrics[status[0], id] = status[0]

def computeMetricMaxBid(npMetrics, id, status):
	npMetrics[status[0], id] = status[3]

def computeMetricMinAsk(npMetrics, id, status):
	npMetrics[status[0], id] = status[4]

def computeMetricBidVolume(npMetrics, id, status):
	npMetrics[status[0], id] = status[2][status[3]][0]

def computeMetricAskVolume(npMetrics, id, status):
	npMetrics[status[0], id] = status[2][status[4]][1]

def computeMetricMid(npMetrics, id, status):
	npMetrics[status[0], id] = (status[3] + status[4]) / 200

def computeMetricLastTrade(npMetrics, id, status):
	npMetrics[status[0], id] = status[1]

def computeMetricOIR(npMetrics, id, status):
	npMetrics[status[0], id] = None #not yet implemented