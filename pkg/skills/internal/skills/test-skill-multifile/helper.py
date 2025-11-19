#!/usr/bin/env python3
"""
Helper script for processing data
This demonstrates a script that can be loaded into context or executed directly
"""

def process_data(input_text):
    """Process input text and return result"""
    return f"Processed: {input_text.upper()}"

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1:
        result = process_data(sys.argv[1])
        print(result)
    else:
        print("Usage: python helper.py <input_text>")
