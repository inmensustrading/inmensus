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

conn = pymysql.connect(
	host="inmensus-trading-db-1.ce50oojfsygk.us-east-2.rds.amazonaws.com",
	user="GILGAMESH",
	passwd="3Dd7tAN66wqbjDaD",
	db="inmensus_trading_db_1")
cur = conn.cursor()

cur.execute("delete from gemini_change;")
cur.execute("show columns from gemini_change;")
for r in cur:
    print(r)
cur.execute("select count(*) from gemini_change;")
print("Size: ", end="")
for r in cur:
    print(r)
input("Enter to continue fetching...")

#get all data
cur.execute("select * from gemini_change;")
with open(toRelPath("assets\\fetch-mysql.json"), "w") as outfile:
    json.dump(cur.fetchall(), outfile)

cur.close()
conn.close()