# Source from https://stackoverflow.com/a/60804101
# Licensed under CC BY-SA 4.0

from OpenSSL import crypto

import subprocess


def cert_gen(
    name="commonName",
    validityEndInSeconds=(10 * 365 * 24 * 60 * 60),
    target_dir=".",
):
    # create a key pair
    k = crypto.PKey()
    k.generate_key(crypto.TYPE_RSA, 2048)

    # create a self-signed cert
    cert = crypto.X509()

    cert.get_subject().CN = name

    cert.gmtime_adj_notBefore(0)
    cert.gmtime_adj_notAfter(validityEndInSeconds)

    cert.set_issuer(cert.get_subject())

    cert.set_pubkey(k)
    cert.sign(k, "sha512")

    with open(f"{target_dir}/{name}.crt", "wt") as f:
        f.write(crypto.dump_certificate(crypto.FILETYPE_PEM, cert).decode("utf-8"))

    subprocess.run(
        ["docker", "cp", f"{target_dir}/{name}.crt", "kind-control-plane:/certs"],
        check=True,
    )
