#!/usr/bin/env python3
"""Dev-only supervisor CLI — delegates to dev_supervisor."""
from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import dev_supervisor as supervisor


def main() -> None:
    parser = argparse.ArgumentParser(description="QuarkGate dev singleton supervisor")
    parser.add_argument("--once", action="store_true", help="Dedupe duplicates and exit")
    parser.add_argument("--watch", action="store_true", help="Alias for --supervise")
    parser.add_argument("--supervise", action="store_true", help="Dedupe + restart down services")
    parser.add_argument("--interval", type=float, default=15.0)
    parser.add_argument("--verbose", "-v", action="store_true")
    args = parser.parse_args()

    supervisor.load_env()

    if args.supervise or args.watch:
        if args.verbose:
            print(f"[supervisor] watching every {args.interval}s — logs in {supervisor.LOG_DIR}")
        while True:
            supervisor.run_supervise(verbose=args.verbose)
            time.sleep(args.interval)
    else:
        supervisor.run_once(verbose=args.verbose)


if __name__ == "__main__":
    main()
