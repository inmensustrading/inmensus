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

print("Columns:")
cur.execute("SHOW columns FROM gemini_change;")
for r in cur:
    print(r)
cur.execute("SELECT count(*) FROM gemini_change;")
print("Size: ", end="")
for r in cur:
    print(r)

clearDB = input("Reset database? (y/n): ")
if clearDB == "y":
	cur.execute("DELETE FROM gemini_change;")

	cur.execute("SELECT count(*) FROM gemini_change;")
	print("Size: ", end="")
	for r in cur:
		print(r)

input("Enter to fetch full database...")

#get all data
cur.execute("SELECT * FROM gemini_change LIMIT 10;")
with open(toRelPath("db.json"), "w") as outfile:
    json.dump(cur.fetchall(), outfile)

cur.close()
conn.close()