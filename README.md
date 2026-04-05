
<div align="center">

# DIBs

**Domain Intelligence & Behavior System**

*A DNS Intelligence & Infrastructure Behavioral Analysis Framework*

[![Version](https://img.shields.io/badge/Version-v1.0.0-informational?style=flat-square)](./VERSION.md)

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square\&logo=go\&logoColor=white)](https://golang.org/)
[![Build Status](https://img.shields.io/badge/Build-Passing-2ea44f?style=flat-square\&logo=github-actions\&logoColor=white)](https://github.com/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue?style=flat-square)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen?style=flat-square)](https://github.com/)
[![Redis](https://img.shields.io/badge/Redis-Required-DC382D?style=flat-square\&logo=redis\&logoColor=white)](https://redis.io/)
[![Platform](https://img.shields.io/badge/Platform-Linux-FCC624?style=flat-square\&logo=linux\&logoColor=black)](https://www.kernel.org/)

[![Reproducible Snapshot](https://img.shields.io/badge/Reproducible-Snapshot-blueviolet?style=flat-square)](#-reproducible-execution-snapshot)

*High-throughput domain intelligence, DNS resolution, and behavioral analysis at scale.*

---

[Overview](#overview) · [Version](#version) · [Whitepaper](#whitepaper)
· [Architecture](#architecture) · [Features](#features) · [Getting Started](#getting-started) · [API](#api-control-plane) · [Use Cases](#-use-cases) · [Citation](#citation)
</div>

---

## Overview

**DIBs** is a modular framework for generating, resolving, analyzing, and correlating domain intelligence at scale. It is designed for offensive security workflows, threat intelligence operations, and DNS telemetry analysis.

The system operates as a structured pipeline:

```

Domain Generation → DNS Resolution → Intelligence Extraction → Correlation → Output

````

Outputs are emitted as structured NDJSON, enabling integration with SIEM platforms and downstream processing systems.

This work is part of ongoing offensive security research conducted by Biswadeb Mukherjee (Independent Research).

---

## Whitepaper

The full technical whitepaper is available on my website:

[Read the Whitepaper](https://official-biswadeb941.in)

GitHub Release: https://github.com/Mr-Biswadeb-Mukherjee/DIBs/releases/tag/v1.0.0

This whitepaper documents the system architecture, methodology, and findings from a controlled large-scale DNS intelligence operation.

---

## Version

* Public Release Notes: [VERSION.md](./VERSION.md)

This release defines the system’s supported capabilities, operational guarantees, and known limitations.
Operators are encouraged to review the version contract before running large-scale workloads.

---

## Architecture

DIBs follows a unidirectional processing pipeline with controlled execution stages.

### High-Level Diagram

![System Architecture Diagram](System_Design/DIBs_SAD.drawio.png)

---

## Features

- Domain Mutation Engine
  - Bitsquatting  
  - Typosquatting  
  - Combosquatting  
  - Homograph generation  
  - Phonetic mutations  
  - Similarity scoring (Jaro–Winkler)  
  - Subdomain permutations  

Note: All generated domains are validated, deduplicated, and scored before processing.

- High-Speed DNS Engine
  - Recursive and stub resolution modes  
  - Multi-record support (A, AAAA, CNAME, MX, TXT, SOA)  
  - Adaptive timeout and retry logic  
  - High-concurrency worker execution  

- Intelligence Extraction
  - A / AAAA enumeration  
  - CNAME chain resolution  
  - Nameserver profiling  
  - MX and TXT record extraction  
  - Provider attribution  
  - TTL anomaly detection  
  - Fast-flux detection  
  - DNSSEC validation  

- Correlation Engine
  - Infrastructure clustering (IP / ASN)  
  - Domain relationship mapping  
  - Shared infrastructure detection  
- Output System
  - Generated domains  
  - Resolved domains  
  - DNS intelligence records  
  - Infrastructure clusters  
  - Runtime metrics  

Note: All outputs are NDJSON-based

---

## Getting Started

### Prerequisites

* Go 1.21+  
* Redis 6.0+  
* Linux (recommended)  

---

### Installation

```bash
git clone https://github.com/Mr-Biswadeb-Mukherjee/DIBs
cd DIBs
go mod tidy
````

---

### Run

```bash
go run .
```

---

## API Control Plane

DIBs exposes an API for managing scan lifecycle and runtime control.

### Core Endpoints

| Endpoint        | Method |
| --------------- | ------ |
| /healthz        | GET    |
| /api/v3/start   | POST   |
| /api/v3/stop    | POST   |
| /api/v3/status  | GET    |
| /api/v3/metrics | GET    |

Authentication uses **ed25519 public key validation** via the `X-API-Key` header.

---


## 📦 Reproducible Execution Snapshot

A complete execution snapshot of DIBs is provided for research, validation, and reproducibility in the directory `release`

This snapshot includes:

* Full runtime outputs (`.ndjson`)
* Execution logs
* Input datasets
* Configuration files
* Generated result artifacts
* Compiled binary used during execution

### 🔽 Extract the Snapshot

```bash
tar --zstd -xvf DIBs_v1.tar.zst
```
### ▶️ Reproduce the Execution

Follow the detailed execution steps in:

```bash
RUN.md
```

### ⚠️ Notes

* DNS and infrastructure data are time-dependent; results may vary on re-execution
* For exact analysis, use the provided snapshot outputs
* Ensure Redis is properly configured before running the system


---

## 🎯 Use Cases

* Threat intelligence enrichment
* Phishing infrastructure detection
* Red team reconnaissance
* Domain monitoring
* Incident response
* DNS telemetry research

---

## Citation

If you use this work, please cite:

```bibtex
@techreport{offsec-biswadeb2026-dibsv1,
  author       = {Mukherjee, Biswadeb},
  title        = {Domain Intelligence \& Behaviour System (DIBs)},
  version      = {v1},
  year         = {2026},
  institution  = {Independent Research},
  url          = {https://official-biswadeb941.in},
}
```

---

## License

The source code is licensed under the Apache License 2.0.

The whitepaper and documentation are licensed under CC BY 4.0.

---

## Author

Biswadeb Mukherjee

Offensive Security Specialist · Malware Engineer

ORCID: https://orcid.org/0009-0005-4610-4010

---

<div align="center">
<sub>Built for operators who need answers, not dashboards.</sub>
</div>

<div align="center">
<sub>Copyright 2026 Biswadeb Mukherjee</sub>
</div>
