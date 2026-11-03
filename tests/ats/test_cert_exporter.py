import logging
import re
import requests
import time
import tempfile
import subprocess
from functools import partial
from pathlib import Path
from json import dumps
from typing import Dict, List
from datetime import datetime, timedelta

import pykube
import pytest
from pytest_helm_charts.fixtures import Cluster
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.k8s.daemon_set import wait_for_daemon_sets_to_run
from pytest_helm_charts.giantswarm_app_platform.app import (
    AppFactoryFunc,
    ConfiguredApp,
)

from cert_util import cert_gen


logger = logging.getLogger(__name__)

app_name = "cert-exporter"
namespace_name = "kube-system"
cert_manager_app_chart_version = "3.8.1"
# change this if your want to change the kind config
daemonset_port = 30017
deployment_port = 30018
host_ports = [
    daemonset_port,
    deployment_port,
]

timeout: int = 360


def prepare_services(kube_cluster: Cluster) -> None:
    kubectl = partial(kube_cluster.kubectl, namespace=namespace_name, output_format="")

    # patch services to be NodePort services
    # otherwise tests won't work
    ce_patch = dumps(
        {
            "spec": {
                "type": "NodePort",
                "ports": [{"name": "cert-exporter", "port": 9005, "nodePort": 30007}],
            }
        }
    )
    kubectl("patch service cert-exporter-daemonset", patch=ce_patch)
    logger.info("Patched cert-exporter-daemonset service")

    sce_patch = dumps(
        {
            "spec": {
                "type": "NodePort",
                "ports": [
                    {"name": "cert-exporter", "port": 9005, "nodePort": 30008}
                ],
            }
        }
    )
    kubectl("patch service cert-exporter-deployment", patch=sce_patch)
    logger.info("Patched cert-exporter-deployment service")


@pytest.mark.smoke
@pytest.mark.upgrade
def test_api_working(kube_cluster: Cluster) -> None:
    """Very minimalistic example of using the [kube_cluster](pytest_helm_charts.fixtures.kube_cluster)
    fixture to get an instance of [Cluster](pytest_helm_charts.clusters.Cluster) under test
    and access its [kube_client](pytest_helm_charts.clusters.Cluster.kube_client) property
    to get access to Kubernetes API of cluster under test.
    Please refer to [pykube](https://pykube.readthedocs.io/en/latest/api/pykube.html) to get docs
    for [HTTPClient](https://pykube.readthedocs.io/en/latest/api/pykube.html#pykube.http.HTTPClient).
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1


@pytest.mark.smoke
@pytest.mark.upgrade
def test_cluster_info(
    kube_cluster: Cluster, cluster_type: str, test_extra_info: Dict[str, str]
) -> None:
    """Example shows how you can access additional information about the cluster the tests are running on"""
    logger.info(f"Running on cluster type {cluster_type}")
    key = "external_cluster_type"
    if key in test_extra_info:
        logger.info(f"{key} is {test_extra_info[key]}")
    assert kube_cluster.kube_client is not None
    assert cluster_type != ""


@pytest.fixture(scope="module")
def certexporter_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    deployments = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        ["cert-exporter-deployment"],
        namespace_name,
        timeout,
    )
    return deployments


@pytest.fixture(scope="module")
def cert_manager_app_cr(app_factory: AppFactoryFunc) -> ConfiguredApp:
    res = app_factory(
        app_name="cert-manager-app",  # app_name
        app_version=cert_manager_app_chart_version,  # app_version
        catalog_name="giantswarm-stable",  # catalog_name
        catalog_namespace="giantswarm",  # catalog_namespace
        catalog_url="https://giantswarm.github.io/giantswarm-catalog/",  # catalog_url
        namespace="cert-manager-app",
        deployment_namespace="cert-manager-app",
        timeout_sec=timeout,
    )
    return res


@pytest.fixture(scope="module")
def certexporter_daemonset(kube_cluster: Cluster) -> List[pykube.DaemonSet]:
    daemonsets = wait_for_daemon_sets_to_run(
        kube_cluster.kube_client,
        ["cert-exporter-daemonset"],
        namespace_name,
        timeout,
    )
    return daemonsets


def retrieve_metrics(port: int) -> List[str]:
    r = requests.get(f"http://127.0.0.1:{port}/metrics")

    assert r.status_code == 200

    raw_metrics = r.text
    metrics_lines = [
        line for line in raw_metrics.splitlines() if not line.startswith("#")
    ]

    assert len(metrics_lines) != 0

    return metrics_lines


def check_expiry(ts):
    def check(metric):
        val = metric.split(" ")[1]
        return abs(int(float(val)) - ts) < 5

    return check


def assert_metric(metrics: List[str], metric: str, validator=None) -> None:
    if validator is None:
        found = [m for m in metrics if m.startswith(metric)]
    else:
        found = [m for m in metrics if m.startswith(metric) and validator(m)]
    assert len(found) == 1


LABEL_RE = re.compile(r'(\w+)="((?:[^"\\]|\\.)*)"')


def parse_sample(line: str):
    """Split an exposition line into (metric name, {label: value}, value)."""
    name, _, rest = line.partition("{")
    body, _, value = rest.rpartition("}")
    labels = dict(LABEL_RE.findall(body))
    return name, labels, float(value.split()[0])


def samples_with(metrics: List[str], metric_name: str, **labels) -> List[str]:
    """Lines of metric_name whose labels include every given label=value.

    Prometheus renders labels in alphabetical order, so a label added by a
    newer chart (serialnumber, fingerprint) can land anywhere in the line.
    Matching on the label set instead of a textual prefix keeps these tests
    independent of that, and lets the upgrade scenario run them against the
    released chart as well as the chart under test.
    """
    found = []
    for m in metrics:
        if not m.startswith(metric_name + "{"):
            continue
        _, got, _ = parse_sample(m)
        if all(got.get(k) == v for k, v in labels.items()):
            found.append(m)
    return found


def assert_sample(
    metrics: List[str], metric_name: str, labels: Dict[str, str], validator=None
) -> None:
    found = samples_with(metrics, metric_name, **labels)
    if validator is not None:
        found = [m for m in found if validator(m)]
    assert (
        len(found) == 1
    ), f"expected exactly one {metric_name} sample with {labels}, got {found}"


@pytest.mark.smoke
@pytest.mark.upgrade
def test_pods_available(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    for s in certexporter_deployment:
        logger.info(
            f"Deployment '{s.name}' has {s.obj['status']['readyReplicas']} readyReplicas"
        )
        assert int(s.obj["status"]["readyReplicas"]) > 0
    for ds in certexporter_daemonset:
        logger.info(
            f"DaemonSet '{ds.name}' has status numberReady = {ds.obj['status']['numberReady']}"
        )
        assert int(ds.obj["status"]["numberReady"]) > 0


@pytest.mark.smoke
@pytest.mark.upgrade
def test_exporters_reachable(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    # let the dust settle
    time.sleep(10)

    # check if exporters respond to anything
    for port in host_ports:
        retrieve_metrics(port)


@pytest.mark.functional
@pytest.mark.upgrade
def test_file_certificate_metrics(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_not_after"

    cert_name = "filesystem-cert"

    expires_in_secs = 3600
    expected_expiry = int(
        (datetime.now() + timedelta(seconds=expires_in_secs)).strftime("%s")
    )

    with tempfile.TemporaryDirectory() as tmpdirname:
        cert = cert_gen(name=cert_name, not_after=expires_in_secs)[0]
        cert_path = f"{tmpdirname}/{cert_name}.crt"

        with open(cert_path, "wt") as f:
            f.write(cert)

        # kind-control-plane is the container name
        subprocess.run(
            ["docker", "cp", cert_path, "kind-control-plane:/certs"],
            check=True,
        )

    # let the dust settle
    time.sleep(5)

    # request from daemonset port
    # Match on the label set, not a textual prefix: the released chart and the
    # chart under test differ in which labels they add (serialnumber,
    # fingerprint), and the upgrade scenario runs this against both.
    ds_metrics = retrieve_metrics(daemonset_port)
    assert_sample(
        ds_metrics,
        metric_name,
        {"path": f"/certs/{cert_name}.crt"},
        check_expiry(expected_expiry),
    )

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    assert len([m for m in deploy_metrics if m.startswith(metric_name)]) == 0

    # cleanup
    subprocess.run(
        ["docker", "exec", "kind-control-plane", "bash", "-c", "rm -rf /certs/*"],
        check=True,
    )


@pytest.mark.functional
@pytest.mark.upgrade
def test_secret_metrics(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_secret_not_after"

    cert_name = "secret-cert"

    expires_in_secs = 3600
    expected_expiry = int(
        (datetime.now() + timedelta(seconds=expires_in_secs)).strftime("%s")
    )

    # Create a kubernetes.io/tls secret
    with tempfile.TemporaryDirectory() as tmpdirname:
        (cert, key) = cert_gen(name=cert_name, not_after=expires_in_secs)
        cert_path = f"{tmpdirname}/{cert_name}.crt"
        key_path = f"{tmpdirname}/{cert_name}.key"

        with open(cert_path, "wt") as f:
            f.write(cert)

        with open(key_path, "wt") as f:
            f.write(key)

        kube_cluster.kubectl(
            f"create secret tls {cert_name}",
            output_format="",
            cert=cert_path,
            key=key_path,
        )

    # let the dust settle
    time.sleep(5)

    # request from deployment port
    # Match on the label set, not a textual prefix: the released chart and the
    # chart under test differ in which labels they add (serialnumber,
    # fingerprint), and the upgrade scenario runs this against both.
    deploy_metrics = retrieve_metrics(deployment_port)
    assert_sample(
        deploy_metrics,
        metric_name,
        {
            "certificatename": "",
            "name": cert_name,
            "namespace": "default",
            "secretkey": "tls.crt",
        },
        check_expiry(expected_expiry),
    )

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    assert len([m for m in ds_metrics if m.startswith(metric_name)]) == 0

    # cleanup
    kube_cluster.kubectl(
        f"delete secret {cert_name}",
        output_format="",
    )


@pytest.mark.functional
# Not marked as upgrade: the previous released chart does not contain the fix
# this test asserts on, so it can only run against the chart under test.
def test_secret_metrics_with_concatenated_certificates(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
):
    """Regression test for https://github.com/giantswarm/roadmap/issues/4323.

    A secret key holding several concatenated certificates (as produced by e.g.
    Kyverno webhook cert rotation) used to emit samples with identical label
    sets. Prometheus rejected the scrape with HTTP 500, so the whole /metrics
    endpoint went blank, not just that secret's metric. Each certificate in the
    chain must be reported, and unrelated secrets must keep being served even
    though the chain repeats a certificate.
    """
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_secret_not_after"

    chain_name = "concatenated-cert"
    unrelated_name = "unrelated-cert"

    leaf_expires_in_secs = 3600
    ca_expires_in_secs = 7200

    # Pinned so the test can assert on the serialnumber label without depending
    # on how long key generation took.
    leaf_serial = 1001
    ca_serial = 1002
    unrelated_serial = 1003

    with tempfile.TemporaryDirectory() as tmpdirname:
        # Distinct serial numbers, as real certificates have.
        (leaf, leaf_key) = cert_gen(
            name=chain_name, not_after=leaf_expires_in_secs, serial=leaf_serial
        )
        (ca, _) = cert_gen(
            name=f"{chain_name}-ca", not_after=ca_expires_in_secs, serial=ca_serial
        )
        (unrelated, unrelated_key) = cert_gen(
            name=unrelated_name,
            not_after=leaf_expires_in_secs,
            serial=unrelated_serial,
        )

        chain_path = f"{tmpdirname}/{chain_name}.crt"
        chain_key_path = f"{tmpdirname}/{chain_name}.key"
        unrelated_path = f"{tmpdirname}/{unrelated_name}.crt"
        unrelated_key_path = f"{tmpdirname}/{unrelated_name}.key"

        # Leaf, its CA, and the CA a second time. The repeated certificate is
        # what used to produce two samples with identical labels.
        with open(chain_path, "wt") as f:
            f.write(leaf + ca + ca)

        with open(chain_key_path, "wt") as f:
            f.write(leaf_key)

        with open(unrelated_path, "wt") as f:
            f.write(unrelated)

        with open(unrelated_key_path, "wt") as f:
            f.write(unrelated_key)

        kube_cluster.kubectl(
            f"create secret tls {chain_name}",
            output_format="",
            cert=chain_path,
            key=chain_key_path,
        )
        kube_cluster.kubectl(
            f"create secret tls {unrelated_name}",
            output_format="",
            cert=unrelated_path,
            key=unrelated_key_path,
        )

    # let the dust settle
    time.sleep(5)

    try:
        # retrieve_metrics asserts the endpoint answers 200 and returns samples,
        # which is the core of the regression: it used to be 500 and empty.
        deploy_metrics = retrieve_metrics(deployment_port)

        chain_metrics = samples_with(
            deploy_metrics,
            metric_name,
            certificatename="",
            name=chain_name,
            namespace="default",
            secretkey="tls.crt",
        )

        # Leaf and CA, each reported exactly once. The repeated CA must not add
        # a second, colliding sample.
        assert len(chain_metrics) == 2, (
            f"expected 2 samples for the concatenated chain, got {len(chain_metrics)}: "
            f"{chain_metrics}"
        )

        # Both certificates are there, told apart by the serialnumber label that
        # v2.11.1 added. Asserting on the serials instead of on expiry
        # timestamps keeps this independent of how long key generation and
        # secret creation took. The exporter formats the serial as lowercase hex.
        expected_serials = {f"{leaf_serial:x}", f"{ca_serial:x}"}
        got_serials = {m.split('serialnumber="')[1].split('"')[0] for m in chain_metrics}
        assert got_serials == expected_serials, (
            f"expected serials {sorted(expected_serials)}, got {sorted(got_serials)}: "
            f"{chain_metrics}"
        )

        # The unrelated secret must still be exported. This is what silently
        # disappeared when the colliding samples failed the whole scrape.
        unrelated_metrics = samples_with(
            deploy_metrics,
            metric_name,
            certificatename="",
            name=unrelated_name,
            namespace="default",
            secretkey="tls.crt",
        )
        assert len(unrelated_metrics) == 1, (
            "expected the unrelated secret to still be exported, got "
            f"{unrelated_metrics}"
        )
    finally:
        # cleanup
        kube_cluster.kubectl(
            f"delete secret {chain_name} {unrelated_name}",
            output_format="",
        )


@pytest.mark.functional
# Not marked as upgrade: the previous released chart does not contain the fix
# this test asserts on, so it can only run against the chart under test.
def test_secret_metrics_with_same_serial_certificates(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
):
    """Kyverno regenerates its webhook CA with the same key and serial number 0
    and keeps the expired CA in the secret for a year. Both certificates must be
    exported, told apart by the fingerprint label, so the alert sees the newer one.
    """
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_secret_not_after"
    secret_name = "same-serial-ca"

    old_expires_in_secs = 3600
    new_expires_in_secs = 7200
    expected = {
        int((datetime.now() + timedelta(seconds=s)).strftime("%s"))
        for s in (old_expires_in_secs, new_expires_in_secs)
    }

    with tempfile.TemporaryDirectory() as tmpdirname:
        # Same serial for two different certificates, as Kyverno produces.
        (old, old_key) = cert_gen(
            name=secret_name, not_after=old_expires_in_secs, serial=0
        )
        (new, _) = cert_gen(name=secret_name, not_after=new_expires_in_secs, serial=0)

        cert_path = f"{tmpdirname}/{secret_name}.crt"
        key_path = f"{tmpdirname}/{secret_name}.key"

        with open(cert_path, "wt") as f:
            f.write(old + new)

        with open(key_path, "wt") as f:
            f.write(old_key)

        kube_cluster.kubectl(
            f"create secret tls {secret_name}",
            output_format="",
            cert=cert_path,
            key=key_path,
        )

    # let the dust settle
    time.sleep(5)

    try:
        metrics = retrieve_metrics(deployment_port)

        samples = samples_with(
            metrics,
            metric_name,
            certificatename="",
            name=secret_name,
            namespace="default",
            secretkey="tls.crt",
        )
        assert len(samples) == 2, (
            f"expected 2 samples for two certs with serial 0, got {samples}"
        )

        fingerprints = {m.split('fingerprint="')[1].split('"')[0] for m in samples}
        assert len(fingerprints) == 2, f"expected distinct fingerprints, got {samples}"
        assert all(
            len(fp) == 16 and fp == fp.lower() for fp in fingerprints
        ), fingerprints

        # Both expiries are present, so max() over the series is the newer one.
        got = {int(float(m.split(" ")[1])) for m in samples}
        assert all(
            any(abs(g - e) < 5 for g in got) for e in expected
        ), f"expected {expected}, got {got}"
    finally:
        # cleanup
        kube_cluster.kubectl(f"delete secret {secret_name}", output_format="")


@pytest.mark.functional
# @pytest.mark.upgrade
def test_certificate_cr_metrics(
    request,
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
    cert_manager_app_cr: ConfiguredApp,
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    # apply certificate
    kube_cluster.kubectl(
        "apply",
        filename=Path(request.fspath.dirname) / "cert-manager-certificate.yaml",
        output_format="",
    )

    # let the dust settle
    time.sleep(10)

    # Request metrics from the exporters and check if they match expected values
    metric_name = "cert_exporter_certificate_cr_not_after"

    def validate_cert_metric(m):
        return (
            ('issuer_ref="selfsigned-giantswarm"' in m)
            and ('managed_issuer="true",name="test-cert",namespace="default"' in m)
            and ('name="test-cert",namespace="default"' in m)
            and ('namespace="default"' in m)
        )

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    assert_metric(deploy_metrics, metric_name, validate_cert_metric)

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    assert len([m for m in ds_metrics if m.startswith(metric_name)]) == 0
