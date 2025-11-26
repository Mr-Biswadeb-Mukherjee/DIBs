# 🌀 Infermal_v2  
**Domain Intelligence and DNS Behavior Analysis Framework**

Infermal_v2 is a modular framework for generating, resolving, analyzing, and correlating domain-level intelligence at scale. It combines DNS analytics, OSINT techniques, domain-mutation logic, enrichment workflows, and worker-driven processing into a complete lifecycle system for domain intelligence.

---

## 🧭 Background and Motivation

In 2024, ICANN and KORLabs published the **INFERMAL** project (Inferential Analysis of Maliciously Registered Domains), a study focused on registration-time characteristics such as registrar behavior, pricing, verification indicators, and TLD patterns. The publication demonstrated the value of domain-focused intelligence research.

**Infermal_v2 is not affiliated with, endorsed by, or derived from ICANN or KORLabs' INFERMAL project.**  
References to INFERMAL are included only to provide context for the importance of domain-centric studies.

Infermal_v2 takes a different approach by analyzing **behavioral and DNS lifecycle patterns**, emphasizing DNS activity, infrastructure evolution, and operational fingerprints.

---

## 🚀 Overview

Infermal_v2 follows an end-to-end domain intelligence pipeline:

**Domain Generation → DNS Resolution → Intelligence Extraction → Correlation → Output**

The framework is intended for:

- OSINT analysts  
- Threat intelligence teams  
- Red team operators  
- Incident responders  
- Large-scale DNS telemetry analysts  

It is implemented in Go to support fast, concurrent data processing across high-volume workloads.

---

## ✨ Key Features

### 1. Domain Mutation and Squatting Algorithms

Infermal_v2 can generate extensive domain variants to detect impersonation attempts, phishing surfaces, and potential adversarial infrastructure. Supported mutation techniques include:

- Bitsquatting  
- Combosquatting  
- Homograph mutations  
- Jaro–Winkler similarity  
- Phonetic-based squatting  
- Subdomain variations  
- Typo-squatting  

---

### 2. High-Speed DNS Processing

The DNS engine provides recursive and stub resolution with adaptive cooldowns and throttling. A worker-based scheduling model enables large-scale domain resolution while respecting network and DNS infrastructure limits.

Supported record types include: **A, AAAA, CNAME, TXT, MX, SOA**, and others.

---

### 3. Domain Intelligence Extraction

Infermal_v2 supports modular extractors that capture detailed DNS behavior and operational characteristics, such as:

- TTL anomalies  
- IP rotation and fast-flux patterns  
- Domain parking indicators  
- DNSSEC validation  
- Nameserver behavior  
- CNAME chain mapping  
- DNS entropy and similarity metrics  

These attributes collectively form a behavioral fingerprint for each domain.

---

### 4. Redis-Backed Worker Architecture

Redis powers the distributed processing layer, enabling:

- Task queuing  
- Caching  
- Concurrency control  
- Load distribution across workers  

This architecture ensures scalability and predictable throughput during large batch operations.

---

### 5. Output System

The Filewriter module provides structured results in formats suitable for downstream tools:

- CSV  
- JSON  
- Custom output formats  

This enables seamless integration with SIEM platforms, external pipelines, visualization tools, and analytical engines.

---

## 📜 License

Add your preferred license (MIT, Apache-2.0, GPL, or a custom license) and include it as a `LICENSE` file at the repository root.

---

## 🤝 Contributions

Contributions are welcome through pull requests and discussions in GitHub issues.  
For major changes, create an issue first to outline proposed design and architectural considerations.

---

## 🎯 Project Vision

Infermal_v2 aims to deliver a scalable framework for analyzing how domains behave across the internet ecosystem. While research such as ICANN/KORLabs' INFERMAL study focuses on registration patterns, Infermal_v2 shifts attention to DNS behavior, lifecycle characteristics, and operational indicators that reveal active threats.

The long-term goal is to merge OSINT, DNS telemetry, and automated enrichment into a unified and extensible intelligence platform.

---

## 🔗 References

Public material referenced for context:

- INFERMAL Study (ICANN & KORLabs) —  
  https://www.icann.org/resources/pages/inferential-analysis-maliciously-registered-domains-infermal-2024-12-03-en

Additional public documentation from ICANN, KORLabs, and related DNS research is cited solely as contextual background.

---

## ✍️ Author

**Mr. Biswadeb Mukherjee**  
Offensive Security Specialist | Malware Developer | Software Engineer
