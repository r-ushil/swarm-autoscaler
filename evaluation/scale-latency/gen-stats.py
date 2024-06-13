import os
import pandas as pd
import numpy as np
import scipy.stats as stats
import matplotlib.pyplot as plt

# Function to compute statistics
def compute_statistics(data):
    mean_latency = np.mean(data)
    ci_95 = stats.norm.interval(0.95, loc=mean_latency, scale=stats.sem(data))
    percentile_95 = np.percentile(data, 95)
    percentile_99 = np.percentile(data, 99)
    
    return mean_latency, ci_95, percentile_95, percentile_99

# Directory paths and labels
autoscalers = {
    "k8s-hpa": "Kubernetes\nHPA",
    "swarm-caas": "Reflex\n(Microservice-Based)",
    "knative": "Knative",
    "swarm-faas": "Reflex\n(Serverless)"
}
colors = {
    "k8s-hpa": "green",
    "swarm-caas": "orange",
    "knative": "gray",
    "swarm-faas": "orange"
}
result_file = "results/stats.csv"
os.makedirs("results", exist_ok=True)

# Collect statistics for each autoscaler
stats_data = []

for autoscaler, label in autoscalers.items():
    csv_path = os.path.join(autoscaler.split('\n')[0], "scale_up_latency.csv")
    if os.path.exists(csv_path):
        df = pd.read_csv(csv_path, header=None, names=["index", "start_time", "end_time", "latency"])
        mean_latency, ci_95, percentile_95, percentile_99 = compute_statistics(df["latency"])
        stats_data.append([label, mean_latency, ci_95[0], ci_95[1], percentile_95, percentile_99])

# Save statistics to CSV
stats_df = pd.DataFrame(stats_data, columns=["Autoscaler", "Mean Latency", "CI 95 Lower", "CI 95 Upper", "95th Percentile", "99th Percentile"])
stats_df.to_csv(result_file, index=False)

# Plotting
plt.figure(figsize=(10, 8))
mean_latencies = stats_df["Mean Latency"]
ci_lower = stats_df["CI 95 Lower"]
ci_upper = stats_df["CI 95 Upper"]
error_bars = [mean_latencies - ci_lower, ci_upper - mean_latencies]

bars = plt.bar(stats_df["Autoscaler"], mean_latencies, color=[colors[key.split('\n')[0]] for key in autoscalers], yerr=error_bars, capsize=5)
plt.xlabel("Autoscaler", fontweight='bold')
plt.ylabel("Average Scaling Latency (seconds)", fontweight='bold')
#plt.title("Mean Scaling Latency with 95% CI", fontweight='bold')
plt.grid(axis='y', alpha=0.5)

# Adjust x-axis labels
plt.xticks(rotation=0, ha='center')

# Adding labels to the top of the error bars
for bar, mean_latency, ci_high in zip(bars, mean_latencies, ci_upper):
    yval = bar.get_height()
    plt.text(bar.get_x() + bar.get_width() / 2.0, ci_high + 0.1, f'{mean_latency:.2f}', ha='center', va='bottom', fontsize=10, color='black')

# Adjust y-axis limit
plt.ylim(0, ci_upper.max() + 0.3)

# Save the plot
plt.savefig("results/mean_latency_bar_chart.png")

# Show the plot
plt.show()

