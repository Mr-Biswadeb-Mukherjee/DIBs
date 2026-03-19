# 🌀 Infermal_v2

### Domain Intelligence and DNS Behavior Analysis Framework

![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Build](https://img.shields.io/badge/Build-Passing-brightgreen)
![Status](https://img.shields.io/badge/Status-Active-blue)
![Contributions](https://img.shields.io/badge/Contributions-Welcome-orange)

---

## 📌 Overview

**Infermal_v2** is a modular, high-performance framework for generating, resolving, analyzing, and correlating domain intelligence at scale.

It combines:

* DNS analytics
* OSINT techniques
* Domain mutation logic
* Distributed worker pipelines
* Intelligence enrichment

into a unified lifecycle system:

```
Domain Generation → DNS Resolution → Intelligence Extraction → Correlation → Output
```

---

## 🧭 Background

The **INFERMAL (2024)** study by ICANN and KORLabs demonstrated the value of analyzing domain registration patterns.

> ⚠️ Infermal_v2 is an independent project and is **not affiliated with or derived from INFERMAL**.

Unlike registration-focused research, Infermal_v2 focuses on:

* DNS behavior
* Infrastructure evolution
* Operational patterns
* Lifecycle-based intelligence

---

## ⚙️ Architecture

```
                ┌──────────────────────┐
                │ Domain Generator     │
                └─────────┬────────────┘
                          │
                ┌─────────▼────────────┐
                │ DNS Resolver Engine  │
                └─────────┬────────────┘
                          │
                ┌─────────▼────────────┐
                │ Intelligence Extract │
                └─────────┬────────────┘
                          │
                ┌─────────▼────────────┐
                │ Correlation Engine   │
                └─────────┬────────────┘
                          │
                ┌─────────▼────────────┐
                │ Output (CSV/JSON)    │
                └──────────────────────┘
```

---

## ✨ Features

### 🔹 Domain Mutation Engine

Detect impersonation and adversarial domains using:

* Bitsquatting
* Typo-squatting
* Combosquatting
* Homograph attacks
* Phonetic mutations
* Jaro–Winkler similarity
* Subdomain permutations

---

### 🔹 High-Speed DNS Engine

* Recursive + stub resolution
* Adaptive rate limiting
* Worker-based concurrency
* Supports:

  ```
  A, AAAA, CNAME, TXT, MX, SOA
  ```

---

### 🔹 Intelligence Extraction

Extract behavioral fingerprints:

* TTL anomalies
* Fast-flux / IP rotation
* DNSSEC validation
* Nameserver profiling
* CNAME chain mapping
* Entropy analysis

---

### 🔹 Distributed Processing (Redis)

* Task queueing
* Load distribution
* Caching layer
* Horizontal scalability

---

### 🔹 Output System

Export structured intelligence:

* JSON
* CSV
* Custom schemas

Integrates easily with:

* SIEM systems
* Data pipelines
* Visualization tools

---

## 🚀 Getting Started

### Prerequisites

* Go 1.21+
* Redis
* Linux (recommended)

---

### Installation

```bash
git clone https://github.com/yourusername/infermal_v2.git
cd infermal_v2
go mod tidy
```

---

### Configuration

Create a config file:

```yaml
redis:
  host: localhost
  port: 6379

dns:
  timeout: 3s
  retries: 2

workers:
  count: 50
```

---

### Run

```bash
go run main.go
```

---

## 📂 Project Structure

```
infermal_v2/
│
├── cmd/                # Entry points
├── internal/
│   ├── generator/      # Domain mutation logic
│   ├── resolver/       # DNS engine
│   ├── extractor/      # Intelligence extraction
│   ├── correlator/     # Analysis engine
│   ├── worker/         # Worker pool + Redis
│   └── output/         # File writers
│
├── configs/
├── scripts/
├── test/
└── main.go
```

---

## 🧠 Use Cases

* Threat intelligence enrichment
* Phishing infrastructure detection
* Domain monitoring at scale
* Red team reconnaissance
* Incident response analysis
* DNS telemetry research

---

## 📜 License

This project is licensed under the **MIT License**.
See the `LICENSE` file for details.

---

## 🤝 Contributing

Contributions are welcome.

### Workflow:

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

For major changes, open an issue first to discuss:

* Design approach
* Architectural impact
* Performance considerations

---

## 🎯 Vision

Infermal_v2 aims to become a **unified domain intelligence platform** by combining:

* OSINT
* DNS telemetry
* Behavioral analytics
* Automated enrichment

into a scalable and extensible ecosystem.

---

## 🔗 References

* INFERMAL Study (ICANN & KORLabs)
  [https://www.icann.org/resources/pages/inferential-analysis-maliciously-registered-domains-infermal-2024-12-03-en](https://www.icann.org/resources/pages/inferential-analysis-maliciously-registered-domains-infermal-2024-12-03-en)

---

## ✍️ Author

**Biswadeb Mukherjee**
Offensive Security Specialist | Malware Developer | Software Engineer
