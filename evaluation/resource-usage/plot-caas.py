import pandas as pd
import matplotlib.pyplot as plt
import os

# Ensure the results directory exists
os.makedirs('results', exist_ok=True)

df = pd.read_csv("results/stats.csv")
filtered_df = df[df['autoscaler'].isin(['Kubernetes HPA', 'Reflex (Microservice-Based)'])]

print(filtered_df)
# Define the plot parameters
plot_params = [
    ('avg_memory_mib', 'idle', 'Average Memory Usage (Idle)', 'caas_avg_memory_idle.png'),
    ('avg_memory_mib', 'busy', 'Average Memory Usage (Busy)', 'caas_avg_memory_busy.png'),
    ('avg_cpu_millicores', 'idle', 'Average CPU Usage (Idle)', 'caas_avg_cpu_idle.png'),
    ('avg_cpu_millicores', 'busy', 'Average CPU Usage (Busy)', 'caas_avg_cpu_busy.png'),
]
def create_bar_plot(column, state, title, filename):
    plt.figure(figsize=(10, 12))
    subset = filtered_df[filtered_df['state'] == state]
    bars = plt.bar(subset['autoscaler'], subset[column], color=['green', 'orange'])
    plt.xlabel('Autoscaler', fontweight='bold')

    if title == "Average Memory Usage (Idle)" or title == "Average Memory Usage (Busy)":
        y_axis = "Average Memory Usage (MiB)"
    elif title == "Average CPU Usage (Idle)" or title == "Average CPU Usage (Busy)":
        y_axis = "Average CPU Utilisation (millicores)"

    plt.ylabel(y_axis, fontweight='bold')
    plt.title(title, fontweight='bold')
    plt.grid(axis='y', alpha=0.5)
    
    # Add value labels on top of each bar, centralized
    for bar in bars:
        yval = bar.get_height()
        plt.text(bar.get_x() + bar.get_width()/2.0, yval, round(yval, 2), ha='center', va='bottom')  # ha: horizontal alignment, va: vertical alignment
    
    plt.savefig(f'results/{filename}')
    plt.close()

# Generate and save the plots
for col, state, title, fname in plot_params:
    create_bar_plot(col, state, title, fname)

