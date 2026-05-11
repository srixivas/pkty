# pkty Sample Captures

Sample pcap files for testing, demos, and screenshots. Load any with:

```bash
pkty -r samples/<file>.pcap
```

---

## ⬇️ Download samples

Run the download script to fetch verified captures locally (files are gitignored):

```bash
bash samples/download.sh
```

### What you get

| Folder | Source | Size | Protocols | Best for |
|--------|--------|------|-----------|----------|
| `unit42/` | Palo Alto Unit42 | 9 MB zip | DNS, HTTP, TLS, C2 beaconing | DNS widget, TLS Inspector, Remote Hosts, NetGraph |
| `ultimate/` | weberblog.net | 5 MB 7z | 90+ protocols | Protocol Dist, hex dump deep-dives |

**unit42** — 5 real malware infection pcaps (Loki Bot, IcedID, Ave Maria RAT, infostealer, spambot). Zip password: `infected`. Great for seeing pkty catch actual suspicious patterns.

**ultimate** — Synthetic but extremely broad — nearly every protocol in one file. Good for stress-testing the Protocol Distribution widget.

---

## 🔬 More sources

| Source | URL | Notes |
|--------|-----|-------|
| **Palo Alto Unit42** | https://github.com/PaloAltoNetworks/Unit42-Wireshark-tutorials | Real malware traffic, password: `infected` |
| **Malware Traffic Analysis** | https://malware-traffic-analysis.net | Brad Duncan's real C2/malware captures. Password on site |
| **NETRESEC / MACCDC** | https://www.netresec.com/?page=PcapFiles | Collegiate cyber defense competition captures |
| **CyberDefenders** | https://cyberdefenders.org | Blue-team CTF challenges with pcap downloads |
| **Wireshark samples** | https://wiki.wireshark.org/SampleCaptures | Protocol-specific reference captures |
| **PacketLife** | https://packetlife.net/captures | Clean multi-protocol captures |

---

## What makes a good pkty demo pcap

- **DNS queries** — fills the DNS Queries widget fast
- **TLS with SNI** — lights up the TLS Inspector
- **Multiple destination IPs** — makes NetGraph and Remote Hosts interesting
- **Some HTTP/80** — shows alongside HTTPS in Proto Dist
- **500–5000 packets** — enough to scroll, fast enough to replay

---

<!-- TODO: evaluate MACCDC captures for a cybersecurity-focused demo tape -->
<!-- TODO: add more malware families from malware-traffic-analysis.net -->
<!-- TODO: script to auto-select best pcap based on packet count / protocol mix -->
