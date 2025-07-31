#!/usr/bin/env python3

import os
import subprocess
import re
from datetime import datetime

# Change to docs directory
os.chdir('docs')

# Get all markdown files with their git creation dates
files_with_dates = []

for filename in os.listdir('.'):
    if filename.endswith('.md') and filename not in ['README.md', 'CLAUDE.md']:
        # Get creation date from git
        try:
            result = subprocess.run(
                ['git', 'log', '--follow', '--format=%ai', '--reverse', filename],
                capture_output=True, text=True
            )
            if result.stdout:
                date_str = result.stdout.strip().split('\n')[0]
                # Parse the date
                date = datetime.strptime(date_str[:19], '%Y-%m-%d %H:%M:%S')
                files_with_dates.append((date, filename))
        except Exception as e:
            print(f"Error processing {filename}: {e}")

# Sort by date
files_with_dates.sort(key=lambda x: x[0])

# Rename files
special_files = ['compiler-enhancements-success-report.md']
counter = 1

for date, old_name in files_with_dates:
    if old_name in special_files:
        print(f"Keeping special file: {old_name}")
        continue
    
    # Extract title from filename
    match = re.match(r'^\d+_(.+)$', old_name)
    if match:
        title = match.group(1)
    else:
        # No number prefix, use whole name without .md
        title = old_name
    
    # Create new name
    new_name = f"{counter:03d}_{title}"
    
    # Ensure it ends with .md
    if not new_name.endswith('.md'):
        new_name = new_name.replace('.md', '') + '.md'
    
    if old_name != new_name:
        print(f"Renaming: {old_name} -> {new_name}")
        subprocess.run(['git', 'mv', old_name, new_name])
    else:
        print(f"Already correct: {old_name}")
    
    counter += 1

print("\nDone! All files renamed in chronological order.")