import pandas as pd
import matplotlib.pyplot as plt
import os

# Ensure the results directory exists
os.makedirs('results', exist_ok=True)

df = pd.read_csv("results/stats.csv")
filtered_df = df[df['autoscaler'].isin(['Kubernetes HPA', 'Reflex (Microservice-Based)'])]

# Define the plot parameters
plot_params = [
    ('avg_memory_mib', 'idle', 'Average Memory Usage (Idle)', 'avg_memory_idle.png'),
    ('avg_memory_mib', 'busy', 'Average Memory Usage (Busy)', 'avg_memory_busy.png'),
    ('avg_cpu_millicores', 'idle', 'Average CPU Usage (Idle)', 'avg_cpu_idle.png'),
    ('avg_cpu_millicores', 'busy', 'Average CPU Usage (Busy)', 'avg_cpu_busy.png'),
]
def create_bar_plot(column, state, title, filename):
    subset = filtered_df[filtered_df['state'] == state]
    plt.figure(figsize=(10, 6))
    bars = plt.bar(subset['autoscaler'], subset[column], color=['blue', 'gray', 'orange'])
    plt.xlabel('Autoscaler')
    plt.ylabel(column.replace('_', ' ').title())
    plt.title(title)
    
    # Add value labels on top of each bar, centralized
    for bar in bars:
        yval = bar.get_height()
        plt.text(bar.get_x() + bar.get_width()/2.0, yval, round(yval, 2), ha='center', va='bottom')  # ha: horizontal alignment, va: vertical alignment
    
    plt.savefig(f'results/{filename}')
    plt.close()

# Generate and save the plots
for col, state, title, fname in plot_params:
    create_bar_plot(col, state, title, fname)

