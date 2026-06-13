#!/usr/bin/env python3
from __future__ import annotations

import os
import shutil
from pathlib import Path

import kagglehub


DATASET = "olistbr/brazilian-ecommerce"


def main() -> None:
    target = Path(os.environ.get("LIBREDASH_DATA_DIR", ".data/olist")).resolve()
    target.mkdir(parents=True, exist_ok=True)

    source = Path(kagglehub.dataset_download(DATASET))
    copied = 0

    for csv in source.glob("*.csv"):
        destination = target / csv.name
        shutil.copy2(csv, destination)
        copied += 1

    print(f"Downloaded {DATASET}")
    print(f"Source: {source}")
    print(f"Copied {copied} CSV files to {target}")


if __name__ == "__main__":
    main()

