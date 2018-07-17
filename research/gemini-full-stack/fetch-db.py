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
import numpy

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
print("Size:", cur.fetchall())

cur.execute("SELECT * FROM " + DB_TABLE + " WHERE reason = 'connect';")
print(cur.fetchall())

#cur.execute("DELETE FROM gemini_change;")

input("Enter to fetch full database...")

#get all data
cur.execute("SELECT * FROM " + DB_TABLE + ";")
with open(toRelPath("db.json"), "w") as outfile:
    json.dump(cur.fetchall(), outfile)

cur.close()
conn.close()