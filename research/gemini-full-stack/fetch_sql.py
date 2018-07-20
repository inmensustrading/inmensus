import sql_to_dataframe as std
import matplotlib.pyplot as plt
import rain

def main():
	df = std.sqlToDataframe(
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
	print(df)
	df.to_csv(rain.toRelPath("data\\df-cache.csv"), index = False)
	plt.plot(df["time"], df["mid"])
	plt.show()

if __name__ == "__main__":
	main()