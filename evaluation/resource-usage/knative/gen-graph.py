import pandas as pd
import matplotlib.pyplot as plt

# Load the CSV data
data = pd.read_csv('knative_metrics.csv')

# Convert timestamp column to datetime
data['timestamp'] = pd.to_datetime(data['timestamp'])

# Remove the 'm' and 'Mi' units and convert to numeric values
data['cpu_usage'] = data['cpu_usage'].str.replace('m', '').astype(float)
data['memory_usage'] = data['memory_usage'].str.replace('Mi', '').astype(float)

# Aggregate CPU and memory usage by timestamp
total_usage = data.groupby('timestamp').agg({
    'cpu_usage': 'sum',
    'memory_usage': 'sum'
}).reset_index()

# Plot total CPU usage
plt.figure(figsize=(14, 7))
plt.plot(total_usage['timestamp'], total_usage['cpu_usage'], marker='o', linestyle='-')
plt.title('Total CPU Usage Over Time')
plt.xlabel('Timestamp')
plt.ylabel('CPU Usage (millicores)')
plt.grid(True)
plt.tight_layout()
plt.savefig('graphs/total_cpu_usage.png')
plt.show()

# Plot total memory usage
plt.figure(figsize=(14, 7))
plt.plot(total_usage['timestamp'], total_usage['memory_usage'], marker='o', linestyle='-')
plt.title('Total Memory Usage Over Time')
plt.xlabel('Timestamp')
plt.ylabel('Memory Usage (Mi)')
plt.grid(True)
plt.tight_layout()
plt.savefig('graphs/total_memory_usage.png')
plt.show()

