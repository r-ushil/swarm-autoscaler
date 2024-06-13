import os
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
from scipy import stats

# Create results directory if not exists
os.makedirs('results', exist_ok=True)

# Define directories and files
directories = ['keda', 'knative', 'reflex']
csv_files = {
    'keda': 'keda_zero_to_one.csv',
    'knative': 'knative_zero_to_one.csv',
    'reflex': 'swarm_zero_to_one.csv'
}

# Initialize a list to collect stats
stats_list = []

# Loop through directories and calculate statistics
for service, csv_file in csv_files.items():
    df = pd.read_csv(os.path.join(service, csv_file))
    mean_val = df['second_number'].mean()
    sem = stats.sem(df['second_number'])  # Standard error of the mean
    ci = sem * stats.t.ppf((1 + 0.95) / 2., len(df['second_number'])-1)  # 95% confidence interval
    perc_95 = df['second_number'].quantile(0.95)
    perc_99 = df['second_number'].quantile(0.99)

    # Append stats to the list
    stats_list.append({
        'Service': service.capitalize(),
        'Mean': mean_val,
        '95% CI Lower': mean_val - ci,
        '95% CI Upper': mean_val + ci,
        '95th Percentile': perc_95,
        '99th Percentile': perc_99
    })

# Convert the list to a DataFrame
stats_df = pd.DataFrame(stats_list)

# Save stats to a CSV file
stats_df.to_csv('results/stats.csv', index=False)

# Plot the bar chart
plt.figure(figsize=(8, 10))
bars = plt.bar(stats_df['Service'], stats_df['Mean'], color=['blue', 'gray', 'orange'], yerr=(stats_df['95% CI Upper'] - stats_df['Mean']), capsize=5, width=0.3)

# Add error bars for 95% CI and value labels
for idx, row in stats_df.iterrows():
    plt.errorbar(idx, row['Mean'], yerr=[[row['Mean'] - row['95% CI Lower']], [row['95% CI Upper'] - row['Mean']]], fmt='o', color='black', capsize=5)
    plt.text(idx, row['95% CI Upper'] + 0.1, f"{row['Mean']:.2f}", ha='center', va='bottom', fontsize=10, color='black')

# Adjust y-axis limit
plt.ylim(0, stats_df['95% CI Upper'].max() + 0.3)

# Add titles and labels with bold font
plt.title('Average Latency of Autoscalers for Scaling From 0 to 1\n(with 95% Confidence Intervals)', fontweight='bold')
plt.xlabel('Autoscaler', fontweight='bold')
plt.ylabel('Average Latency (s)', fontweight='bold')

# Add grid for better readability
plt.grid(axis='y', alpha=0.5)
# Save the plot
plt.savefig('results/zero-to-one.png')

# Show the plot
plt.show()

