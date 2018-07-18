import os

def toRelPath(origPath):
	"""Converts path to path relative to current script

	origPath:	path to convert
	"""
	if not hasattr(toRelPath, "__location__"):
		toRelPath.__location__ = os.path.realpath(os.path.join(os.getcwd(), os.path.dirname(__file__)))
	return os.path.join(toRelPath.__location__, origPath)

####end of library

import pymysql
import json

DB_TABLE = "gemini_monitor"

conn = pymysql.connect(
	host="inmensus-trading-db-1.ce50oojfsygk.us-east-2.rds.amazonaws.com",
	user="GILGAMESH",
	passwd="3Dd7tAN66wqbjDaD",
	db="inmensus_trading_db_1")
cur = conn.cursor()

cur.execute("SHOW columns FROM " + DB_TABLE + ";")
print("Columns:", cur.fetchall())

cur.execute("SELECT count(*) FROM " + DB_TABLE + ";")
rows = cur.fetchall()[0][0]
print("Size:", rows)

ROWS = 50000
fetchDB = input("(y/n) Fetch full database? ")
if fetchDB == "y":
	#get all data
	with open(toRelPath("db.json"), "w") as outfile:
		outfile.write("[")
		for a in range(0, rows, ROWS):
			print("Fetching", ROWS, "rows with offset", a)
			cur.execute("SELECT * FROM " + DB_TABLE + " LIMIT " + str(ROWS) + " OFFSET " + str(a) + ";")
			data = cur.fetchall()

			for b in range(len(data)):
				outfile.write("[" + str(data[b][0]) + "," + 
					str(data[b][1]) + "," + 
					str(data[b][2][0]) + "," + 
					str(data[b][3]) + "," + 
					str(data[b][4]) + "," + 
					str(data[b][5]) + "]")
				if b != len(data) - 1:
					outfile.write(",")
			if rows - a > ROWS:
				outfile.write(",")
		outfile.write("]")

cur.close()
conn.close()