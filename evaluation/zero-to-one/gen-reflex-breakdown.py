import pandas as pd
import matplotlib.pyplot as plt

# Load data from files
scaling_decisions_file = 'reflex/scaling.log'
latency_data_file = 'reflex/swarm_zero_to_one.csv'

# Read scaling decisions
with open(scaling_decisions_file, 'r') as f:
    scaling_decisions = f.readlines()

# Process scaling decisions
scaling_decisions = [line.split(' ')[0] + ' ' + line.split(' ')[1] for line in scaling_decisions]
decision_times = pd.DataFrame(scaling_decisions, columns=["time"])
decision_times["time"] = pd.to_datetime(decision_times["time"])

# Read latency data
latency_data = pd.read_csv(latency_data_file)
latency_data["time"] = pd.to_datetime(latency_data["time"])

# Ensure latency column exists
if 'second_number' in latency_data.columns:
    latency_data.rename(columns={'second_number': 'latency'}, inplace=True)
else:
    print("Error: 'second_number' column not found in latency data.")
    exit()

# Merge DataFrames on time
merged_df = pd.merge_asof(latency_data.sort_values('time'), decision_times.sort_values('time'), on="time", direction="backward")

# Calculate decision times
merged_df["decision_time"] = (merged_df["time"] - decision_times["time"]).dt.total_seconds()

# Ensure the merged DataFrame has the required columns
if 'latency' not in merged_df.columns or 'decision_time' not in merged_df.columns:
    print("Error: Required columns not found in merged DataFrame.")
    print(merged_df.head())
    exit()

# Plotting
plt.figure(figsize=(10, 6))
plt.plot(merged_df["time"], merged_df["latency"], label="Scaling Latency", color="blue")

plt.fill_between(merged_df["time"], 0, merged_df["decision_time"], label="Decision Time", color="brown", alpha=0.5)
plt.fill_between(merged_df["time"], merged_df["decision_time"], merged_df["latency"], label="Scaling Time", color="orange", alpha=0.5)

plt.xlabel("Time")
plt.ylabel("Latency (seconds)")
plt.title("Scaling Latency Over Time")
plt.legend(loc="center left", bbox_to_anchor=(1, 0.5))
plt.grid(True)

# Save the plot as a PNG file
output_file = 'results/reflex-breakdown.png'
plt.savefig(output_file, bbox_inches='tight')

print(f"Plot saved as {output_file}")

