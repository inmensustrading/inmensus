import sql_to_dataframe as std
import matplotlib.pyplot as plt
import rain

def main():
	df = std.sqlToDataframe(rain.toRelPath("data\\dataframe-cache.csv"), 
		useCache = False,
		metrics = ["time", 
			"max-bid", 
			"min-ask", 
			"mid", 
			"bid-volume",
			"ask-volume", 
			"last-trade", 
			"oir"
		],
		sqlPassword = "3Dd7tAN66wqbjDaD")
	plt.plot(df["time"], df["mid"])
	plt.show()

if __name__ == "__main__":
	main()