# Measurement container for linux profiling runs: ubuntu + GNU time
# (reports "Maximum resident set size" from wait4 rusage).
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends time && rm -rf /var/lib/apt/lists/*
WORKDIR /prof
