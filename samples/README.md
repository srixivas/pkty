# pkty Sample Captures

Sample pcap files for testing and demo recording. Load any of these with:

```bash
pkty -r samples/<file>.pcap
```

---

## Included samples

<!-- Add downloaded pcap files here as you collect them -->

| File | Source | Protocols | Notes |
|------|--------|-----------|-------|
| _(none yet — see sources below)_ | | | |

---

## Recommended sources

### 🔬 Malware / threat traffic (real-world)

| Source | URL | Notes |
|--------|-----|-------|
| **Palo Alto Unit42 Wireshark tutorials** | https://github.com/PaloAltoNetworks/Unit42-Wireshark-tutorials | Real malware pcaps (Loki Bot, IcedID, Ave Maria RAT). Zip password: `infected` |
| **Malware Traffic Analysis** | https://malware-traffic-analysis.net | Brad Duncan's collection of real C2/malware traffic. Password shown on site |
| **NETRESEC MACCDC** | https://www.netresec.com/?page=PcapFiles | Collegiate cyber defense competition captures — rich mix of attack + defense |

### 🧪 Protocol / CTF samples

| Source | URL | Notes |
|--------|-----|-------|
| **CyberDefenders** | https://cyberdefenders.org | Free blue-team challenges, many include pcap downloads |
| **Wireshark sample captures** | https://wiki.wireshark.org/SampleCaptures | Protocol-specific samples, good for widget testing |
| **PacketLife** | https://packetlife.net/captures | Clean protocol captures across many layers |

---

## What makes a good pkty demo pcap

- **DNS queries** — fills the DNS Queries widget fast
- **TLS with SNI** — lights up the TLS Inspector
- **Multiple destination IPs** — makes NetGraph and Remote Hosts interesting
- **Some HTTP/80** — shows alongside HTTPS in Proto Dist
- **500–5000 packets** — enough to scroll, fast enough to replay

---

<!-- TODO: download and commit 1-2 good samples for offline demo use -->
<!-- TODO: evaluate MACCDC 2012 captures for a compelling cybersecurity demo -->
<!-- TODO: add a script to auto-download and verify samples -->
