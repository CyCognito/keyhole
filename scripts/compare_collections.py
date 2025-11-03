#!/usr/bin/env python3
"""
Extract collection statistics from MongoDB HTML reports and output as CSV.
Each row contains: date, collection name, and metrics.
"""

import re
import sys
import csv

def parse_size(size_str):
    """Convert size string (e.g., '1.5 GB', '500 KB') to bytes."""
    if not size_str or size_str == 'N/A':
        return 0
    size_str = size_str.strip().upper()
    multipliers = {'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3, 'TB': 1024**4, 'PB': 1024**5}
    match = re.match(r'([\d.]+)\s*([A-Z]+)?', size_str)
    if not match:
        return 0
    value = float(match.group(1))
    unit = match.group(2) or 'B'
    return int(value * multipliers.get(unit, 1))

def parse_number(num_str):
    """Parse number string with commas."""
    if not num_str:
        return 0
    return int(num_str.replace(',', ''))

def extract_generated_date(html_content):
    """Extract the 'Generated' timestamp from HTML content."""
    # Pattern: Generated: 2006-01-02 15:04:05
    match = re.search(r'Generated:\s*([\d]{4}-[\d]{2}-[\d]{2}\s+[\d]{2}:[\d]{2}:[\d]{2})', html_content)
    if match:
        return match.group(1)
    return "Unknown"

def extract_collections(html_content):
    """Extract collection statistics from HTML content."""
    collections = {}
    
    # Find all collection sections
    # Pattern: <h3>database.collection</h3> followed by a table with stats
    pattern = r'<h3>([^<]+)</h3>.*?<table>.*?<tr><td>Number of Documents</td><td>([^<]+)</td></tr>.*?<tr><td>Average Document Size</td><td>([^<]+)</td></tr>.*?<tr><td>Data Size</td><td>([^<]+)</td></tr>.*?<tr><td>Storage Size</td><td>([^<]+)</td></tr>'
    
    matches = re.finditer(pattern, html_content, re.DOTALL)
    
    for match in matches:
        ns = match.group(1).strip()
        count = parse_number(match.group(2))
        avg_size = parse_size(match.group(3))
        data_size = parse_size(match.group(4))
        storage_size = parse_size(match.group(5))
        
        collections[ns] = {
            'count': count,
            'avg_size': avg_size,
            'data_size': data_size,
            'storage_size': storage_size
        }
    
    return collections

def main():
    if len(sys.argv) < 3:
        print("Usage: compare_collections.py <file1> <file2> [output.csv]")
        sys.exit(1)
    
    file1 = sys.argv[1]
    file2 = sys.argv[2]
    output_file = sys.argv[3] if len(sys.argv) > 3 else 'collection_stats.csv'
    
    try:
        with open(file1, 'r') as f:
            content1 = f.read()
        with open(file2, 'r') as f:
            content2 = f.read()
    except FileNotFoundError as e:
        print(f"Error: File not found - {e}", file=sys.stderr)
        sys.exit(1)
    
    # Extract dates and collections from both files
    date1 = extract_generated_date(content1)
    date2 = extract_generated_date(content2)
    collections1 = extract_collections(content1)
    collections2 = extract_collections(content2)
    
    # Get all unique collection names
    all_collections = sorted(set(collections1.keys()) | set(collections2.keys()))
    
    # Write CSV
    with open(output_file, 'w', newline='') as csvfile:
        fieldnames = ['Date', 'Collection', 'Document Count', 'Average Document Size (bytes)', 
                      'Data Size (bytes)', 'Storage Size (bytes)']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        
        writer.writeheader()
        
        # Write rows for file1
        for ns in all_collections:
            if ns in collections1:
                c = collections1[ns]
                writer.writerow({
                    'Date': date1,
                    'Collection': ns,
                    'Document Count': c['count'],
                    'Average Document Size (bytes)': c['avg_size'],
                    'Data Size (bytes)': c['data_size'],
                    'Storage Size (bytes)': c['storage_size']
                })
            else:
                # Collection exists in file2 but not file1
                writer.writerow({
                    'Date': date1,
                    'Collection': ns,
                    'Document Count': '',
                    'Average Document Size (bytes)': '',
                    'Data Size (bytes)': '',
                    'Storage Size (bytes)': ''
                })
        
        # Write rows for file2
        for ns in all_collections:
            if ns in collections2:
                c = collections2[ns]
                writer.writerow({
                    'Date': date2,
                    'Collection': ns,
                    'Document Count': c['count'],
                    'Average Document Size (bytes)': c['avg_size'],
                    'Data Size (bytes)': c['data_size'],
                    'Storage Size (bytes)': c['storage_size']
                })
            else:
                # Collection exists in file1 but not file2
                writer.writerow({
                    'Date': date2,
                    'Collection': ns,
                    'Document Count': '',
                    'Average Document Size (bytes)': '',
                    'Data Size (bytes)': '',
                    'Storage Size (bytes)': ''
                })
    
    print(f"CSV file written to: {output_file}")
    print(f"Date 1 ({file1}): {date1}")
    print(f"Date 2 ({file2}): {date2}")
    print(f"Total collections: {len(all_collections)}")
    print(f"Total rows: {len(all_collections) * 2}")

if __name__ == '__main__':
    main()

