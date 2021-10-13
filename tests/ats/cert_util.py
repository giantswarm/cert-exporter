# Source from https://stackoverflow.com/a/60804101
# Licensed under CC BY-SA 4.0

from OpenSSL import crypto


def cert_gen(
    name="commonName",
    not_after=(10 * 365 * 24 * 60 * 60),
):
    # create a key pair
    k = crypto.PKey()
    k.generate_key(crypto.TYPE_RSA, 2048)

    # create a self-signed cert
    cert = crypto.X509()

    cert.get_subject().CN = name

    cert.gmtime_adj_notBefore(0)
    cert.gmtime_adj_notAfter(not_after)

    cert.set_issuer(cert.get_subject())

    cert.set_pubkey(k)
    cert.sign(k, "sha512")

    return (
        crypto.dump_certificate(crypto.FILETYPE_PEM, cert).decode("utf-8"),
        crypto.dump_privatekey(crypto.FILETYPE_PEM, k).decode("utf-8"),
    )
