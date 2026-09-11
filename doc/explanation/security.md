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

Ubuntu LTS releases subscribed to Ubuntu Pro can use the [Ubuntu Security Guide (USG)](https://documentation.ubuntu.com/security/compliance/usg/) for CIS hardening. Refer to the LXD documentation on {ref}`lxd:howto-security-harden-cis` for related details about auditing LXD hosts with USG.

(exp-security-snaps)=
## Snaps and supported versions

The MicroCloud team maintains both Long Term Support (LTS) and feature releases. See {ref}`ref-releases-snaps` and our {ref}`ref-release-notes` for details about the currently supported releases.

MicroCloud and its components are distributed as [snaps](https://snapcraft.io/docs), which enhances security by providing a confined environment with a streamlined update mechanism. Both LTS and feature channels receive regular security updates through Canonical’s official infrastructure.

All snaps are digitally signed using {ref}`assertions <snap:explanation-security-assertions>` to guarantee authenticity and integrity.

(exp-security-reporting)=
## Security reporting and disclosure

MicroCloud adheres to the [Ubuntu disclosure policy](https://ubuntu.com/security/disclosure-policy). Report potential security issues privately through GitHub by [filing a security advisory](https://github.com/canonical/microcloud/security/advisories/new). Please include a clear description of the issue, affected MicroCloud versions, reproduction steps, and any known mitigation strategies. Refer to the [MicroCloud security policy](https://github.com/canonical/microcloud/blob/main/SECURITY.md) for details.

(exp-security-microcloud)=
## MicroCloud

### Cryptography

MicroCloud manages cluster membership and encrypted communication through mTLS and certificate-based identities. When a machine joins a cluster, it verifies the cluster’s certificate fingerprint and receives the complete set of member certificates, establishing a consistent trust store.

During the join process, MicroCloud uses an **explicit trust establishment mechanism** designed to prevent secret leakage and mitigate {spellexception}`man-in-the-middle` attacks. This mechanism uses a Hash-Based Message Authentication Code (HMAC) to sign the messages exchanged between the machine that initiates the join process and the joining peers.

```{figure} /images/microcloud_secure_join.svg
:alt: A diagram of the MicroCloud join process
:align: center
:width: 75%

MicroCloud join process
```

MicroCloud uses a passphrase to generate the HMAC. If you begin the join process in {ref}`interactive mode <howto-initialize-interactive>`, then MicroCloud generates the passphrase as a concatenation of four words randomly selected from the [EFF short list](https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases) of words with unique three-character prefixes. Using this list means that you only need to type the first three characters of each word for the remainder to be guessed with auto-completion. MicroCloud displays this passphrase on the member initiating the process, and you must input the passphrase on the joining member. If, however, you use a {ref}`preseed file <ref-preseed>` to automate the initialization process, then you must specify the passphrase yourself.

On the joining member, MicroCloud uses the [Argon2 function](https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-argon2-03#section-3.1) to generate a key from the passphrase and a random salt.[^1] The joiner then sends a request to the initiator that contains the joiner's public certificate, the random salt, and an HMAC created from the Argon2 key and the body of the request. The initiator uses the passphrase to validate the HMAC and, if the HMAC is valid, adds the joiner's certificate to a temporary trust store. The initiator then sends its own public certificate and an HMAC back to the joiner, which similarly uses the passphrase to validate the HMAC.

[^1]: MicroCloud follows the second recommended option for Argon2 [parameter choice](https://www.rfc-editor.org/info/rfc9106/#name-parameter-choice) proposed by RFC 9106: {math}`t=3` iterations, {math}`p=4` lanes, {math}`m=2^{16}` KiB (64 MiB of RAM), and 256-bit tag size.

Once the initiator and joiner have exchanged certificates, they can establish mTLS and use that channel to form the MicroCloud, LXD, MicroCeph, and MicroOVN clusters. The MicroCloud, MicroCeph, and MicroOVN Dqlite clusters are created with [MicroCluster](https://github.com/canonical/microcluster) and the LXD cluster is set up with {ref}`Dqlite <lxd:dqlite-internals>` alone.

The passphrase used for joining is never transmitted over the network. The join process also enforces rate limits and session timeouts to reduce the risk of replay and brute-force attacks.

For further information about how MicroCloud establishes trust, refer to the [public specification](https://discourse.ubuntu.com/t/explicit-trust-establishment-mechanism-for-microcloud/44261).

### Logging

MicroCloud creates logs through systemd. These logs can be accessed with `sudo snap logs microcloud`.

(exp-security-lxd)=
## LXD

For details on LXD’s security architecture and operational guidance, see the {ref}`LXD security overview <lxd:exp-security>` and the {ref}`LXD hardening guide <lxd:howto-security-harden>`.

(exp-security-microceph)=
## MicroCeph

The {doc}`MicroCeph security documentation <microceph:snap/explanation/security/security-overview>` provides information on encryption, authentication, best practices for secure deployment and operation, and more.

(exp-security-microovn)=
## MicroOVN

MicroOVN secures its network endpoints using the TLS protocol (version 1.2 or higher), along with P-384 elliptic curve keys. For details, refer to the MicroOVN documentation on {doc}`cryptography <microovn:reference/cryptography>`, {doc}`working with TLS <microovn:how-to/tls>`, and the {doc}`MicroOVN security process <microovn:reference/security>`.
