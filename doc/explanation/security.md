---
discourse: ubuntu:[Discourse&#32;-&#32;Explicit&#32;trust&#32;establishment&#32;mechanism&#32;for&#32;MicroCloud](44261)
---

(exp-security)=
# Security

MicroCloud’s security model is based on explicit trust and secure-by-default components. Each MicroCloud deployment consists of independently secured components ({ref}`exp-security-lxd`, {ref}`exp-security-microceph`, and {ref}`exp-security-microovn`), each enforcing authentication, encryption, and access control within its own domain.

MicroCloud {ref}`further enforces security <exp-security-microcloud>` through the use of mutual TLS (mTLS), certificate-based identities, and an explicit trust establishment mechanism. Its deployment as a collection of signed, confined snaps on Ubuntu further strengthens its overall security posture.

(exp-security-ubuntu)=
## Ubuntu security

MicroCloud runs on Ubuntu and benefits from all [Ubuntu platform security measures](https://ubuntu.com/security), including kernel hardening, signed packages, and continuous security maintenance. For production environments, we recommend using a recent Ubuntu LTS release to ensure long-term support and predictable security updates.

(exp-security-snaps)=
## Snaps

MicroCloud and its components are distributed as [snaps](https://snapcraft.io/docs), which enhances security through providing a confined environment with a streamlined update mechanism. Both LTS and feature channels receive regular security updates through Canonical’s official infrastructure.

All snaps are digitally signed using [assertions](https://snapcraft.io/docs/assertions) to guarantee authenticity and integrity.

(exp-security-reporting)=
## Security reporting and disclosure

Report potential security issues privately through GitHub by [filing a security advisory](https://github.com/canonical/microcloud/security/advisories/new). Please include a clear description of the issue, affected MicroCloud versions, reproduction steps, and any known mitigation strategies.

(exp-security-microcloud)=
## MicroCloud

MicroCloud manages cluster membership and encrypted communication through mTLS and certificate-based identities. When a machine joins a cluster, it verifies the cluster’s certificate fingerprint and receives the complete set of member certificates, establishing a consistent trust store.

During the join process, MicroCloud uses an **explicit trust establishment mechanism** designed to prevent secret leakage and mitigate man-in-the-middle attacks. This mechanism uses a Hash-Based Message Authentication Code (HMAC) to sign the messages exchanged between the machine that initiates the join process and the joining peers. The shared secret used for joining is never transmitted over the network. The join process also enforces rate limits and session timeouts to reduce the risk of replay and brute-force attacks. For further information, refer to the [public specification](https://discourse.ubuntu.com/t/explicit-trust-establishment-mechanism-for-microcloud/44261).

(exp-security-lxd)=
## LXD

For details on LXD’s security architecture and operational guidance, see the {ref}`LXD security overview <lxd:exp-security>` and the {ref}`LXD hardening guide <lxd:howto-security-harden>`.

(exp-security-microceph)=
## MicroCeph

The {doc}`MicroCeph security documentation <microceph:explanation/security/security-overview>` provides information on encryption, authentication, best practices for secure deployment and operation, and more.

(exp-security-microovn)=
## MicroOVN

MicroOVN secures its network endpoints using the TLS protocol (version 1.2 or higher), along with P-384 elliptic curve keys. For details, refer to the {doc}`MicroOVN documentation on its use of cryptography <microovn:reference/cryptography>`. Also see the {doc}`MicroOVN security process documentation <microovn:reference/security>`.
