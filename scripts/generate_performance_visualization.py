#!/usr/bin/env python3

"""
MinZ Performance Visualization Generator
Creates visual reports from test results
"""

import csv
import json
import os
import sys
from datetime import datetime
from pathlib import Path

# ASCII art for terminal visualization
def generate_ascii_chart(data, title, max_width=60):
    """Generate ASCII bar chart"""
    print(f"\n{title}")
    print("=" * (max_width + 20))
    
    if not data:
        print("No data available")
        return
    
    max_value = max(data.values())
    
    for label, value in sorted(data.items(), key=lambda x: -x[1]):
        bar_width = int((value / max_value) * max_width)
        bar = "█" * bar_width
        print(f"{label:20} |{bar} {value:.1f}%")

def generate_html_report(test_dir):
    """Generate interactive HTML report with charts"""
    
    # Read performance data
    perf_file = Path(test_dir) / "performance_report.csv"
    if not perf_file.exists():
        print(f"Error: {perf_file} not found")
        return
    
    performance_data = []
    with open(perf_file, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            performance_data.append(row)
    
    # Generate HTML
    html_content = f"""
<!DOCTYPE html>
<html>
<head>
    <title>MinZ Performance Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body {{
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }}
        .container {{
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }}
        h1 {{
            color: #333;
            text-align: center;
        }}
        .chart-container {{
            width: 48%;
            display: inline-block;
            margin: 1%;
            height: 400px;
        }}
        .summary {{
            background-color: #e8f4f8;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }}
        table {{
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }}
        th, td {{
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }}
        th {{
            background-color: #4CAF50;
            color: white;
        }}
        tr:hover {{
            background-color: #f5f5f5;
        }}
        .improved {{
            color: green;
            font-weight: bold;
        }}
        .regression {{
            color: red;
            font-weight: bold;
        }}
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 MinZ Optimization Performance Report</h1>
        <p style="text-align: center; color: #666;">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>
        
        <div class="summary">
            <h2>📊 Summary Statistics</h2>
            <ul>
                <li>Total Examples Tested: <strong>{len(performance_data)}</strong></li>
                <li>Average Cycle Improvement: <strong>{calculate_average_improvement(performance_data):.1f}%</strong></li>
                <li>Best Performer: <strong>{get_best_performer(performance_data)}</strong></li>
                <li>Examples with Improvements: <strong>{count_improvements(performance_data)}</strong></li>
            </ul>
        </div>
        
        <div class="chart-container">
            <canvas id="cycleChart"></canvas>
        </div>
        <div class="chart-container">
            <canvas id="sizeChart"></canvas>
        </div>
        
        <h2>📋 Detailed Results</h2>
        <table id="resultsTable">
            <thead>
                <tr>
                    <th>Example File</th>
                    <th>Size Reduction</th>
                    <th>Instruction Reduction</th>
                    <th>Cycle Improvement</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody>
                {generate_table_rows(performance_data)}
            </tbody>
        </table>
    </div>
    
    <script>
        // Cycle Improvement Chart
        const cycleCtx = document.getElementById('cycleChart').getContext('2d');
        const cycleChart = new Chart(cycleCtx, {{
            type: 'bar',
            data: {{
                labels: {json.dumps(get_example_names(performance_data))},
                datasets: [{{
                    label: 'Cycle Improvement %',
                    data: {json.dumps(get_cycle_improvements(performance_data))},
                    backgroundColor: 'rgba(75, 192, 192, 0.6)',
                    borderColor: 'rgba(75, 192, 192, 1)',
                    borderWidth: 1
                }}]
            }},
            options: {{
                responsive: true,
                maintainAspectRatio: false,
                plugins: {{
                    title: {{
                        display: true,
                        text: 'Estimated Cycle Improvements by Example'
                    }}
                }},
                scales: {{
                    y: {{
                        beginAtZero: true,
                        ticks: {{
                            callback: function(value) {{
                                return value + '%';
                            }}
                        }}
                    }}
                }}
            }}
        }});
        
        // Size Reduction Chart
        const sizeCtx = document.getElementById('sizeChart').getContext('2d');
        const sizeChart = new Chart(sizeCtx, {{
            type: 'bar',
            data: {{
                labels: {json.dumps(get_example_names(performance_data))},
                datasets: [{{
                    label: 'Size Reduction %',
                    data: {json.dumps(get_size_reductions(performance_data))},
                    backgroundColor: 'rgba(255, 159, 64, 0.6)',
                    borderColor: 'rgba(255, 159, 64, 1)',
                    borderWidth: 1
                }}]
            }},
            options: {{
                responsive: true,
                maintainAspectRatio: false,
                plugins: {{
                    title: {{
                        display: true,
                        text: 'Code Size Reductions by Example'
                    }}
                }},
                scales: {{
                    y: {{
                        beginAtZero: true,
                        ticks: {{
                            callback: function(value) {{
                                return value + '%';
                            }}
                        }}
                    }}
                }}
            }}
        }});
    </script>
</body>
</html>
"""
    
    # Save HTML report
    html_file = Path(test_dir) / "performance_report.html"
    with open(html_file, 'w') as f:
        f.write(html_content)
    
    print(f"✅ HTML report generated: {html_file}")
    
    # Also generate ASCII visualization for terminal
    cycle_data = {}
    for row in performance_data[:10]:  # Top 10 for terminal display
        if row.get('Est. Cycle Improvement %'):
            try:
                cycle_data[Path(row['File']).name] = float(row['Est. Cycle Improvement %'])
            except ValueError:
                pass
    
    generate_ascii_chart(cycle_data, "Top 10 Cycle Improvements")

# Helper functions for HTML generation
def calculate_average_improvement(data):
    improvements = []
    for row in data:
        if row.get('Est. Cycle Improvement %'):
            try:
                improvements.append(float(row['Est. Cycle Improvement %']))
            except ValueError:
                pass
    return sum(improvements) / len(improvements) if improvements else 0

def get_best_performer(data):
    best = None
    best_improvement = -999
    for row in data:
        if row.get('Est. Cycle Improvement %'):
            try:
                improvement = float(row['Est. Cycle Improvement %'])
                if improvement > best_improvement:
                    best_improvement = improvement
                    best = Path(row['File']).name
            except ValueError:
                pass
    return f"{best} ({best_improvement:.1f}%)" if best else "N/A"

def count_improvements(data):
    count = 0
    for row in data:
        if row.get('Est. Cycle Improvement %'):
            try:
                if float(row['Est. Cycle Improvement %']) > 0:
                    count += 1
            except ValueError:
                pass
    return count

def get_example_names(data):
    return [Path(row['File']).name for row in data if row.get('File')]

def get_cycle_improvements(data):
    improvements = []
    for row in data:
        if row.get('Est. Cycle Improvement %'):
            try:
                improvements.append(float(row['Est. Cycle Improvement %']))
            except ValueError:
                improvements.append(0)
        else:
            improvements.append(0)
    return improvements

def get_size_reductions(data):
    reductions = []
    for row in data:
        if row.get('Size Reduction %'):
            try:
                reductions.append(float(row['Size Reduction %']))
            except ValueError:
                reductions.append(0)
        else:
            reductions.append(0)
    return reductions

def generate_table_rows(data):
    rows = []
    for row in data:
        try:
            cycle_imp = float(row.get('Est. Cycle Improvement %', 0))
            size_red = float(row.get('Size Reduction %', 0))
            inst_red = float(row.get('Instruction Reduction %', 0))
            
            status_class = 'improved' if cycle_imp > 0 else ('regression' if cycle_imp < 0 else '')
            status = '✅ Improved' if cycle_imp > 0 else ('⚠️ Regression' if cycle_imp < 0 else '➖ No change')
            
            rows.append(f"""
                <tr>
                    <td>{Path(row['File']).name}</td>
                    <td>{size_red:.1f}%</td>
                    <td>{inst_red:.1f}%</td>
                    <td class="{status_class}">{cycle_imp:.1f}%</td>
                    <td>{status}</td>
                </tr>
            """)
        except (ValueError, KeyError):
            pass
    
    return ''.join(rows)

def main():
    if len(sys.argv) < 2:
        print("Usage: generate_performance_visualization.py <test_results_directory>")
        sys.exit(1)
    
    test_dir = sys.argv[1]
    if not os.path.exists(test_dir):
        print(f"Error: Directory {test_dir} not found")
        sys.exit(1)
    
    print(f"🎨 Generating performance visualizations for {test_dir}")
    generate_html_report(test_dir)
    print("\n✅ Visualization complete!")

if __name__ == "__main__":
    main()