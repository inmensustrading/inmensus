import matplotlib.pyplot as plt
import pandas as pd
import numpy as np
import keras
import os

import rain

rain.setCUDAVisible("-1")

DATA_CACHE = "data/df-cache.csv"
FREQUENCY = 6
TIMESTEP_LEN = 150
PREDICT_LEN = 100

#model
model = keras.models.Sequential()
#timesteps of history - with a vector of size 1 per timestep
model.add(keras.layers.GRU(128, input_shape = (TIMESTEP_LEN, 1), return_sequences = True))
model.add(keras.layers.BatchNormalization())
model.add(keras.layers.Activation("tanh"))
model.add(keras.layers.Dropout(0.2))
model.add(keras.layers.GRU(128))
model.add(keras.layers.BatchNormalization())
model.add(keras.layers.Activation("tanh"))
model.add(keras.layers.Dropout(0.2))
model.add(keras.layers.Dense(1))
model.add(keras.layers.Activation("linear"))
model.compile(loss = "mean_squared_logarithmic_error", 
	optimizer = keras.optimizers.adam(lr = 0.0003))
model.load_weights(rain.toRelPath("weights/0-1-49.8703.h5"))

#data
df = pd.read_csv(rain.toRelPath(DATA_CACHE))
df = df.iloc[::int(2 * FREQUENCY), :]
df.reset_index(inplace = True)
mid = df["mid"]
mid.fillna(method = "bfill", inplace = True)
plt.plot(mid)
plt.legend()
plt.show()

#predict from random place in the sequence
start = np.random.randint(0, len(mid) // 2)
pattern = mid[start:start + TIMESTEP_LEN].values.tolist()
predictions = pattern
for a in range(PREDICT_LEN):
	predictions.append(model.predict(
		np.reshape(pattern, (1, TIMESTEP_LEN, 1)), 
		verbose = 0)[0][0])
	pattern = predictions[-TIMESTEP_LEN:]

print(predictions)
plt.plot(predictions)
plt.axvline(x = TIMESTEP_LEN, c = "r")
plt.legend()
plt.show()