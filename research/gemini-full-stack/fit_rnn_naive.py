import matplotlib.pyplot as plt
import pandas as pd
import numpy as np
import keras
import os

import rain

rain.setCUDAVisible("0, 1")

MODEL_NAME = "0"
DATA_CACHE = "data/df-cache.csv"
FREQUENCY = 6
TIMESTEP_LEN = 150
VALIDATION_RATIO = 0.05
EPOCHS = 8
BATCH_SIZE = 256

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

#data
df = pd.read_csv(rain.toRelPath(DATA_CACHE))
df = df.iloc[::int(2 * FREQUENCY), :]
df.reset_index(inplace = True)
mid = df["mid"]
mid.fillna(method = "bfill", inplace = True)
plt.plot(mid)
plt.legend()
plt.show()

#prep
train = [[], []]
for a in range(len(mid) - TIMESTEP_LEN):
    train[0].append(mid[a:a + TIMESTEP_LEN].values)
    train[1].append(mid[a + TIMESTEP_LEN])
train[0] = np.reshape(train[0], (len(train[0]), TIMESTEP_LEN, 1))
print("Patters:", len(train[0]))

valCount = int(VALIDATION_RATIO * len(train[0]))
validation = [train[0][-valCount:], train[1][-valCount:]]
train = [train[0][:-valCount], train[1][:-valCount]]
print(len(train[0]), len(validation[0]))
print(train[0][0], train[1][0])

#fit
model.fit(
    train[0], 
    train[1], 
    epochs = EPOCHS, 
    batch_size = BATCH_SIZE, 
    callbacks = [
        keras.callbacks.ModelCheckpoint(
            rain.toRelPath("weights/" + MODEL_NAME + "_{epoch}_{val_loss:.4f}.h5"), 
            save_weights_only = True
        )
    ],
    validation_data = (
        validation[0], 
        validation[1])
    )