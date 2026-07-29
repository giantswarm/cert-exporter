# Source from https://stackoverflow.com/a/60804101
# Licensed under CC BY-SA 4.0

import secrets

from OpenSSL import crypto


def cert_gen(
    name="commonName",
    not_after=(10 * 365 * 24 * 60 * 60),
    serial=None,
):
    # create a key pair
    k = crypto.PKey()
    k.generate_key(crypto.TYPE_RSA, 2048)

    # create a self-signed cert
    cert = crypto.X509()

    cert.get_subject().CN = name

    # Real certificates carry a unique serial number, and cert-exporter relies on
    # it to tell certificates concatenated in one key apart. pyOpenSSL defaults
    # to 0, so without this every generated certificate shared a serial and any
    # test concatenating two of them would collide for reasons that have nothing
    # to do with the exporter. Default to a unique serial so callers cannot hit
    # that by omission; pass one explicitly when a test needs it to be stable.
    if serial is None:
        serial = secrets.randbits(64) + 1
    cert.set_serial_number(serial)

    cert.gmtime_adj_notBefore(0)
    cert.gmtime_adj_notAfter(not_after)

    cert.set_issuer(cert.get_subject())

    cert.set_pubkey(k)
    cert.sign(k, "sha512")

    return (
        crypto.dump_certificate(crypto.FILETYPE_PEM, cert).decode("utf-8"),
        crypto.dump_privatekey(crypto.FILETYPE_PEM, k).decode("utf-8"),
    )
