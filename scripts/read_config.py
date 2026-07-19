#!/usr/bin/env python3
"""Read a value from config.yaml by section and key name.
Usage: python3 read_config.py <section> <key> [config_path]

Exit codes: 0=success, 1=not found or error
"""
import sys

def read_config(config_path, section, key):
    in_section = False
    try:
        with open(config_path) as f:
            for line in f:
                raw = line.rstrip('\n')
                stripped = raw.strip()
                if not stripped or stripped.startswith('#'):
                    continue
                indent = len(raw) - len(raw.lstrip())
                if indent == 0:
                    in_section = (stripped.rstrip(':') == section)
                elif in_section:
                    if ':' in stripped:
                        k, _, v = stripped.partition(':')
                        k = k.strip()
                        if k == key:
                            # Remove inline comment and surrounding quotes
                            val = v.split('#')[0].strip()
                            val = val.strip('"').strip("'")
                            return val
    except (IOError, OSError):
        pass
    return None

if __name__ == '__main__':
    if len(sys.argv) < 3:
        sys.exit(1)
    section = sys.argv[1]
    key = sys.argv[2]
    config_path = sys.argv[3] if len(sys.argv) > 3 else 'config.yaml'
    result = read_config(config_path, section, key)
    if result is None:
        sys.exit(1)
    print(result)
