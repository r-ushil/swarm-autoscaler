import os
import pandas as pd
from scipy import stats as scistats

def remove_outliers(df, column):
    Q1 = df[column].quantile(0.25)
    Q3 = df[column].quantile(0.75)
    IQR = Q3 - Q1
    lower_bound = Q1 - 1.5 * IQR
    upper_bound = Q3 + 1.5 * IQR
    outliers = df[(df[column] < lower_bound) | (df[column] > upper_bound)]
    return df[(df[column] >= lower_bound) & (df[column] <= upper_bound)]

def process_csv(file_path, service, state):
    df = pd.read_csv(file_path, header=None, names=['timestamp', 'namespace', 'pod', 'cpu_usage', 'memory_usage'])

    # Remove the 'm' from cpu_usage and handle cases like '.4m'
    df['cpu_usage'] = df['cpu_usage'].str.replace('m', '', regex=False).replace('', '0')
    df['cpu_usage'] = pd.to_numeric(df['cpu_usage'], errors='coerce').fillna(0).astype(float)

    # Remove the 'Mi' from memory_usage and handle cases like '15Mi'
    df['memory_usage'] = df['memory_usage'].str.replace('Mi', '', regex=False).replace('', '0')
    df['memory_usage'] = pd.to_numeric(df['memory_usage'], errors='coerce').fillna(0).astype(float)

    aggregated = df.groupby('timestamp').agg({'cpu_usage': 'sum', 'memory_usage': 'sum'}).reset_index()

    # Remove outliers based on aggregated totals
    #aggregated = remove_outliers(aggregated, 'cpu_usage')
    #aggregated = remove_outliers(aggregated, 'memory_usage')

    mean_cpu = aggregated['cpu_usage'].mean()
    mean_memory = aggregated['memory_usage'].mean()
    sem_cpu = scistats.sem(aggregated['cpu_usage'])
    sem_memory = scistats.sem(aggregated['memory_usage'])
    ci_cpu = sem_cpu * scistats.t.ppf((1 + 0.95) / 2., len(aggregated) - 1)
    ci_memory = sem_memory * scistats.t.ppf((1 + 0.95) / 2., len(aggregated) - 1)

    return {
        'autoscaler': service,
        'state': state,
        'avg_cpu_millicores': mean_cpu,
        'avg_memory_mib': mean_memory,
        '95_ci_cpu': ci_cpu,
        '95_ci_memory': ci_memory
    }

def main():
    # Create results directory if not exists
    os.makedirs('results', exist_ok=True)

    # Define directories and files
    directories = ['k8s_hpa', 'keda', 'knative', 'reflex_faas']
    file_types = ['metrics.csv', 'metrics_idle.csv']
    services = {
        'k8s_hpa': 'Kubernetes HPA',
        'keda': 'KEDA',
        'knative': 'Knative',
        'reflex_faas': 'Reflex (Serverless)'
        'reflex_caas': 'Reflex (Microservice-Based)'
    }

    stats_list = []

    # Loop through directories and process files
    for directory in directories:
        for file_type in file_types:
            state = 'idle' if 'idle' in file_type else 'busy'
            file_path = os.path.join(directory, 'results', f'{directory}_{file_type}')
            if os.path.exists(file_path):
                service = services[directory]
                stats = process_csv(file_path, service, state)
                stats_list.append(stats)

    # Convert stats list to DataFrame and save to CSV
    stats_df = pd.DataFrame(stats_list)
    stats_df.to_csv('results/stats.csv', index=False)

if __name__ == "__main__":
    main()
